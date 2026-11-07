package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The monthly contribution of the care contract (#4684). The months lie far
// ahead: a contract version never takes effect in the past (#4681), so the
// fixtures stay valid whatever day the suite runs.

const (
	careTariffsPath     = "/traeger/care-tariffs"
	careRunsPath        = "/traeger/care-billing-runs"
	careMonth           = "2030-01-01"
	careModuleUntil1400 = int64(9101)
	careModuleUntil1630 = int64(9102)
)

type careInvoiceBody struct {
	ID             int64  `json:"id"`
	Number         string `json:"number"`
	ContractID     int64  `json:"contract_id"`
	SchoolID       int64  `json:"school_id"`
	ChildFirstName string `json:"child_first_name"`
	Recipient      struct {
		FirstName string `json:"first_name"`
		Email     string `json:"email"`
	} `json:"recipient"`
	Positions []struct {
		ModuleName string `json:"module_name"`
		Weekdays   []struct {
			Weekday      string `json:"weekday"`
			MonthlyCents int64  `json:"monthly_cents"`
		} `json:"weekdays"`
		AmountCents int64 `json:"amount_cents"`
	} `json:"positions"`
	TotalCents int64 `json:"total_cents"`
}

type careRunBody struct {
	ID           int64             `json:"id"`
	BillingMonth string            `json:"billing_month"`
	Status       string            `json:"status"`
	MonthRule    string            `json:"month_rule"`
	Invoices     []careInvoiceBody `json:"invoices"`
	Unpriced     []struct {
		Weekday string `json:"weekday"`
	} `json:"unpriced"`
	Billed       int   `json:"billed"`
	PreviewCount int   `json:"preview_count"`
	PreviewCents int64 `json:"preview_cents"`
}

type careService struct {
	weekday  string
	offering int64
	module   string
	endsAt   string
}

// careContract stores an active contract with one effective version from
// effectiveFrom and its party, as a conclusion leaves it (#4680, #4681).
func careContract(t *testing.T, db *bun.DB, organizationID, schoolID int64, child, effectiveFrom string, services ...careService) int64 {
	t.Helper()
	ctx := context.Background()
	var contractID, versionID int64
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status,
			sent_at, concluded_at, conclusion_method)
		VALUES (?, ?, ?, 'aktiv', NOW(), NOW(), 'papier') RETURNING id`,
		organizationID, schoolID, testpkg.UniqueTestTenantID(t)).Scan(ctx, &contractID))
	require.NoError(t, db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			status, effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		VALUES (?, ?, 1, 'wirksam', ?::date, ?, 'Beispiel', 'Grundschule Ispringen', 'open_ended', 1, 'month_end',
			false, '', false, '', '', 'SJR Pforzheim') RETURNING id`,
		organizationID, contractID, effectiveFrom, child).Scan(ctx, &versionID))
	for _, service := range services {
		_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id,
				weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, ?, ?, ?, ?::time)`,
			organizationID, versionID, service.weekday, service.offering, service.module, service.endsAt)
		require.NoError(t, err)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO care_contract.contract_parties (organization_id, contract_id, first_name, last_name, email)
		VALUES (?, ?, 'Eva', 'Beispiel', ?)`, organizationID, contractID, fmt.Sprintf("eva.%d@example.org", contractID))
	require.NoError(t, err)
	return contractID
}

func sjrServices() []careService {
	return []careService{
		{"mon", careModuleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"},
		{"tue", careModuleUntil1630, "Betreuung bis 16:30 Uhr", "16:30"},
	}
}

// carePriceList sets the SJR prices of a school from valid on: per booked
// weekday and month 32 € until 14:00, 45,50 € until 16:30.
func carePriceList(schoolID int64, validFrom string, until1400 int64) map[string]any {
	prices := []map[string]any{}
	for _, weekday := range []string{"mon", "tue", "wed", "thu", "fri"} {
		prices = append(prices,
			map[string]any{"care_offering_id": careModuleUntil1400, "module_name": "Betreuung bis 14:00 Uhr", "weekday": weekday, "monthly_cents": until1400},
			map[string]any{"care_offering_id": careModuleUntil1630, "module_name": "Betreuung bis 16:30 Uhr", "weekday": weekday, "monthly_cents": 4550})
	}
	return map[string]any{"school_id": schoolID, "valid_from": validFrom, "prices": prices}
}

type careBillingFixture struct {
	db      *bun.DB
	traeger *traegerPortalHandler
	office  traegerOffice
}

func newCareBillingFixture(t *testing.T, name string) careBillingFixture {
	t.Helper()
	_, portal, _, _ := newTraegerPortal(t)
	db := testpkg.SetupTestDB(t)
	office := newTraegerOffice(t, db, portal, name, identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, db, office.organization)
	rec := traegerJSON(t, portal, office.token, http.MethodPut, numberRangesPath+"/care", map[string]any{"pattern": "MB-{JJJJ}-{NNNN}"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return careBillingFixture{db: db, traeger: portal, office: office}
}

func (f careBillingFixture) startRun(t *testing.T, month string) careRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunsPath, map[string]any{"billing_month": month})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
}

func (f careBillingFixture) run(t *testing.T, id int64) careRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
}

func careRunPath(id int64, suffix string) string {
	return careRunsPath + "/" + strconv.FormatInt(id, 10) + suffix
}

func careRelease(run careRunBody) map[string]any {
	return map[string]any{"document_count": run.PreviewCount, "total_cents": run.PreviewCents}
}

// The monthly run bills the SJR example through the routes: one position per
// module with its weekday, the sum of the booked weekdays, issued with a
// number; the invoice never changes and a later price never reaches back.
func TestCareMonthlyRunBillsTheContractByTheTariffsOfItsSchool(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsbeitrag")
	school, other := f.office.schools[0], f.office.schools[1]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// The other school of the Träger has its own price in the same run.
	otherContract := careContract(t, f.db, f.office.organization, other, "Noah", careMonth, sjrServices()[0])
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(other, "2029-09-01", 2800))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	run := f.startRun(t, careMonth)
	preview := f.run(t, run.ID)
	assert.Equal(t, "calculated", preview.Status)
	assert.Equal(t, "version_on_first", preview.MonthRule, "the default bills the full month by the version on the 1st")
	require.Len(t, preview.Invoices, 2)
	totals := map[int64]int64{}
	for _, invoice := range preview.Invoices {
		totals[invoice.ContractID] = invoice.TotalCents
		assert.Empty(t, invoice.Number, "a draft has no number")
	}
	assert.Equal(t, map[int64]int64{contract: 7750, otherContract: 2800}, totals)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"),
		map[string]any{"document_count": 2, "total_cents": 1})
	assert.Equal(t, http.StatusConflict, rec.Code, "a changed sum is refused: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
	assert.Equal(t, "released", released.Status)
	require.Len(t, released.Invoices, 2)
	var mia careInvoiceBody
	for _, invoice := range released.Invoices {
		assert.Regexp(t, `^MB-\d{4}-000[12]$`, invoice.Number)
		if invoice.ContractID == contract {
			mia = invoice
		}
	}
	require.Len(t, mia.Positions, 2, "one position per module, not per day")
	assert.Equal(t, "Betreuung bis 14:00 Uhr", mia.Positions[0].ModuleName)
	assert.Equal(t, "mon", mia.Positions[0].Weekdays[0].Weekday)
	assert.Equal(t, int64(3200), mia.Positions[0].AmountCents)
	assert.Equal(t, int64(4550), mia.Positions[1].AmountCents)
	assert.Equal(t, "Eva", mia.Recipient.FirstName)

	// A price of a billed month is refused; the next month takes it.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, careMonth, 9900))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_tariff_month_billed")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, careTariffsPath+fmt.Sprintf("/%d/2029-09-01", school), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2030-02-01", 3600))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	again := f.run(t, run.ID)
	assert.Equal(t, mia.TotalCents, again.Invoices[0].TotalCents+again.Invoices[1].TotalCents-totals[otherContract],
		"the issued invoice keeps its prices")

	// The issued invoice and its run never change, for any role.
	_, err := f.db.ExecContext(context.Background(), `UPDATE parent_billing.care_invoices SET total_cents = 1 WHERE id = ?`, mia.ID)
	require.Error(t, err)
	_, err = f.db.ExecContext(context.Background(), `UPDATE parent_billing.care_runs SET month_rule = 'pro_rata' WHERE id = ?`, run.ID)
	require.Error(t, err)
	// The database alone keeps a contract to one invoice per month.
	_, err = f.db.ExecContext(context.Background(), `INSERT INTO parent_billing.care_invoices (organization_id, run_id, number,
			number_year, number_sequence, issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name,
			child_last_name, recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
		SELECT organization_id, run_id, number || '-X', number_year, number_sequence + 100, issued_on, billing_month,
			care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_first_name,
			recipient_last_name, recipient_email, month_rule, total_cents FROM parent_billing.care_invoices WHERE id = ?`, mia.ID)
	require.ErrorContains(t, err, "uq_care_invoices_contract_month")

	// A second run of the month bills nobody twice.
	second := f.startRun(t, careMonth)
	detail := f.run(t, second.ID)
	assert.Empty(t, detail.Invoices)
	assert.Equal(t, 2, detail.Billed)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(second.ID, "/release"), careRelease(detail))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_run_events WHERE run_id = ? AND action = 'read'`, run.ID).
		Scan(context.Background(), &reads))
	assert.Equal(t, 2, reads, "every read of the run is in its trail")
}

// Two releases of one run in two transactions at once issue one invoice
// per contract and month; a second open run of a month is refused.
func TestCareMonthlyRunReleasedTwiceAtOnceBillsEveryChildOnce(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsbeitrag parallel")
	school := f.office.schools[0]
	for _, child := range []string{"Mia", "Noah", "Lea"} {
		careContract(t, f.db, f.office.organization, school, child, careMonth, sjrServices()...)
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunsPath, map[string]any{"billing_month": careMonth})
	assert.Equal(t, http.StatusConflict, rec.Code, "one open run per month: %s", rec.Body.String())
	preview := f.run(t, run.ID)
	require.Len(t, preview.Invoices, 3)

	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i] = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview)).Code
		}()
	}
	wg.Wait()
	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes)

	var invoices, contracts, sequences int
	require.NoError(t, f.db.NewRaw(`SELECT count(*), count(DISTINCT care_contract_id), count(DISTINCT number_sequence)
		FROM parent_billing.care_invoices WHERE organization_id = ? AND billing_month = ?::date`,
		f.office.organization, careMonth).Scan(context.Background(), &invoices, &contracts, &sequences))
	assert.Equal(t, 3, invoices)
	assert.Equal(t, 3, contracts)
	assert.Equal(t, 3, sequences, "numbers without a gap or a double")
}

// The monthly billing needs the right "Abrechnung" and stays with its
// Träger: another Träger sees neither its runs nor its prices.
func TestCareMonthlyBillingNeedsTheRightAndStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsbeitrag Grenze")
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := f.startRun(t, careMonth)
	preview := f.run(t, run.ID)

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	requests := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, careTariffsPath, nil},
		{http.MethodPut, careTariffsPath, carePriceList(school, "2029-10-01", 1)},
		{http.MethodDelete, careTariffsPath + fmt.Sprintf("/%d/2029-09-01", school), nil},
		{http.MethodGet, careRunsPath, nil},
		{http.MethodPost, careRunsPath, map[string]any{"billing_month": "2030-02-01"}},
		{http.MethodGet, careRunPath(run.ID, ""), nil},
		{http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview)},
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
	}

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Monatsbeitrag Fremd", identityaccess.TraegerRoleBilling)
	for _, request := range requests[5:] {
		rec := traegerJSON(t, f.traeger, foreign.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, careTariffsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Betreuung bis 14:00 Uhr", "no price or module of another Träger")
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, careRunsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[struct {
		Runs []careRunBody `json:"runs"`
	}](t, rec.Body.Bytes()).Runs)
	// A run of the other Träger never bills this Träger's contract.
	foreignRun := traegerJSON(t, f.traeger, foreign.token, http.MethodPost, careRunsPath, map[string]any{"billing_month": careMonth})
	require.Equal(t, http.StatusCreated, foreignRun.Code, foreignRun.Body.String())
	foreignDetail := traegerJSON(t, f.traeger, foreign.token, http.MethodGet,
		careRunPath(decodeSchoolPortalData[careRunBody](t, foreignRun.Body.Bytes()).ID, ""), nil)
	require.Equal(t, http.StatusOK, foreignDetail.Code, foreignDetail.Body.String())
	assert.Empty(t, decodeSchoolPortalData[careRunBody](t, foreignDetail.Body.Bytes()).Invoices)
}

// A price list starts on the first day of a month and names each module
// and weekday once.
func TestCarePriceListIsCheckedBeforeItIsSaved(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsbeitrag Preise")
	school := f.office.schools[0]
	bad := []map[string]any{
		carePriceList(school, "2029-09-02", 3200),
		carePriceList(0, "2029-09-01", 3200),
		carePriceList(school, "2029-09-01", -1),
		{"school_id": school, "valid_from": "2029-09-01", "prices": []map[string]any{
			{"care_offering_id": careModuleUntil1400, "module_name": "M", "weekday": "mon", "monthly_cents": 1},
			{"care_offering_id": careModuleUntil1400, "module_name": "M", "weekday": "mon", "monthly_cents": 2}}},
		{"school_id": school, "valid_from": "2029-09-01", "prices": []map[string]any{
			{"care_offering_id": careModuleUntil1400, "module_name": "M", "weekday": "montag", "monthly_cents": 1}}},
	}
	for _, body := range bad {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "parent_billing.care_tariff_invalid")
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunsPath, map[string]any{"billing_month": "2030-01-15"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_run_invalid")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, careTariffsPath+fmt.Sprintf("/%d/2029-09-01", school), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	// A school of another Träger gets no price of this Träger.
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Monatsbeitrag Preise Fremd", identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(foreign.schools[0], "2029-09-01", 3200))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_tariff_invalid")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careTariffsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "OGS Nord", "the price list names the schools of the Träger")
	assert.NotContains(t, rec.Body.String(), "Preise Fremd", "and no school of another Träger")
}

// The list of the monthly runs and the price list read the same statements
// for any number of runs, contracts and prices.
func TestCareMonthlyListsQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsbeitrag Budget")
	school := f.office.schools[0]
	list := func(path string) []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.startRun(t, careMonth)
	runs, tariffs := list(careRunsPath), list(careTariffsPath)
	careContract(t, f.db, f.office.organization, f.office.schools[1], "Noah", careMonth, sjrServices()...)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(f.office.schools[1], "2029-10-01", 2800))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.startRun(t, "2030-02-01")
	assert.Len(t, list(careRunsPath), len(runs), "the list reads the same statements for any number of runs")
	assert.Len(t, list(careTariffsPath), len(tariffs), "the price list reads the same statements for any number of prices")
	querybudgets.AssertQueryBudget(t, "api.traeger.care_billing_runs", runs)
	querybudgets.AssertQueryBudget(t, "api.traeger.care_tariffs", tariffs)
}

// The holiday and the monthly invoices take their numbers from two number
// ranges. With the same pattern in both, a holiday release that would
// repeat a number of a monthly invoice is refused and uses no number.
func TestHolidayAndMonthlyInvoicesNeverShareANumber(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Nummern Monat und Ferien")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, f.office.organization)
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, numberRangesPath+"/care", map[string]any{"pattern": "R-{NNNN}"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, careRunsPath, map[string]any{"billing_month": careMonth})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	careRun := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, careRunPath(careRun.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, careRunPath(careRun.ID, "/release"),
		careRelease(decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "R-0001", decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes()).Invoices[0].Number)

	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	setNumberRange(t, f, billing, "R-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.invoice_number_taken")
	var holidayDocuments int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.documents WHERE organization_id = ?`, f.office.organization).
		Scan(context.Background(), &holidayDocuments))
	assert.Zero(t, holidayDocuments, "the refused release issued nothing")

	setNumberRange(t, f, billing, "F-{NNNN}")
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, "another pattern releases: %s", rec.Body.String())
}
