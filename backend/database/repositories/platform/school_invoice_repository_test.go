package platform_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupInvoiceRepoTest gives each test its own tenant inside the package's
// database clone (PerTestTenants in TestMain), so no per-row cleanup is needed.
func setupInvoiceRepoTest(t *testing.T) (*bun.DB, platformModels.SchoolInvoiceRepository, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	return db, platformRepo.NewSchoolInvoiceRepository(db), tenantID
}

func newInvoice(label string, cents int64, due timezone.Date) *platformModels.SchoolInvoice {
	return &platformModels.SchoolInvoice{
		PeriodLabel: label,
		AmountCents: cents,
		DueDate:     due,
		Status:      platformModels.InvoiceStatusOpen,
	}
}

func TestSchoolInvoiceRepository_CreateAndFind(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	invoice := newInvoice("Januar 2026", 19900, timezone.NewDate(2026, time.January, 31))
	invoice.InvoiceNumber = fmt.Sprintf("R-%d", testpkg.UniqueSuffix())
	invoice.Note = "Erste Rechnung"

	err := tenant.WithTenantTx(context.Background(), db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if createErr := repo.Create(ctx, invoice); createErr != nil {
			return createErr
		}

		stored, findErr := repo.FindByIDOrNil(ctx, invoice.ID)
		if findErr != nil {
			return findErr
		}
		require.NotNil(t, stored)

		assert.Equal(t, tenantID, stored.GetTenantID(), "tenant_id is stamped from the transaction")
		assert.Equal(t, "Januar 2026", stored.PeriodLabel)
		assert.Equal(t, int64(19900), stored.AmountCents)
		assert.Equal(t, platformModels.InvoiceStatusOpen, stored.Status)
		assert.Nil(t, stored.PaidOn)
		assert.Equal(t, "Erste Rechnung", stored.Note)
		return nil
	})
	require.NoError(t, err)

	assert.NotZero(t, invoice.ID)
}

// The whole point of timezone.Date: a 31 January due date must come back as
// 31 January, not 30 January, whatever the wall clock says when the test runs.
func TestSchoolInvoiceRepository_DatesRoundTripAsCalendarDays(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	due := timezone.NewDate(2026, time.January, 1)
	paid := timezone.NewDate(2026, time.January, 2)

	invoice := newInvoice("Januar 2026", 1000, due)
	invoice.Status = platformModels.InvoiceStatusPaid
	invoice.PaidOn = &paid

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			if createErr := repo.Create(ctx, invoice); createErr != nil {
				return createErr
			}
			stored, findErr := repo.FindByIDOrNil(ctx, invoice.ID)
			if findErr != nil {
				return findErr
			}
			require.NotNil(t, stored)
			assert.Equal(t, "2026-01-01", stored.DueDate.String())
			require.NotNil(t, stored.PaidOn)
			assert.Equal(t, "2026-01-02", stored.PaidOn.String())
			return nil
		}))
}

func TestSchoolInvoiceRepository_ListForTenant_OrdersByDueDateDescending(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			for _, spec := range []struct {
				label string
				due   timezone.Date
			}{
				{"Februar 2026", timezone.NewDate(2026, time.February, 28)},
				{"April 2026", timezone.NewDate(2026, time.April, 30)},
				{"Januar 2026", timezone.NewDate(2026, time.January, 31)},
			} {
				if err := repo.Create(ctx, newInvoice(spec.label, 100, spec.due)); err != nil {
					return err
				}
			}

			invoices, err := repo.ListForTenant(ctx)
			if err != nil {
				return err
			}

			require.Len(t, invoices, 3)
			assert.Equal(t, "April 2026", invoices[0].PeriodLabel)
			assert.Equal(t, "Februar 2026", invoices[1].PeriodLabel)
			assert.Equal(t, "Januar 2026", invoices[2].PeriodLabel)
			return nil
		}))
}

// Two invoices on the same due date must come back in a stable order — an
// unstable list would reshuffle on every page load.
func TestSchoolInvoiceRepository_ListForTenant_TiebreaksByIDDescending(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	due := timezone.NewDate(2026, time.May, 31)
	var firstID, secondID int64

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			first := newInvoice("Mai 2026 (a)", 100, due)
			if err := repo.Create(ctx, first); err != nil {
				return err
			}
			firstID = first.ID

			second := newInvoice("Mai 2026 (b)", 200, due)
			if err := repo.Create(ctx, second); err != nil {
				return err
			}
			secondID = second.ID

			invoices, err := repo.ListForTenant(ctx)
			if err != nil {
				return err
			}
			require.Len(t, invoices, 2)
			assert.Equal(t, secondID, invoices[0].ID)
			assert.Equal(t, firstID, invoices[1].ID)
			return nil
		}))
}

func TestSchoolInvoiceRepository_ListForTenant_EmptyForNewSchool(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			invoices, err := repo.ListForTenant(ctx)
			if err != nil {
				return err
			}
			assert.Empty(t, invoices)
			return nil
		}))
}

func TestSchoolInvoiceRepository_UpdateAndDelete(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	paid := timezone.NewDate(2026, time.February, 3)

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			invoice := newInvoice("Januar 2026", 19900, timezone.NewDate(2026, time.January, 31))
			if err := repo.Create(ctx, invoice); err != nil {
				return err
			}

			invoice.Status = platformModels.InvoiceStatusPaid
			invoice.PaidOn = &paid
			if err := repo.Update(ctx, invoice); err != nil {
				return err
			}

			stored, err := repo.FindByIDOrNil(ctx, invoice.ID)
			if err != nil {
				return err
			}
			require.NotNil(t, stored)
			assert.Equal(t, platformModels.InvoiceStatusPaid, stored.Status)
			require.NotNil(t, stored.PaidOn)
			assert.Equal(t, "2026-02-03", stored.PaidOn.String())

			if err := repo.Delete(ctx, invoice.ID); err != nil {
				return err
			}

			gone, err := repo.FindByIDOrNil(ctx, invoice.ID)
			if err != nil {
				return err
			}
			assert.Nil(t, gone)
			return nil
		}))
}

func TestSchoolInvoiceRepository_FindByIDOrNil_UnknownIDIsNil(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			stored, err := repo.FindByIDOrNil(ctx, int64(999_999_999))
			if err != nil {
				return err
			}
			assert.Nil(t, stored)
			return nil
		}))
}

// The tenant boundary: school B must not see, read or delete school A's
// invoices. RLS (migration 1.15.332) plus the repository's tenant filter.
func TestSchoolInvoiceRepository_IsTenantIsolated(t *testing.T) {
	t.Parallel()

	db, repo, tenantA := setupInvoiceRepoTest(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)

	var invoiceID int64

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantA,
		func(ctx context.Context, _ bun.Tx) error {
			invoice := newInvoice("Januar 2026", 19900, timezone.NewDate(2026, time.January, 31))
			if err := repo.Create(ctx, invoice); err != nil {
				return err
			}
			invoiceID = invoice.ID
			return nil
		}))
	require.NotZero(t, invoiceID)

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantB,
		func(ctx context.Context, _ bun.Tx) error {
			invoices, err := repo.ListForTenant(ctx)
			if err != nil {
				return err
			}
			assert.Empty(t, invoices, "school B must not see school A's invoices")

			stored, err := repo.FindByIDOrNil(ctx, invoiceID)
			if err != nil {
				return err
			}
			assert.Nil(t, stored, "a foreign invoice id must read as not found")
			return nil
		}))

	// And school A still has its row: B's attempts changed nothing.
	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantA,
		func(ctx context.Context, _ bun.Tx) error {
			invoices, err := repo.ListForTenant(ctx)
			if err != nil {
				return err
			}
			assert.Len(t, invoices, 1)
			return nil
		}))
}

// The unique index only applies to invoices that carry a number: several
// not-yet-numbered invoices per school must remain possible.
func TestSchoolInvoiceRepository_InvoiceNumberUniquePerTenant(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	number := fmt.Sprintf("R-%d", testpkg.UniqueSuffix())

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			first := newInvoice("Januar 2026", 100, timezone.NewDate(2026, time.January, 31))
			first.InvoiceNumber = number
			return repo.Create(ctx, first)
		}))

	err := tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			duplicate := newInvoice("Februar 2026", 100, timezone.NewDate(2026, time.February, 28))
			duplicate.InvoiceNumber = number
			return repo.Create(ctx, duplicate)
		})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uq_school_invoices_number")
}

func TestSchoolInvoiceRepository_AllowsSeveralUnnumberedInvoices(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			for _, label := range []string{"Januar 2026", "Februar 2026"} {
				invoice := newInvoice(label, 100, timezone.NewDate(2026, time.January, 31))
				if err := repo.Create(ctx, invoice); err != nil {
					return err
				}
			}
			invoices, err := repo.ListForTenant(ctx)
			if err != nil {
				return err
			}
			assert.Len(t, invoices, 2)
			return nil
		}))
}

// The model's Validate runs inside base.Repository.Create, so an invalid row
// never reaches PostgreSQL.
func TestSchoolInvoiceRepository_Create_RejectsInvalidRow(t *testing.T) {
	t.Parallel()

	db, repo, tenantID := setupInvoiceRepoTest(t)

	err := tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, _ bun.Tx) error {
			invalid := newInvoice("", 100, timezone.NewDate(2026, time.January, 31))
			return repo.Create(ctx, invalid)
		})

	require.Error(t, err)
	assert.ErrorIs(t, err, platformModels.ErrInvoicePeriodLabelRequired)
}

// Defense in depth: even if Validate were bypassed, the DB CHECK constraint
// rejects a paid invoice without a payment date.
func TestSchoolInvoiceRepository_DatabaseRejectsPaidWithoutDate(t *testing.T) {
	t.Parallel()

	db, _, tenantID := setupInvoiceRepoTest(t)

	err := tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, tx bun.Tx) error {
			_, execErr := tx.ExecContext(ctx, `
				INSERT INTO platform.school_invoices
					(tenant_id, period_label, amount_cents, due_date, status)
				VALUES (?, 'Januar 2026', 100, DATE '2026-01-31', 'bezahlt')`, tenantID)
			return execErr
		})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "chk_school_invoice_paid_on")
}

func TestSchoolInvoiceRepository_DatabaseRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	db, _, tenantID := setupInvoiceRepoTest(t)

	err := tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, tx bun.Tx) error {
			_, execErr := tx.ExecContext(ctx, `
				INSERT INTO platform.school_invoices
					(tenant_id, period_label, amount_cents, due_date, status)
				VALUES (?, 'Januar 2026', 100, DATE '2026-01-31', 'überfällig')`, tenantID)
			return execErr
		})

	require.Error(t, err, "overdue is derived, never stored")
}

func TestSchoolInvoiceRepository_DatabaseRejectsNegativeAmount(t *testing.T) {
	t.Parallel()

	db, _, tenantID := setupInvoiceRepoTest(t)

	err := tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, tx bun.Tx) error {
			_, execErr := tx.ExecContext(ctx, `
				INSERT INTO platform.school_invoices
					(tenant_id, period_label, amount_cents, due_date, status)
				VALUES (?, 'Januar 2026', -1, DATE '2026-01-31', 'offen')`, tenantID)
			return execErr
		})

	require.Error(t, err)
}
