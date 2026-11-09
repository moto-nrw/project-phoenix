package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
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
	Fingerprint  string                `json:"fingerprint"`
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
		VALUES (?, ?, ?::date, 1, ?) ON CONFLICT (school_id, billing_month, version) DO NOTHING`, organizationID, schoolID, month, meals)
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
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents, "fingerprint": preview.Fingerprint})
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
		map[string]any{"document_count": laterPreview.PreviewCount, "total_cents": laterPreview.PreviewCents, "fingerprint": laterPreview.Fingerprint})
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
		map[string]any{"document_count": reissue.PreviewCount, "total_cents": reissue.PreviewCents, "fingerprint": reissue.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var positions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.care_meal_positions WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &positions))
	assert.Equal(t, 2, positions, "the cancelled invoice and its reissue each keep their Mensa month")
}

// The monthly run asks for the attendance window of its schools from inside
// its Träger transaction, whose row security hides the settings of the
// schools. The binding reads each school's own retention in a read of its
// own instead of falling back to the default, and each school's deletion
// mark in a school transaction of its own (#4686): a school with 40 days
// keeps a later day than one with the 365-day default, and a mark the
// cleanup left holds whatever the retention says.
func TestCareMealAttendanceWindowReadsTheSchoolsRetentionInTheTraegerTransaction(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Aufbewahrung")
	short, long := f.office.schools[0], f.office.schools[1]
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.attendance_retention_days', '40'::jsonb)`, short)
	require.NoError(t, err)
	unit := testpkg.TenantRuntime(t, f.db)
	settings, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, f.db), unit)
	require.NoError(t, err)
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: f.db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	find := owners.MealAttendanceFrom(settings.Settings, owners.SchoolAttendanceMarks(presence))
	window := func() map[int64]calendar.Date {
		var cutoffs map[int64]calendar.Date
		require.NoError(t, testpkg.WithinTraeger(testpkg.ContextWithTenantRuntime(ctx, unit), f.office.organization,
			func(ctx context.Context) error {
				cutoffs, err = find(ctx, []int64{short, long})
				return err
			}))
		return cutoffs
	}
	before := window()
	assert.True(t, before[short].After(before[long]), "40 days keep a later first day than the 365-day default")

	_, err = f.db.ExecContext(ctx, `INSERT INTO active.attendance_retention_marks (school_id, deleted_before) VALUES (?, DATE '2099-01-01')`, long)
	require.NoError(t, err)
	assert.Equal(t, calendar.NewDate(2099, 1, 1), window()[long], "the deletion mark holds whatever the retention says")
}

// A school releases a Mensa month between the review and the release
// (#5916): Ben has no contract, so count and sum of the invoices stay the
// same and only the children the run leaves out change. The fingerprint
// covers the Mensa, so the release refuses the stale review.
func TestCareMealReleaseAfterTheReviewRefusesTheRunRelease(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Prüfung")
	db := testpkg.SetupTestDB(t)
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-11-01", "per_meal", 480))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := f.startRun(t, careMonth)
	reviewed := f.run(t, run.ID)

	releaseMealMonth(t, db, f.office.organization, school, testpkg.UniqueTestTenantID(t), "2029-12-01", "Ben", 4)
	now := f.run(t, run.ID)
	require.Equal(t, reviewed.PreviewCount, now.PreviewCount, "count and sum stay the same")
	require.Equal(t, reviewed.PreviewCents, now.PreviewCents)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(reviewed))
	assert.Equal(t, http.StatusConflict, rec.Code, "a Mensa release since the review: %s", rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(now))
	assert.Equal(t, http.StatusOK, rec.Code, "the review of the new state releases: %s", rec.Body.String())
}

// An excluded invoice is not issued, and neither are its Mensa months: Mia
// is excluded, so her released December stays unbilled and the next run
// bills it; Lea's invoice is issued with hers.
func TestCareMealExcludedChildGetsNoMensaPosition(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Ausschluss")
	db := testpkg.SetupTestDB(t)
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	careContract(t, f.db, f.office.organization, school, "Lea", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-11-01", "per_meal", 480))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	releaseMealMonth(t, db, f.office.organization, school, testpkg.UniqueTestTenantID(t), "2029-12-01", "Mia", 2)
	releaseMealMonth(t, db, f.office.organization, school, testpkg.UniqueTestTenantID(t), "2029-12-01", "Lea", 3)

	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/exclusions"),
		map[string]any{"contract_id": mia, "service": "care", "reason": "Rückfrage"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reviewed := f.run(t, run.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(reviewed))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	type issuedMeal struct {
		Contract int64 `bun:"care_contract_id"`
		Quantity int   `bun:"quantity"`
	}
	var issued []issuedMeal
	require.NoError(t, f.db.NewRaw(`SELECT i.care_contract_id, p.quantity FROM parent_billing.care_meal_positions p
		JOIN parent_billing.care_invoices i ON i.id = p.invoice_id WHERE i.run_id = ?`, run.ID).Scan(context.Background(), &issued))
	require.Len(t, issued, 1, "only Lea's Mensa is issued")
	assert.NotEqual(t, mia, issued[0].Contract)
	assert.Equal(t, 3, issued[0].Quantity)

	nextRun := f.startRun(t, "2030-02-01")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(nextRun.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	next := decodeSchoolPortalData[careMealRunBody](t, rec.Body.Bytes())
	var miaMeals int
	for _, invoice := range next.Invoices {
		if invoice.ContractID == mia {
			miaMeals = len(invoice.MealPositions)
		}
	}
	assert.Equal(t, 1, miaMeals, "the next run bills the Mensa of the excluded invoice")
}

// careMealPaymentRun is a run with the service and the payment of each
// invoice (#4689, ADR 0167).
type careMealPaymentRun struct {
	Invoices []struct {
		ContractID    int64  `json:"contract_id"`
		Service       string `json:"service"`
		Positions     []any  `json:"positions"`
		MealPositions []any  `json:"meal_positions"`
		TotalCents    int64  `json:"total_cents"`
		Payment       *struct {
			Method    string `json:"method"`
			DueOn     string `json:"due_on"`
			DueSource string `json:"due_source"`
		} `json:"payment"`
	} `json:"invoices"`
	PreviewCount int    `json:"preview_count"`
	PreviewCents int64  `json:"preview_cents"`
	Fingerprint  string `json:"fingerprint"`
}

// mealPaymentRun starts the January run of Mia (modules 77,50 €, December
// with 2 meals at 4,80 €) with the due dates of the run per payment method
// and reads it.
func mealPaymentRun(t *testing.T, f careBillingFixture, terms map[string]any) (int64, careMealPaymentRun) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-11-01", "per_meal", 480))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	if terms != nil {
		setTerms(t, f.traeger, f.office.token, "meal", terms)
	}
	if terms != nil && terms["payment_method"] == "direct_debit" {
		// A direct debit needs the creditor and the family's mandate (#4695).
		saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 1, "missing_mandate": "transfer"})
		createMandate(t, f, mandateBody(contract, "SJR-MENSA-1"))
	}
	releaseMealMonth(t, db, f.office.organization, school, testpkg.UniqueTestTenantID(t), "2029-12-01", "Mia", 2)
	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, runDueDatesPathOf("care", run.ID), dueDates("2030-02-02", "2030-02-04"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return run.ID, decodeSchoolPortalData[careMealPaymentRun](t, rec.Body.Bytes())
}

// releaseMealPaymentRun releases the run and reads the service, the payment
// and the Mensa positions of each issued invoice.
func releaseMealPaymentRun(t *testing.T, f careBillingFixture, runID int64, preview careMealPaymentRun) []struct {
	Service string `bun:"service"`
	Method  string `bun:"payment_method"`
	DueOn   string `bun:"due_on"`
	Meals   int    `bun:"meals"`
	Total   int64  `bun:"total_cents"`
} {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents, "fingerprint": preview.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var issued []struct {
		Service string `bun:"service"`
		Method  string `bun:"payment_method"`
		DueOn   string `bun:"due_on"`
		Meals   int    `bun:"meals"`
		Total   int64  `bun:"total_cents"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT i.service, i.payment_method, i.due_on::text AS due_on, i.total_cents,
		(SELECT count(*) FROM parent_billing.care_meal_positions m WHERE m.invoice_id = i.id) AS meals
		FROM parent_billing.care_invoices i WHERE i.run_id = ? ORDER BY i.number`, runID).Scan(context.Background(), &issued))
	return issued
}

// Without terms of its own the Mensa travels on the invoice of the care
// (ADR 0167 point 1): one invoice for Mia with modules and Mensa, paid and
// due as the care.
func TestCareMealsTravelOnTheCareInvoiceByDefault(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa gebündelt")
	runID, preview := mealPaymentRun(t, f, nil)
	require.Len(t, preview.Invoices, 1)
	invoice := preview.Invoices[0]
	assert.Equal(t, "care", invoice.Service)
	assert.Len(t, invoice.Positions, 2)
	assert.Len(t, invoice.MealPositions, 1)
	assert.Equal(t, int64(7750+960), invoice.TotalCents)
	require.NotNil(t, invoice.Payment)
	assert.Equal(t, "transfer", invoice.Payment.Method)
	assert.Equal(t, "2030-02-02", invoice.Payment.DueOn)

	issued := releaseMealPaymentRun(t, f, runID, preview)
	require.Len(t, issued, 1)
	assert.Equal(t, "care", issued[0].Service)
	assert.Equal(t, 1, issued[0].Meals)
	assert.Equal(t, int64(7750+960), issued[0].Total)
}

// A Träger that bills the Mensa apart (bundle_with empty) gets an invoice of
// its own for it, paid by direct debit and due on the run's date for direct
// debits; the care's invoice keeps the modules, transfer and its date.
func TestCareMealsGoOnAnInvoiceOfTheirOwnWithTheirTerms(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa eigene Rechnung")
	runID, preview := mealPaymentRun(t, f, map[string]any{"payment_method": "direct_debit", "due_days": 30, "bundle_with": ""})
	require.Len(t, preview.Invoices, 2, "the care and the Mensa of Mia")
	care, meal := preview.Invoices[0], preview.Invoices[1]
	assert.Equal(t, "care", care.Service)
	assert.Equal(t, int64(7750), care.TotalCents)
	assert.Empty(t, care.MealPositions)
	require.NotNil(t, care.Payment)
	assert.Equal(t, "transfer", care.Payment.Method)
	assert.Equal(t, "2030-02-02", care.Payment.DueOn)
	assert.Equal(t, "meal", meal.Service)
	assert.Equal(t, care.ContractID, meal.ContractID)
	assert.Empty(t, meal.Positions)
	assert.Len(t, meal.MealPositions, 1)
	assert.Equal(t, int64(960), meal.TotalCents)
	require.NotNil(t, meal.Payment)
	assert.Equal(t, "direct_debit", meal.Payment.Method)
	assert.Equal(t, "2030-02-04", meal.Payment.DueOn)
	assert.Equal(t, int64(7750+960), preview.PreviewCents)

	issued := releaseMealPaymentRun(t, f, runID, preview)
	require.Len(t, issued, 2)
	assert.Equal(t, "care", issued[0].Service)
	assert.Equal(t, "transfer", issued[0].Method)
	assert.Zero(t, issued[0].Meals)
	assert.Equal(t, "meal", issued[1].Service)
	assert.Equal(t, "direct_debit", issued[1].Method)
	assert.Equal(t, "2030-02-04", issued[1].DueOn)
	assert.Equal(t, 1, issued[1].Meals, "the Mensa positions stay with the Mensa invoice")
	assert.Equal(t, int64(960), issued[1].Total)
}

// A Storno of an invoice with modules and Mensa, then the Mensa billed apart
// (ADR 0167), then the reissue of the month: the Mensa month moves onto an
// invoice of its own and still follows its chain, so the release succeeds
// and the new position names the cancelled invoice as its predecessor.
func TestCareMealReissueKeepsItsChainWhenTheMensaMovesOntoItsOwnInvoice(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Wechsel")
	db := testpkg.SetupTestDB(t)
	brandCareTraeger(t, f)
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-12-01", "flat", 5500))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	releaseMealMonth(t, db, f.office.organization, school, testpkg.UniqueTestTenantID(t), "2029-12-01", "Mia", 1)

	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(f.run(t, run.ID)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	issued := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes()).Invoices[0]
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost,
		careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", issued.ID)), map[string]any{"reason": "Falscher Betrag"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	setTerms(t, f.traeger, f.office.token, "meal", map[string]any{"payment_method": "transfer", "due_days": 14, "bundle_with": ""})
	again := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(again.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reissue := decodeSchoolPortalData[careMealPaymentRun](t, rec.Body.Bytes())
	require.Len(t, reissue.Invoices, 2, "the care and the Mensa of Mia, each an invoice of its own")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(again.ID, "/release"),
		map[string]any{"document_count": reissue.PreviewCount, "total_cents": reissue.PreviewCents, "fingerprint": reissue.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, "the release does not collide with the cancelled Mensa month: %s", rec.Body.String())

	var replaces []int64
	require.NoError(t, db.NewRaw(`SELECT p.replaces_invoice_id FROM parent_billing.care_meal_positions AS p
		JOIN parent_billing.care_invoices AS i ON i.id = p.invoice_id WHERE i.run_id = ?`, again.ID).Scan(context.Background(), &replaces))
	assert.Equal(t, []int64{issued.ID}, replaces, "the Mensa position replaces the cancelled invoice")
}

// releaseMealMonthAgain stores a later version of a released Mensa month
// with one child, as the OGS-Leitung's release after a correction leaves it.
func releaseMealMonthAgain(t *testing.T, db *bun.DB, organizationID, schoolID, studentID int64, month, first string, meals, version int) {
	t.Helper()
	ctx := context.Background()
	var monthID int64
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.meal_billing_months (organization_id, school_id, billing_month, child_count,
		meal_count, version, reason) VALUES (?, ?, ?::date, 1, ?, ?, 'Anwesenheit nachgetragen') RETURNING id`,
		organizationID, schoolID, month, meals, version).Scan(ctx, &monthID))
	_, err := db.ExecContext(ctx, `INSERT INTO schedule.meal_billing_month_children (organization_id, month_id, student_id, first_name,
		last_name, registered_days, meal_days) VALUES (?, ?, ?, ?, 'Beispiel', ?, ?)`, organizationID, monthID, studentID, first, meals, meals)
	require.NoError(t, err)
}

// A correction after the issue (#4686): December billed with 2 meals
// (9,60 €), then the school releases it again with 3. The issued invoice
// stays; the next run names the change, and after the Storno the reissue
// of January bills the latest release: 3 meals, 14,40 €.
func TestCareMealReleaseAgainReachesTheReissueAfterAStorno(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Korrektur")
	db := testpkg.SetupTestDB(t)
	brandCareTraeger(t, f)
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-12-01", "per_meal", 480))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	mia := testpkg.UniqueTestTenantID(t)
	releaseMealMonth(t, db, f.office.organization, school, mia, "2029-12-01", "Mia", 2)

	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(f.run(t, run.ID)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	issued := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes()).Invoices[0]

	releaseMealMonthAgain(t, db, f.office.organization, school, mia, "2029-12-01", "Mia", 3, 2)
	later := f.startRun(t, "2030-02-01")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(later.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	changed := decodeSchoolPortalData[struct {
		MealChanged []struct {
			InvoiceNumber  string `json:"invoice_number"`
			BilledQuantity int    `json:"billed_quantity"`
			BilledCents    int64  `json:"billed_cents"`
			NewQuantity    int    `json:"new_quantity"`
			NewCents       int64  `json:"new_cents"`
		} `json:"meal_changed"`
	}](t, rec.Body.Bytes())
	require.Len(t, changed.MealChanged, 1, "the office sees the release changed after the issue")
	assert.Equal(t, issued.Number, changed.MealChanged[0].InvoiceNumber)
	assert.Equal(t, int64(960), changed.MealChanged[0].BilledCents)
	assert.Equal(t, int64(1440), changed.MealChanged[0].NewCents)
	var stillIssued int64
	require.NoError(t, db.NewRaw(`SELECT amount_cents FROM parent_billing.care_meal_positions WHERE invoice_id = ?`, issued.ID).
		Scan(context.Background(), &stillIssued))
	assert.Equal(t, int64(960), stillIssued, "the issued invoice never changes")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost,
		careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", issued.ID)), map[string]any{"reason": "Mensa korrigiert"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	again := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(again.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reissue := decodeSchoolPortalData[careMealRunBody](t, rec.Body.Bytes())
	require.Len(t, reissue.Invoices, 1)
	require.Len(t, reissue.Invoices[0].MealPositions, 1)
	assert.Equal(t, 3, reissue.Invoices[0].MealPositions[0].Quantity)
	assert.Equal(t, int64(1440), reissue.Invoices[0].MealPositions[0].AmountCents, "the reissue bills the latest release")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(again.ID, "/release"),
		map[string]any{"document_count": reissue.PreviewCount, "total_cents": reissue.PreviewCents, "fingerprint": reissue.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var version int
	require.NoError(t, db.NewRaw(`SELECT p.release_version FROM parent_billing.care_meal_positions AS p
		JOIN parent_billing.care_invoices AS i ON i.id = p.invoice_id WHERE i.run_id = ?`, again.ID).Scan(context.Background(), &version))
	assert.Equal(t, 2, version)
}

// A cost bearer share covers the modules, the Mensa or both (#4686, ADR 0168
// point 12). Mia books Mon until 14:00 and Tue until 16:30 (77,50 €) and ate
// 2 × 4,80 € = 9,60 € in December. Bildung und Teilhabe takes the lunch in
// full, the Jugendamt half the care (38,75 €): the family pays 38,75 €. The
// issued invoice keeps each share with its scope, and the list for Bildung
// und Teilhabe shows the Mensa as its base.
func TestCareCostSharesTakeTheMensaByTheirScope(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Kostenträger")
	db := testpkg.SetupTestDB(t)
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careMealTariffsPath, mealPrice(school, "2029-12-01", "per_meal", 480))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	releaseMealMonth(t, db, f.office.organization, school, testpkg.UniqueTestTenantID(t), "2029-12-01", "Mia", 2)
	jobcenter := f.costBearer(t, "Jobcenter Musterkreis")
	youthOffice := f.costBearer(t, "Jugendamt Musterkreis")
	f.saveCostShare(t, careCostShare(mia, "education_participation", jobcenter, map[string]any{"share_basis_points": 10000, "scope": "meal"}))
	f.saveCostShare(t, careCostShare(mia, "youth_office", youthOffice, map[string]any{"share_basis_points": 5000}))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careCostSharesPath,
		careCostShare(mia, "other_subsidy", youthOffice, map[string]any{"share_basis_points": 100, "scope": "lunch"}))
	require.Equal(t, http.StatusBadRequest, rec.Code, "an unknown scope is refused: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_cost_share_invalid")

	type scopedShare struct {
		Kind        string `json:"kind"`
		Scope       string `json:"scope"`
		AmountCents int64  `json:"amount_cents"`
	}
	type scopedRun struct {
		careRunBody
		Invoices []struct {
			Number     string        `json:"number"`
			CostShares []scopedShare `json:"cost_shares"`
			TotalCents int64         `json:"total_cents"`
		} `json:"invoices"`
	}
	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[scopedRun](t, rec.Body.Bytes())
	require.Len(t, preview.Invoices, 1)
	want := []scopedShare{{"education_participation", "meal", 960}, {"youth_office", "care", 3875}}
	assert.Equal(t, want, preview.Invoices[0].CostShares)
	assert.Equal(t, int64(3875), preview.Invoices[0].TotalCents, "77,50 + 9,60 − 9,60 − 38,75")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[scopedRun](t, rec.Body.Bytes())
	require.Len(t, released.Invoices, 1)
	assert.Equal(t, want, released.Invoices[0].CostShares, "the issued invoice keeps the scope of each share")
	assert.Equal(t, int64(3875), released.Invoices[0].TotalCents)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		careRunPath(run.ID, "/cost-shares/"+strconv.FormatInt(jobcenter, 10)+"/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), released.Invoices[0].Number+";")
	assert.Contains(t, rec.Body.String(), ";9,60;100 % für die Mensa;9,60;Ausgestellt")
}

// careExclusionRun is a run as the review of single invoices needs it
// (#6346): each invoice with its service and exclusion, the sums and the
// fingerprint.
type careExclusionRun struct {
	Invoices []struct {
		ContractID int64  `json:"contract_id"`
		Service    string `json:"service"`
		TotalCents int64  `json:"total_cents"`
		Exclusion  *struct {
			Reason string `json:"reason"`
		} `json:"exclusion"`
	} `json:"invoices"`
	PreviewCount  int    `json:"preview_count"`
	PreviewCents  int64  `json:"preview_cents"`
	ExcludedCount int    `json:"excluded_count"`
	ExcludedCents int64  `json:"excluded_cents"`
	Fingerprint   string `json:"fingerprint"`
}

// excludedServices names the services of the excluded invoices.
func (run careExclusionRun) excludedServices() []string {
	services := []string{}
	for _, invoice := range run.Invoices {
		if invoice.Exclusion != nil {
			services = append(services, invoice.Service)
		}
	}
	return services
}

func decodeExclusionRun(t *testing.T, rec *httptest.ResponseRecorder) careExclusionRun {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careExclusionRun](t, rec.Body.Bytes())
}

// Mia's care (77,50 €) and her Mensa on an invoice of its own (9,60 €) are
// two invoices of one contract. Excluding one of them leaves out that one
// only (#6346, decision A): in the preview and its sums, after a reload, in
// the release and in the next run. Before #6346 the exclusion held back the
// whole contract, so the care invoice left the preview with the Mensa.
func TestCareMealExclusionLeavesOutOnlyTheChosenInvoice(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mensa Ausschluss je Rechnung")
	runID, preview := mealPaymentRun(t, f, map[string]any{"payment_method": "transfer", "due_days": 14, "bundle_with": ""})
	require.Len(t, preview.Invoices, 2, "the care and the Mensa of Mia")
	mia := preview.Invoices[0].ContractID
	exclude := func(service, reason string) *httptest.ResponseRecorder {
		return traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runID, "/exclusions"),
			map[string]any{"contract_id": mia, "service": service, "reason": reason})
	}
	include := func(service string) *httptest.ResponseRecorder {
		return traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runID, "/inclusions"),
			map[string]any{"contract_id": mia, "service": service})
	}
	read := func() careExclusionRun {
		return decodeExclusionRun(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(runID, ""), nil))
	}

	excluded := decodeExclusionRun(t, exclude("meal", "Essenszahl wird geprüft"))
	assert.Equal(t, []string{"meal"}, excluded.excludedServices(), "only the Mensa invoice is excluded")
	assert.Equal(t, 1, excluded.PreviewCount, "the care invoice stays in the release")
	assert.Equal(t, int64(7750), excluded.PreviewCents)
	assert.Equal(t, 1, excluded.ExcludedCount)
	assert.Equal(t, int64(960), excluded.ExcludedCents)
	assert.Equal(t, excluded, read(), "a reload shows the same scope")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents, "fingerprint": preview.Fingerprint})
	assert.Equal(t, http.StatusConflict, rec.Code, "a release of the state before the exclusion: %s", rec.Body.String())

	rec = include("care")
	assert.Equal(t, http.StatusConflict, rec.Code, "the care invoice is not excluded: %s", rec.Body.String())
	returned := decodeExclusionRun(t, include("meal"))
	assert.Empty(t, returned.excludedServices())
	assert.Equal(t, int64(7750+960), returned.PreviewCents)
	assert.Equal(t, preview.Fingerprint, returned.Fingerprint, "the return restores the reviewed state")

	excluded = decodeExclusionRun(t, exclude("care", "Kündigung wird geprüft"))
	assert.Equal(t, []string{"care"}, excluded.excludedServices(), "only the care invoice is excluded")
	assert.Equal(t, 1, excluded.PreviewCount)
	assert.Equal(t, int64(960), excluded.PreviewCents, "the Mensa invoice stays in the release")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runID, "/release"),
		map[string]any{"document_count": excluded.PreviewCount, "total_cents": excluded.PreviewCents, "fingerprint": excluded.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var issued []string
	require.NoError(t, f.db.NewRaw(`SELECT service FROM parent_billing.care_invoices WHERE run_id = ? ORDER BY service`, runID).
		Scan(context.Background(), &issued))
	assert.Equal(t, []string{"meal"}, issued, "the release issues the Mensa invoice and not the excluded care invoice")

	next := f.startRun(t, careMonth)
	again := decodeExclusionRun(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(next.ID, ""), nil))
	require.Len(t, again.Invoices, 1, "the next run of the month bills the excluded invoice only")
	assert.Equal(t, "care", again.Invoices[0].Service)
	assert.Equal(t, int64(7750), again.Invoices[0].TotalCents)
}
