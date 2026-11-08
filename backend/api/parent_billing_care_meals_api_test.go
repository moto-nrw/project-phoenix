package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The Mensa of the monthly run (#4686). The releases are written as the
// school's Mensa-Monatsfreigabe leaves them; the months lie ahead like the
// other monthly fixtures.

const careMealTariffsPath = "/traeger/care-meal-tariffs"

type careMealInvoiceBody struct {
	ContractID    int64 `json:"contract_id"`
	MealPositions []struct {
		MealMonth   string `json:"meal_month"`
		Mode        string `json:"mode"`
		Quantity    int    `json:"quantity"`
		UnitCents   int64  `json:"unit_cents"`
		AmountCents int64  `json:"amount_cents"`
	} `json:"meal_positions"`
	TotalCents int64 `json:"total_cents"`
}

type careMealRunBody struct {
	ID           int64                 `json:"id"`
	Invoices     []careMealInvoiceBody `json:"invoices"`
	PreviewCount int                   `json:"preview_count"`
	PreviewCents int64                 `json:"preview_cents"`
	MealUnbilled []struct {
		Reason   string `json:"reason"`
		MealDays int    `json:"meal_days"`
	} `json:"meal_unbilled"`
}

// releaseMealMonth stores a released Mensa month of the school with one
// child, as the OGS-Leitung's release leaves it; a second child of the
// month joins the same release.
func releaseMealMonth(t *testing.T, db *bun.DB, organizationID, schoolID, studentID int64, month, first string, meals int) {
	t.Helper()
	ctx := context.Background()
	var monthID int64
	_, err := db.ExecContext(ctx, `INSERT INTO schedule.meal_billing_months (organization_id, school_id, billing_month, child_count, meal_count)
		VALUES (?, ?, ?::date, 1, ?) ON CONFLICT (school_id, billing_month) DO NOTHING`, organizationID, schoolID, month, meals)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`SELECT id FROM schedule.meal_billing_months WHERE school_id = ? AND billing_month = ?::date`,
		schoolID, month).Scan(ctx, &monthID))
	_, err = db.ExecContext(ctx, `INSERT INTO schedule.meal_billing_month_children (organization_id, month_id, student_id, first_name,
		last_name, registered_days, meal_days) VALUES (?, ?, ?, ?, 'Beispiel', ?, ?)`,
		organizationID, monthID, studentID, first, max(meals, 1), meals)
	require.NoError(t, err)
}

func mealPrice(schoolID int64, validFrom, mode string, cents int64) map[string]any {
	return map[string]any{"school_id": schoolID, "valid_from": validFrom, "mode": mode, "amount_cents": cents}
}

// Hardt (MK-001) through the routes: November and December 2029 released
// with 3 and 2 meals at 4,80 € go onto Mia's January invoice next to her
// modules (77,50 €): 14,40 € + 9,60 €. The release issues them once; the
// February run bills them no more, and their price stays.
func TestCareMealMonthsAreBilledOnceOnTheChildsInvoice(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Lauf")
	db := testpkg.SetupTestDB(t)
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	careContract(t, f.db, f.office.organization, school, "Noah", "2030-02-01", sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-11-01", "per_meal", 480))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// Mia keeps her student identity across both monthly releases.
	miaID := testpkg.UniqueTestTenantID(t)
	releaseMealMonth(t, db, f.office.organization, school, miaID, "2029-11-01", "Mia", 3)
	releaseMealMonth(t, db, f.office.organization, school, miaID, "2029-12-01", "Mia", 2)
	releaseMealMonth(t, db, f.office.organization, school, testpkg.UniqueTestTenantID(t), "2029-12-01", "Ben", 4)

	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[careMealRunBody](t, rec.Body.Bytes())
	require.Len(t, preview.Invoices, 1)
	invoice := preview.Invoices[0]
	assert.Equal(t, contract, invoice.ContractID)
	require.Len(t, invoice.MealPositions, 2)
	assert.Equal(t, "2029-11-01", invoice.MealPositions[0].MealMonth)
	assert.Equal(t, 3, invoice.MealPositions[0].Quantity)
	assert.Equal(t, int64(1440), invoice.MealPositions[0].AmountCents)
	assert.Equal(t, int64(960), invoice.MealPositions[1].AmountCents)
	assert.Equal(t, int64(7750+1440+960), invoice.TotalCents)
	require.Len(t, preview.MealUnbilled, 1, "Ben has no contract at the school")
	assert.Equal(t, "no_contract", preview.MealUnbilled[0].Reason)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[careMealRunBody](t, rec.Body.Bytes())
	require.Len(t, released.Invoices, 1)
	assert.Len(t, released.Invoices[0].MealPositions, 2, "the issued invoice keeps its Mensa months")
	assert.Equal(t, int64(7750+1440+960), released.Invoices[0].TotalCents)

	next := f.startRun(t, "2030-02-01")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(next.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, invoice := range decodeSchoolPortalData[careMealRunBody](t, rec.Body.Bytes()).Invoices {
		assert.Empty(t, invoice.MealPositions, "a billed Mensa month is never billed again")
	}
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-12-01", "per_meal", 500))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_meal_tariff_month_billed")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2030-01-01", "flat", 5500))
	assert.Equal(t, http.StatusOK, rec.Code, "a later month gets a new price: %s", rec.Body.String())
}

// Without the right "Abrechnung" every Mensa route answers 403; another
// Träger neither sees nor sets the Mensa of this Träger's schools.
func TestCareMealPricesNeedTheRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Grenze")
	school := f.office.schools[0]
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-11-01", "per_meal", 480))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	for _, request := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, careMealTariffsPath, nil},
		{http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-12-01", "flat", 1)},
		{http.MethodDelete, careMealTariffsPath + fmt.Sprintf("/%d/2029-11-01", school), nil},
	} {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
	}

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Mensa Fremd", identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, careMealTariffsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), `"amount_cents":480`, "no Mensa price of another Träger")
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-12-01", "flat", 1))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_meal_tariff_invalid")
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodDelete, careMealTariffsPath+fmt.Sprintf("/%d/2029-11-01", school), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	for _, body := range []map[string]any{
		mealPrice(school, "2029-11-02", "flat", 1), mealPrice(school, "2029-11-01", "pauschal", 1), mealPrice(school, "2029-11-01", "flat", -1),
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

// The Mensa price list reads the same statements for any number of prices.
func TestCareMealPriceListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Budget")
	list := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, careMealTariffsPath, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(f.office.schools[0], "2029-11-01", "per_meal", 480))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	first := list()
	for _, school := range f.office.schools {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2030-01-01", "flat", 5500))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	assert.Len(t, list().Statements, len(first.Statements), "the Mensa prices read the same statements for any number of prices")
	querybudgets.AssertQueryBudget(t, "api.traeger.care_meal_tariffs", first)
}

// A Storno takes the Mensa months back with the invoice (ADR 0165): the next
// run of the month issues the invoice anew with the same Mensa months, and
// the reissue bills them once.
func TestCareMealMonthsReturnOnTheReissueAfterAStorno(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Storno")
	db := testpkg.SetupTestDB(t)
	brandCareTraeger(t, f)
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-12-01", "flat", 5500))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	releaseMealMonth(t, db, f.office.organization, school, testpkg.UniqueTestTenantID(t), "2029-12-01", "Mia", 1)

	run := f.startRun(t, careMonth)
	preview := f.run(t, run.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	issued := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes()).Invoices[0]
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost,
		careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", issued.ID)), map[string]any{"reason": "Falscher Betrag"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// A later month cannot replace January's invoice chain. It must still
	// release its own module invoices without picking up the cancelled meals.
	later := f.startRun(t, "2030-02-01")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(later.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	laterPreview := decodeSchoolPortalData[careMealRunBody](t, rec.Body.Bytes())
	require.Len(t, laterPreview.Invoices, 1)
	assert.Empty(t, laterPreview.Invoices[0].MealPositions, "cancelled meals belong to the original month's reissue")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(later.ID, "/release"),
		map[string]any{"document_count": laterPreview.PreviewCount, "total_cents": laterPreview.PreviewCents})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	again := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(again.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reissue := decodeSchoolPortalData[careMealRunBody](t, rec.Body.Bytes())
	require.Len(t, reissue.Invoices, 1)
	assert.Equal(t, contract, reissue.Invoices[0].ContractID)
	require.Len(t, reissue.Invoices[0].MealPositions, 1, "the reissue bills the Mensa month the Storno took back")
	assert.Equal(t, int64(5500), reissue.Invoices[0].MealPositions[0].AmountCents)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(again.ID, "/release"),
		map[string]any{"document_count": reissue.PreviewCount, "total_cents": reissue.PreviewCents})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var positions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.care_meal_positions WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &positions))
	assert.Equal(t, 2, positions, "the cancelled invoice and its reissue each keep their Mensa month")
}

// The monthly run asks for the attendance window of its schools from inside
// its Träger transaction, whose row security hides the settings of the
// schools. The binding reads each school's own retention in a read of its
// own instead of falling back to the default (#4686): 40 days before
// 8 November 2026 is 29 September, not the 365-day default.
func TestCareMealAttendanceWindowReadsTheSchoolsRetentionInTheTraegerTransaction(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Aufbewahrung")
	school := f.office.schools[0]
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.attendance_retention_days', '40'::jsonb)`, school)
	require.NoError(t, err)
	unit := testpkg.TenantRuntime(t, f.db)
	settings, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, f.db), unit)
	require.NoError(t, err)
	find := owners.MealAttendanceFrom(settings.Settings, func() calendar.Date { return calendar.NewDate(2026, 11, 8) })

	var cutoffs map[int64]calendar.Date
	require.NoError(t, testpkg.WithinTraeger(testpkg.ContextWithTenantRuntime(ctx, unit), f.office.organization,
		func(ctx context.Context) error {
			cutoffs, err = find(ctx, []int64{school})
			return err
		}))
	assert.Equal(t, map[int64]calendar.Date{school: calendar.NewDate(2026, 9, 29)}, cutoffs)
}
