package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The Jahresbescheid of the monthly care (#6219, ADR 0177) through the
// routes. The year lies far ahead like the monthly fixtures (#4681).

const careNoticeRunsPath = "/traeger/care-notice-runs"

type careNoticeBody struct {
	ID         int64  `json:"id"`
	Number     string `json:"number"`
	Kind       string `json:"kind"`
	ContractID int64  `json:"contract_id"`
	Lines      []struct {
		Label       string `json:"label"`
		Months      int    `json:"months"`
		AnnualCents int64  `json:"annual_cents"`
		DeltaCents  int64  `json:"delta_cents"`
	} `json:"lines"`
	Installments []struct {
		Month          string `json:"month"`
		DueOn          string `json:"due_on"`
		ImmediatelyDue bool   `json:"immediately_due"`
		AmountCents    int64  `json:"amount_cents"`
		DeltaCents     int64  `json:"delta_cents"`
	} `json:"installments"`
	PreviousCents int64 `json:"previous_cents"`
	TotalCents    int64 `json:"total_cents"`
	DeltaCents    int64 `json:"delta_cents"`
	Amends        *struct {
		Number string `json:"number"`
	} `json:"amends"`
	AmendedBy *struct {
		Number string `json:"number"`
	} `json:"amended_by"`
	LegalRemedy string `json:"legal_remedy"`
}

type careNoticeRunBody struct {
	ID                 int64            `json:"id"`
	Status             string           `json:"status"`
	Notices            []careNoticeBody `json:"notices"`
	Unchanged          int              `json:"unchanged"`
	Billed             int              `json:"billed"`
	PreviewCount       int              `json:"preview_count"`
	PreviewCents       int64            `json:"preview_cents"`
	Fingerprint        string           `json:"fingerprint"`
	LegalRemedyMissing bool             `json:"legal_remedy_missing"`
}

func (f careBillingFixture) startNoticeRun(t *testing.T, fromMonth, noticeDate string) careNoticeRunBody {
	t.Helper()
	year, err := strconv.Atoi(fromMonth[:4])
	require.NoError(t, err)
	body := map[string]any{"year": year, "from_month": fromMonth}
	if noticeDate != "" {
		body["notice_date"] = noticeDate
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunsPath, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careNoticeRunBody](t, rec.Body.Bytes())
}

func (f careBillingFixture) noticeRun(t *testing.T, id int64) careNoticeRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careNoticeRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careNoticeRunBody](t, rec.Body.Bytes())
}

func (f careBillingFixture) releaseNoticeRun(t *testing.T, run careNoticeRunBody) careNoticeRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/release"), careNoticeRelease(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careNoticeRunBody](t, rec.Body.Bytes())
}

func careNoticeRunPath(id int64, suffix string) string {
	return careNoticeRunsPath + "/" + strconv.FormatInt(id, 10) + suffix
}

func careNoticeRelease(run careNoticeRunBody) map[string]any {
	return map[string]any{"document_count": run.PreviewCount, "total_cents": run.PreviewCents, "fingerprint": run.Fingerprint}
}

// writeLegalRemedy gives the Träger a template of the fee notice with its
// legal remedy (ADR 0170), as the office saves it.
func writeLegalRemedy(t *testing.T, db *bun.DB, organizationID int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `INSERT INTO platform.traeger_document_templates
		(organization_id, kind, version, subject, body, legal_remedy)
		VALUES (?, 'bescheid', 1, 'Gebührenbescheid', 'Wir setzen fest:', 'Gegen diesen Bescheid kann innerhalb eines Monats Widerspruch erhoben werden.')`,
		organizationID)
	require.NoError(t, err)
}

// newNoticeFixture is a Träger with the SJR prices and Mia's contract from
// January 2030 on: 32,00 € until 14:00 on Monday and 45,50 € until 16:30 on
// Tuesday, 77,50 € a month.
func newNoticeFixture(t *testing.T, name string) (careBillingFixture, int64) {
	t.Helper()
	f := newCareBillingFixture(t, name)
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return f, contract
}

// The notice run assesses September to December in one notice with four
// installments; it needs the legal remedy, takes its number from the range of
// the monthly care, never changes, and no monthly run bills its months.
func TestCareNoticeRunIssuesTheYearAndNoMonthlyRunBillsItAgain(t *testing.T) {
	t.Parallel()
	f, contract := newNoticeFixture(t, "Träger Jahresbescheid")

	run := f.startNoticeRun(t, "2030-09-01", "2030-09-24")
	preview := f.noticeRun(t, run.ID)
	require.Len(t, preview.Notices, 1)
	notice := preview.Notices[0]
	assert.Equal(t, contract, notice.ContractID)
	assert.Equal(t, "initial", notice.Kind)
	assert.Empty(t, notice.Number, "a draft has no number")
	assert.Equal(t, int64(4*7750), notice.TotalCents)
	require.Len(t, notice.Installments, 4)
	assert.True(t, notice.Installments[0].ImmediatelyDue, "September is past on the notice date")
	assert.Equal(t, "2030-10-01", notice.Installments[1].DueOn)
	assert.True(t, preview.LegalRemedyMissing)

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/release"), careNoticeRelease(preview))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.legal_remedy_missing")

	writeLegalRemedy(t, f.db, f.office.organization)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/release"), careNoticeRelease(preview))
	require.Equal(t, http.StatusConflict, rec.Code, "the legal remedy is part of the reviewed state: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_changed")

	released := f.releaseNoticeRun(t, f.noticeRun(t, run.ID))
	assert.Equal(t, "released", released.Status)
	require.Len(t, released.Notices, 1)
	issued := released.Notices[0]
	assert.Regexp(t, `^MB-2030-0001$`, issued.Number)
	assert.Contains(t, issued.LegalRemedy, "Widerspruch", "the notice keeps its copy of the legal remedy")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		careNoticeRunPath(run.ID, "/notices/"+strconv.FormatInt(contract, 10)+"/letter"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), issued.Number)

	// No monthly run bills an assessed month; a month before the notice
	// stays with the monthly run.
	october := f.run(t, f.startRun(t, "2030-10-01").ID)
	assert.Empty(t, october.Invoices, "October is assessed by the notice")
	assert.Equal(t, 1, october.Billed)
	august := f.run(t, f.startRun(t, "2030-08-01").ID)
	require.Len(t, august.Invoices, 1, "August is no month of the notice")

	// The notice, its months and its run never change, for any role.
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `UPDATE parent_billing.care_notices SET total_cents = 1 WHERE id = ?`, issued.ID)
	require.Error(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE parent_billing.care_notice_months SET amount_cents = 1 WHERE notice_id = ?`, issued.ID)
	require.Error(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE parent_billing.care_notice_runs SET total_cents = 1 WHERE id = ?`, run.ID)
	require.Error(t, err)
	// The database keeps a contract to one first notice per year.
	_, err = f.db.ExecContext(ctx, `INSERT INTO parent_billing.care_notices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_year, kind, care_contract_id, school_id, school_name, child_first_name,
			child_last_name, recipient_email, family_email, recipient, payment_method, period_from, period_until, lines,
			previous_cents, total_cents, delta_cents, legal_remedy)
		SELECT organization_id, run_id, number || '-X', number_year, number_sequence + 100, issued_on, billing_year, kind,
			care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_email, family_email,
			recipient, payment_method, period_from, period_until, lines, previous_cents, total_cents, delta_cents, legal_remedy
		FROM parent_billing.care_notices WHERE id = ?`, issued.ID)
	require.ErrorContains(t, err, "uq_care_notices_first")

	// A run of the year with nothing changed issues nothing.
	again := f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "2030-10-01").ID)
	assert.Empty(t, again.Notices)
	assert.Equal(t, 1, again.Unchanged)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(again.ID, "/release"), careNoticeRelease(again))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_notice_run_events
		WHERE run_id = ? AND action IN ('read', 'letter') AND details->'contract_ids' @> to_jsonb(?::bigint)`, run.ID, contract).
		Scan(ctx, &reads))
	assert.Equal(t, 3, reads, "every read and the letter name the contract in the trail")
}

// A month a monthly invoice already bills is not assessed again.
func TestCareNoticeLeavesAMonthTheMonthlyRunBilled(t *testing.T) {
	t.Parallel()
	f, _ := newNoticeFixture(t, "Träger Jahresbescheid nach Monat")
	writeLegalRemedy(t, f.db, f.office.organization)
	september := f.run(t, f.startRun(t, "2030-09-01").ID)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(september.ID, "/release"), careRelease(september))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	preview := f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "2030-09-24").ID)
	require.Len(t, preview.Notices, 1)
	assert.Len(t, preview.Notices[0].Installments, 3, "September has its monthly invoice")
	assert.Equal(t, "2030-10-01", preview.Notices[0].Installments[0].Month)
	assert.Equal(t, 1, preview.Billed)
}

// A changed contract gets a change notice that names the notice it amends;
// from then on the change is the notice of its months, and a notice is
// amended once.
func TestCareNoticeChangeAmendsTheIssuedNotice(t *testing.T) {
	t.Parallel()
	f, contract := newNoticeFixture(t, "Träger Änderungsbescheid")
	writeLegalRemedy(t, f.db, f.office.organization)
	first := f.releaseNoticeRun(t, f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "2030-09-24").ID)).Notices[0]

	// From November Mia comes only on Mondays until 14:00.
	ctx := context.Background()
	var versionID int64
	_, err := f.db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = 'abgeloest'
		WHERE contract_id = ? AND version_number = 1`, contract)
	require.NoError(t, err)
	require.NoError(t, f.db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
			status, effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
			special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner)
		SELECT organization_id, contract_id, 2, 'wirksam', '2030-11-01'::date, child_first_name, child_last_name, school_name,
			term_kind, notice_months, notice_to, special_termination, special_termination_note, statute_required,
			statute_title, statute_url, contract_partner
		FROM care_contract.contract_versions WHERE contract_id = ? AND version_number = 1 RETURNING id`, contract).Scan(ctx, &versionID))
	_, err = f.db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id,
		weekday, care_offering_id, module_name, ends_at) VALUES (?, ?, 'mon', ?, 'Betreuung bis 14:00 Uhr', '14:00'::time)`,
		f.office.organization, versionID, careModuleUntil1400)
	require.NoError(t, err)

	preview := f.noticeRun(t, f.startNoticeRun(t, "2030-11-01", "2030-10-20").ID)
	require.Len(t, preview.Notices, 1)
	change := preview.Notices[0]
	assert.Equal(t, "change", change.Kind)
	require.NotNil(t, change.Amends)
	assert.Equal(t, first.Number, change.Amends.Number)
	assert.Equal(t, first.TotalCents, change.PreviousCents)
	assert.Equal(t, int64(2*3200-2*7750), change.DeltaCents, "November and December lose the Tuesday")
	require.Len(t, change.Installments, 4)
	assert.Equal(t, int64(3200), change.Installments[2].AmountCents)
	assert.Equal(t, int64(-4550), change.Installments[2].DeltaCents)

	released := f.releaseNoticeRun(t, preview)
	require.Len(t, released.Notices, 1)
	assert.Equal(t, "MB-2030-0002", released.Notices[0].Number)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careNoticeRunPath(released.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code)

	// The first notice stays as issued and names its change.
	var firstTotal int64
	require.NoError(t, f.db.NewRaw(`SELECT total_cents FROM parent_billing.care_notices WHERE id = ?`, first.ID).Scan(ctx, &firstTotal))
	assert.Equal(t, first.TotalCents, firstTotal)
	november := f.run(t, f.startRun(t, "2030-11-01").ID)
	assert.Empty(t, november.Invoices, "November is assessed by the change notice")

	// A second change of the same notice is refused by the database.
	_, err = f.db.ExecContext(ctx, `INSERT INTO parent_billing.care_notices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_year, kind, amends_notice_id, care_contract_id, school_id, school_name,
			child_first_name, child_last_name, recipient_email, family_email, recipient, payment_method, period_from,
			period_until, lines, previous_cents, total_cents, delta_cents, legal_remedy)
		SELECT organization_id, run_id, number || '-X', number_year, number_sequence + 100, issued_on, billing_year, 'change',
			?, care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_email, family_email,
			recipient, payment_method, period_from, period_until, lines, previous_cents, total_cents, delta_cents, legal_remedy
		FROM parent_billing.care_notices WHERE id = ?`, first.ID, first.ID)
	require.ErrorContains(t, err, "uq_care_notices_amends")
}

// A notice issued between the review of a monthly run and its release
// changes what the monthly run would bill: the fingerprint of the review
// covers it and the release refuses.
func TestCareMonthlyReleaseRefusesAfterANoticeTookTheMonth(t *testing.T) {
	t.Parallel()
	f, _ := newNoticeFixture(t, "Träger Fingerabdruck Bescheid")
	writeLegalRemedy(t, f.db, f.office.organization)
	other := f.office.schools[1]
	careContract(t, f.db, f.office.organization, other, "Noah", careMonth, sjrServices()[0])
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(other, "2029-09-01", 2800))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	october := f.run(t, f.startRun(t, "2030-10-01").ID)
	require.Len(t, october.Invoices, 2)

	// The notice assesses Mia's school only; Noah stays with the monthly run.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunsPath,
		map[string]any{"year": 2030, "from_month": "2030-09-01", "notice_date": "2030-09-24", "school_ids": []int64{f.office.schools[0]}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	f.releaseNoticeRun(t, f.noticeRun(t, decodeSchoolPortalData[careNoticeRunBody](t, rec.Body.Bytes()).ID))

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(october.ID, "/release"), careRelease(october))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_changed")
	var invoices int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_invoices WHERE run_id = ?`, october.ID).
		Scan(context.Background(), &invoices))
	assert.Zero(t, invoices, "no month is billed twice")
	again := f.run(t, october.ID)
	require.Len(t, again.Invoices, 1, "the review now shows Noah alone")
	assert.Equal(t, 1, again.Billed)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(october.ID, "/release"), careRelease(again))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// Another Träger sees no run; a second open run of a year is refused; the
// list reads the same statements for any number of runs.
func TestCareNoticeRunsStayWithTheirTraegerAndTheListIsFlat(t *testing.T) {
	t.Parallel()
	f, _ := newNoticeFixture(t, "Träger Bescheid Grenze")
	run := f.startNoticeRun(t, "2030-09-01", "")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunsPath,
		map[string]any{"year": 2030, "from_month": "2030-10-01"})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_notice_run_open")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunsPath,
		map[string]any{"year": 2030, "from_month": "2031-01-01"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_notice_run_invalid")

	other := newCareBillingFixture(t, "Träger Bescheid fremd")
	rec = traegerJSON(t, other.traeger, other.office.token, http.MethodGet, careNoticeRunPath(run.ID, ""), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_notice_run_not_found")

	list := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, careNoticeRunsPath, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	one := list()
	f.startNoticeRun(t, "2031-01-01", "")
	assert.Len(t, list().Statements, len(one.Statements))
	querybudgets.AssertQueryBudget(t, "api.traeger.care_notice_runs", one)
}
