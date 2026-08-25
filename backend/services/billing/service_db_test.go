package billing_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/billing"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// tenantTx runs fn inside a tenant transaction — the ambient context
// GetOverview expects when it is called from the tenant HTTP path.
func tenantTx(t *testing.T, db *bun.DB, tenantID int64, fn func(ctx context.Context) error) error {
	t.Helper()
	return tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			return fn(ctx)
		})
}

// The operator write path runs through tenant.WithTenantTx, which the unit
// tests deliberately skip (they wire no *bun.DB). These tests exercise the
// real transaction: the role switch, the RLS session variable, and the
// tenant_id stamping that follows from them.

func newDBService(t *testing.T) (billing.Service, int64) {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	return billing.NewService(billing.Config{
		Invoices: platformRepo.NewSchoolInvoiceRepository(db),
		DB:       db,
	}), tenantID
}

func januaryInput() billing.InvoiceInput {
	return billing.InvoiceInput{
		PeriodLabel: "Januar 2026",
		AmountCents: 19900,
		DueDate:     timezone.NewDate(2026, time.January, 31),
		Status:      platformModel.InvoiceStatusOpen,
	}
}

func TestServiceInTenantTx_CreateListUpdateDelete(t *testing.T) {
	t.Parallel()

	service, tenantID := newDBService(t)
	ctx := context.Background()

	created, err := service.CreateInvoice(ctx, tenantID, januaryInput())
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.NotZero(t, created.ID)

	listed, err := service.ListInvoices(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "Januar 2026", listed[0].PeriodLabel)

	paidOn := timezone.NewDate(2026, time.February, 3)
	updated, err := service.UpdateInvoice(ctx, tenantID, created.ID, billing.InvoiceInput{
		PeriodLabel: "Januar 2026",
		AmountCents: 19900,
		DueDate:     timezone.NewDate(2026, time.January, 31),
		Status:      platformModel.InvoiceStatusPaid,
		PaidOn:      &paidOn,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.PaidOn)
	assert.Equal(t, "2026-02-03", updated.PaidOn.String())
	assert.False(t, updated.Overdue, "a paid invoice is never overdue")

	require.NoError(t, service.DeleteInvoice(ctx, tenantID, created.ID))

	empty, err := service.ListInvoices(ctx, tenantID)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// An invoice id from another school must read as "not found", never as a
// cross-tenant write.
func TestServiceInTenantTx_IsTenantIsolated(t *testing.T) {
	t.Parallel()

	service, tenantA := newDBService(t)
	db := testpkg.SetupTestDB(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)

	ctx := context.Background()

	created, err := service.CreateInvoice(ctx, tenantA, januaryInput())
	require.NoError(t, err)

	_, err = service.UpdateInvoice(ctx, tenantB, created.ID, januaryInput())
	assert.ErrorIs(t, err, billing.ErrInvoiceNotFound)

	assert.ErrorIs(t, service.DeleteInvoice(ctx, tenantB, created.ID),
		billing.ErrInvoiceNotFound)

	foreign, err := service.ListInvoices(ctx, tenantB)
	require.NoError(t, err)
	assert.Empty(t, foreign)

	// School A's invoice survived every attempt.
	own, err := service.ListInvoices(ctx, tenantA)
	require.NoError(t, err)
	assert.Len(t, own, 1)
}

// The unique index reaches the caller as an actionable German message rather
// than a driver error.
func TestServiceInTenantTx_DuplicateInvoiceNumber(t *testing.T) {
	t.Parallel()

	service, tenantID := newDBService(t)
	ctx := context.Background()

	first := januaryInput()
	first.InvoiceNumber = "R-2026-001"
	_, err := service.CreateInvoice(ctx, tenantID, first)
	require.NoError(t, err)

	second := januaryInput()
	second.PeriodLabel = "Februar 2026"
	second.DueDate = timezone.NewDate(2026, time.February, 28)
	second.InvoiceNumber = "R-2026-001"

	_, err = service.CreateInvoice(ctx, tenantID, second)
	assert.ErrorIs(t, err, billing.ErrInvoiceNumberTaken)
}

// GetOverview inside a real tenant transaction: no settings service wired, so
// only the schedule half is asserted here (the settings half has its own
// end-to-end coverage in api/billing).
func TestServiceInTenantTx_OverviewReadsCommittedInvoices(t *testing.T) {
	t.Parallel()

	service, tenantID := newDBService(t)
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	_, err := service.CreateInvoice(ctx, tenantID, januaryInput())
	require.NoError(t, err)

	require.NoError(t, tenantTx(t, db, tenantID, func(ctx context.Context) error {
		overview, overviewErr := service.GetOverview(ctx)
		if overviewErr != nil {
			return overviewErr
		}
		assert.Len(t, overview.Invoices, 1)
		assert.Equal(t, int64(19900), overview.OpenAmountCents)
		assert.True(t, overview.Configured, "an invoice alone means there is a contract")
		return nil
	}))
}
