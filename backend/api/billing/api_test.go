package billing_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	billingAPI "github.com/moto-nrw/project-phoenix/api/billing"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	billingSvc "github.com/moto-nrw/project-phoenix/services/billing"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type overviewResponse struct {
	Status string                      `json:"status"`
	Data   billingSvc.ContractOverview `json:"data"`
}

func setupContractTest(t *testing.T) (*bun.DB, *billingAPI.Resource, int64) {
	t.Helper()

	db, services := testutil.SetupAPITest(t)
	// Every test in this binary owns its tenant (PerTestTenants in TestMain)
	// inside a per-package database clone, so no per-row cleanup is needed.
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	return db, billingAPI.NewResource(services.Billing, services.Settings, db), tenantID
}

func seedInvoice(t *testing.T, db *bun.DB, tenantID int64, label string, cents int64, due timezone.Date, status string, note ...string) {
	t.Helper()

	storedNote := ""
	if len(note) > 0 {
		storedNote = note[0]
	}

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, `
				INSERT INTO platform.school_invoices
					(tenant_id, period_label, amount_cents, due_date, status, note)
				VALUES (?, ?, ?, ?, ?, ?)`,
				tenantID, label, cents, due.String(), status, storedNote)
			return err
		}))
}

func seedSetting(t *testing.T, db *bun.DB, tenantID int64, key string, value any) {
	t.Helper()

	encoded, err := json.Marshal(value)
	require.NoError(t, err)

	require.NoError(t, tenant.WithTenantTx(context.Background(), db, tenantID,
		func(ctx context.Context, tx bun.Tx) error {
			_, execErr := tx.ExecContext(ctx, `
				INSERT INTO config.setting_values (tenant_id, setting_key, value)
				VALUES (?, ?, ?::jsonb)
				ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value`,
				tenantID, key, string(encoded))
			return execErr
		}))
}

func adminClaims(tenantID int64) jwt.AppClaims {
	return testutil.AdminTestClaimsForTenant(1, tenantID)
}

// TestContractOverview_RequiresAuthentication is the baseline: the route sits
// behind the tenant session like every other /api route.
func TestContractOverview_RequiresAuthentication(t *testing.T) {
	t.Parallel()

	_, resource, _ := setupContractTest(t)

	rr := testutil.ExecuteRequest(resource.Router(),
		testutil.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestContractOverview_AcceptsMountedNoSlashPath(t *testing.T) {
	t.Parallel()

	_, resource, tenantID := setupContractTest(t)
	router := chi.NewRouter()
	resource.RegisterRoutes(router, "/contract")

	rr := testutil.ExecuteWithAuthPermissions(t, router,
		testutil.NewRequest(http.MethodGet, "/contract", nil),
		adminClaims(tenantID),
		[]string{"config:manage"})

	assert.Equal(t, http.StatusOK, rr.Code)
}

// Contract and payment data is commercial information about the school. A
// Betreuungskraft with day-to-day permissions must not see it.
func TestContractOverview_RequiresConfigManage(t *testing.T) {
	t.Parallel()

	_, resource, tenantID := setupContractTest(t)

	rr := testutil.ExecuteWithAuthPermissions(t, resource.Router(),
		testutil.NewRequest(http.MethodGet, "/", nil),
		adminClaims(tenantID),
		[]string{"students:read", "config:read"})

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestContractOverview_EmptyContract(t *testing.T) {
	t.Parallel()

	_, resource, tenantID := setupContractTest(t)

	rr := testutil.ExecuteWithAuthPermissions(t, resource.Router(),
		testutil.NewRequest(http.MethodGet, "/", nil),
		adminClaims(tenantID),
		[]string{"config:manage"})

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var response overviewResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))

	assert.False(t, response.Data.Configured)
	assert.Empty(t, response.Data.Invoices)
	assert.Nil(t, response.Data.NextDue)
	assert.Zero(t, response.Data.OpenAmountCents)
	assert.Equal(t, "Noch nicht hinterlegt", response.Data.TierLabel)
}

func TestContractOverview_ReturnsSettingsAndInvoices(t *testing.T) {
	t.Parallel()

	db, resource, tenantID := setupContractTest(t)

	seedSetting(t, db, tenantID, configModel.KeyContractTier, configModel.ContractTierPlus)
	seedSetting(t, db, tenantID, configModel.KeyContractBookedChildren, 150)
	seedSetting(t, db, tenantID, configModel.KeyContractPricePerChildCents, 200)
	seedSetting(t, db, tenantID, configModel.KeyContractBillingCycle, configModel.ContractCycleMonthly)
	seedSetting(t, db, tenantID, configModel.KeyContractTermStart, "2026-01-01")
	seedSetting(t, db, tenantID, configModel.KeyContractInvoiceRecipient, "buchhaltung@schule.test")
	seedSetting(t, db, tenantID, configModel.KeyContractCustomerNumber, "K-10023")
	seedSetting(t, db, tenantID, configModel.KeyContractSupportEmail, "rechnung@moto.test")
	seedSetting(t, db, tenantID, configModel.KeyContractNote, "Preis gilt bis Schuljahresende.")

	past := timezone.TodayDate().AddDays(-30)
	future := timezone.TodayDate().AddDays(30)
	seedInvoice(t, db, tenantID, "Vergangener Zeitraum", 10000, past, platformModel.InvoiceStatusOpen, "Nur intern")
	seedInvoice(t, db, tenantID, "Kommender Zeitraum", 20000, future, platformModel.InvoiceStatusOpen)

	rr := testutil.ExecuteWithAuthPermissions(t, resource.Router(),
		testutil.NewRequest(http.MethodGet, "/", nil),
		adminClaims(tenantID),
		[]string{"config:manage"})

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var response overviewResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))

	data := response.Data
	assert.True(t, data.Configured)
	assert.Equal(t, configModel.ContractTierPlus, data.Tier)
	assert.Equal(t, "Plus", data.TierLabel)
	assert.Equal(t, 150, data.BookedChildren)
	assert.Equal(t, 200, data.PricePerChildCents)
	assert.Equal(t, "Monatlich", data.BillingCycleLabel)
	require.NotNil(t, data.TermStart)
	assert.Equal(t, "2026-01-01", data.TermStart.String())
	assert.Nil(t, data.TermEnd, "an unset term end stays absent instead of becoming 0001-01-01")
	assert.Equal(t, "buchhaltung@schule.test", data.InvoiceRecipient)
	assert.Equal(t, "K-10023", data.CustomerNumber)
	assert.Equal(t, "rechnung@moto.test", data.SupportEmail)
	assert.Equal(t, "Preis gilt bis Schuljahresende.", data.Note)

	require.Len(t, data.Invoices, 2)
	assert.NotContains(t, rr.Body.String(), "Nur intern")
	assert.Equal(t, "Kommender Zeitraum", data.Invoices[0].PeriodLabel, "newest due date first")
	assert.Equal(t, int64(30000), data.OpenAmountCents)

	require.NotNil(t, data.NextDue)
	assert.Equal(t, "Vergangener Zeitraum", data.NextDue.PeriodLabel,
		"an already-missed invoice outranks the next upcoming one")
	assert.True(t, data.NextDue.Overdue)
}

// The overview counts children in status "active" only. A pending child (an
// approved enrolment before its start date) is not yet being cared for.
func TestContractOverview_CountsActiveChildrenOnly(t *testing.T) {
	t.Parallel()

	db, resource, tenantID := setupContractTest(t)

	testpkg.CreateTestStudent(t, db, "Aktiv", "Kind", "1a")

	inactive := testpkg.CreateTestStudent(t, db, "Inaktiv", "Kind", "1a")
	_, err := db.NewUpdate().
		Table("users.students").
		Set("status = ?", "inactive").
		Where("id = ?", inactive.ID).
		Exec(context.Background())
	require.NoError(t, err)

	rr := testutil.ExecuteWithAuthPermissions(t, resource.Router(),
		testutil.NewRequest(http.MethodGet, "/", nil),
		adminClaims(tenantID),
		[]string{"config:manage"})

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var response overviewResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))

	assert.Equal(t, 1, response.Data.ActiveChildren)
}

// The tenant boundary end to end: school B's session must never surface
// school A's invoices.
func TestContractOverview_IsTenantIsolated(t *testing.T) {
	t.Parallel()

	db, resource, tenantA := setupContractTest(t)

	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)

	seedInvoice(t, db, tenantA, "Nur für Schule A", 12345,
		timezone.NewDate(2026, time.January, 31), platformModel.InvoiceStatusOpen)

	rr := testutil.ExecuteWithAuthPermissions(t, resource.Router(),
		testutil.NewRequest(http.MethodGet, "/", nil),
		testutil.AdminTestClaimsForTenant(1, tenantB),
		[]string{"config:manage"})

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var response overviewResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))

	assert.Empty(t, response.Data.Invoices)
	assert.NotContains(t, rr.Body.String(), "Nur für Schule A")
}

// The router exposes GET only. A school must not be able to mark its own
// invoice paid, not even by calling the API directly.
func TestContractRouter_ExposesNoWriteRoutes(t *testing.T) {
	t.Parallel()

	_, resource, tenantID := setupContractTest(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		rr := testutil.ExecuteWithAuthPermissions(t, resource.Router(),
			testutil.NewRequest(method, "/", nil),
			adminClaims(tenantID),
			[]string{"config:manage", "admin:*"})

		assert.Equalf(t, http.StatusMethodNotAllowed, rr.Code,
			"%s must not be routable on the contract surface", method)
	}
}

// failingBillingService lets the handler's error branch be exercised without
// breaking the database the other tests in this binary share.
type failingBillingService struct{}

func (failingBillingService) GetOverview(context.Context) (*billingSvc.ContractOverview, error) {
	return nil, errors.New("database unavailable")
}

func (failingBillingService) ListInvoices(context.Context, int64) ([]billingSvc.InvoiceView, error) {
	return nil, errors.New("not used")
}

func (failingBillingService) CreateInvoice(context.Context, int64, billingSvc.InvoiceInput) (*billingSvc.InvoiceView, error) {
	return nil, errors.New("not used")
}

func (failingBillingService) UpdateInvoice(context.Context, int64, int64, billingSvc.InvoiceInput) (*billingSvc.InvoiceView, error) {
	return nil, errors.New("not used")
}

func (failingBillingService) DeleteInvoice(context.Context, int64, int64) error {
	return errors.New("not used")
}

// A failing read must be a 500 with a German message, never an empty contract
// that would read as "kein Vertrag hinterlegt".
func TestContractOverview_ServiceFailureIs500(t *testing.T) {
	t.Parallel()

	db, _ := testutil.SetupAPITest(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	resource := billingAPI.NewResource(failingBillingService{}, nil, db)

	rr := testutil.ExecuteWithAuthPermissions(t, resource.Router(),
		testutil.NewRequest(http.MethodGet, "/", nil),
		adminClaims(tenantID),
		[]string{"config:manage"})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Vertragsdaten konnten nicht geladen werden")
}

func TestNewResource_WorksWithoutSettingsService(t *testing.T) {
	t.Parallel()

	db, services := testutil.SetupAPITest(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	// nil settings disables the prefetch; the handler must still answer.
	resource := billingAPI.NewResource(services.Billing, nil, db)

	rr := testutil.ExecuteWithAuthPermissions(t, resource.Router(),
		testutil.NewRequest(http.MethodGet, "/", nil),
		adminClaims(tenantID),
		[]string{"config:manage"})

	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}
