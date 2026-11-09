// Router-level tests for the review of a monthly run before its release
// (#5916, ADR 0119 addendum 2026-11-08): schools per run with at most one
// open run per school and month, also when two start at once; the invoice
// date of the run on every invoice; the exclusion of an invoice with its
// reason in the trail and its return in the next run of the month; the
// release that refuses a preview changed since the review; the review list
// without numbers; the right "Abrechnung" and the Träger boundary.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type careRunSchoolBody struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	OpenRunID *int64 `json:"open_run_id"`
}

type careReviewInvoiceBody struct {
	ContractID int64 `json:"contract_id"`
	SchoolID   int64 `json:"school_id"`
	TotalCents int64 `json:"total_cents"`
	Exclusion  *struct {
		Reason    string `json:"reason"`
		AccountID *int64 `json:"account_id"`
	} `json:"exclusion"`
}

type careReviewBody struct {
	careRunBody
	Invoices []careReviewInvoiceBody `json:"invoices"`
	Schools  []struct {
		ID int64 `json:"id"`
	} `json:"schools"`
}

func (f careBillingFixture) startRunWith(t *testing.T, body map[string]any) (int, careRunBody) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunsPath, body)
	if rec.Code != http.StatusCreated {
		return rec.Code, careRunBody{}
	}
	return rec.Code, decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
}

func (f careBillingFixture) review(t *testing.T, id int64) careReviewBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careReviewBody](t, rec.Body.Bytes())
}

// pricedSchools gives both schools of the fixture the SJR prices.
func pricedSchools(t *testing.T, f careBillingFixture) {
	t.Helper()
	for _, school := range f.office.schools[:2] {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
}

// A run bills the schools it names. Runs of disjoint schools of a month may
// be checked side by side; a school is in at most one open run per month,
// and the next run of the month may bill it once the first is released.
func TestCareReviewRunBillsItsSchoolsAndKeepsASchoolInOneOpenRun(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Prüfphase Schulen")
	a, b := f.office.schools[0], f.office.schools[1]
	mia := careContract(t, f.db, f.office.organization, a, "Mia", careMonth, sjrServices()...)
	noah := careContract(t, f.db, f.office.organization, b, "Noah", careMonth, sjrServices()...)
	pricedSchools(t, f)

	code, runA := f.startRunWith(t, map[string]any{"billing_month": careMonth, "school_ids": []int64{a}})
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, []int64{a}, runA.SchoolIDs)
	review := f.review(t, runA.ID)
	require.Len(t, review.Invoices, 1, "the run of school A bills school A only")
	assert.Equal(t, mia, review.Invoices[0].ContractID)
	require.Len(t, review.Schools, 1)

	code, _ = f.startRunWith(t, map[string]any{"billing_month": careMonth, "school_ids": []int64{a, b}})
	assert.Equal(t, http.StatusConflict, code, "school A is in an open run of the month")
	code, runB := f.startRunWith(t, map[string]any{"billing_month": careMonth})
	require.Equal(t, http.StatusCreated, code, "without schools the run takes every school no open run bills")
	assert.Equal(t, []int64{b}, runB.SchoolIDs)
	assert.Equal(t, noah, f.review(t, runB.ID).Invoices[0].ContractID)
	code, _ = f.startRunWith(t, map[string]any{"billing_month": careMonth})
	assert.Equal(t, http.StatusConflict, code, "every school is in an open run of the month")

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunsPath+"/schools?month="+careMonth, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	schools := decodeSchoolPortalData[[]careRunSchoolBody](t, rec.Body.Bytes())
	open := map[int64]int64{}
	for _, school := range schools {
		if school.OpenRunID != nil {
			open[school.ID] = *school.OpenRunID
		}
	}
	assert.Equal(t, map[int64]int64{a: runA.ID, b: runB.ID}, open, "the dialog names the open run of each school")

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Prüfphase Schulen Fremd", identityaccess.TraegerRoleBilling)
	code, _ = f.startRunWith(t, map[string]any{"billing_month": "2030-02-01", "school_ids": []int64{foreign.schools[0]}})
	assert.Equal(t, http.StatusBadRequest, code, "a school of another Träger is refused")

	review = f.review(t, runA.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runA.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	code, next := f.startRunWith(t, map[string]any{"billing_month": careMonth, "school_ids": []int64{a}})
	require.Equal(t, http.StatusCreated, code, "after the release a later run of the month may bill school A")
	assert.Empty(t, f.review(t, next.ID).Invoices, "Mia is billed once")
	assert.Equal(t, 1, f.review(t, next.ID).Billed, "school A counts its own issued invoice")
	assert.Zero(t, f.review(t, runB.ID).Billed, "school B does not count school A's issued invoice")
}

// Two runs that start at once with overlapping schools: one starts, the
// other is refused; with disjoint schools both start (uq_care_run_schools_open).
func TestCareReviewRunsStartedAtOnceNeverShareASchool(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Prüfphase parallel")
	a, b := f.office.schools[0], f.office.schools[1]
	startAtOnce := func(month string, first, second []int64) []int {
		codes := make([]int, 2)
		var wg sync.WaitGroup
		for i, schools := range [][]int64{first, second} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes[i], _ = f.startRunWith(t, map[string]any{"billing_month": month, "school_ids": schools})
			}()
		}
		wg.Wait()
		return codes
	}
	assert.ElementsMatch(t, []int{http.StatusCreated, http.StatusConflict}, startAtOnce(careMonth, []int64{a, b}, []int64{b}))
	assert.ElementsMatch(t, []int{http.StatusCreated, http.StatusCreated}, startAtOnce("2030-02-01", []int64{a}, []int64{b}))
	var open []int64
	require.NoError(t, f.db.NewRaw(`SELECT school_id FROM parent_billing.care_run_schools
		WHERE organization_id = ? AND billing_month = ?::date AND open ORDER BY school_id`, f.office.organization, careMonth).
		Scan(context.Background(), &open))
	assert.Len(t, open, len(dedupe(open)), "no school is in two open runs of the month")
}

func dedupe(ids []int64) []int64 {
	seen := map[int64]bool{}
	result := []int64{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

// The invoice date of a run goes on every invoice of its release and on the
// run; it may lie in the billed month (SJR: October on 30.10.). Without one
// the release takes its own day. A date far from the month is refused.
func TestCareReviewInvoiceDateGoesOnEveryInvoice(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Prüfphase Rechnungsdatum")
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	careContract(t, f.db, f.office.organization, school, "Lea", careMonth, sjrServices()...)
	pricedSchools(t, f)

	code, _ := f.startRunWith(t, map[string]any{"billing_month": careMonth, "invoice_date": "2030-04-01"})
	assert.Equal(t, http.StatusBadRequest, code, "an invoice date two months after the month is refused")
	code, run := f.startRunWith(t, map[string]any{"billing_month": careMonth, "invoice_date": "2030-01-15"})
	require.Equal(t, http.StatusCreated, code)
	require.NotNil(t, run.InvoiceDate)
	assert.Equal(t, "2030-01-15", *run.InvoiceDate)

	path := careRunPath(run.ID, "/invoice-date")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"invoice_date": "2029-11-30"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "before the month before: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"invoice_date": "2030-01-30"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	review := decodeSchoolPortalData[careReviewBody](t, rec.Body.Bytes())
	assert.Equal(t, "2030-01-30", *review.InvoiceDate)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
	assert.Equal(t, "2030-01-30", *released.IssuedOn)
	var dates []string
	require.NoError(t, f.db.NewRaw(`SELECT DISTINCT issued_on::text FROM parent_billing.care_invoices WHERE run_id = ?`, run.ID).
		Scan(context.Background(), &dates))
	assert.Equal(t, []string{"2030-01-30"}, dates, "every invoice of the run carries the invoice date")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"invoice_date": nil})
	assert.Equal(t, http.StatusConflict, rec.Code, "a released run keeps its date: %s", rec.Body.String())
}

// An excluded invoice is not issued; the trail keeps the reason, the
// account and the time; the next run of the month bills the child again.
func TestCareReviewExcludedInvoiceComesBackInTheNextRun(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Prüfphase Ausschluss")
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	lea := careContract(t, f.db, f.office.organization, school, "Lea", careMonth, sjrServices()...)
	pricedSchools(t, f)
	run := f.startRun(t, careMonth)
	exclusions := careRunPath(run.ID, "/exclusions")

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, exclusions, map[string]any{"contract_id": mia, "service": "care", "reason": " "})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "an exclusion needs a reason: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, exclusions, map[string]any{"contract_id": mia, "reason": "x"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "an exclusion names the service of its invoice (#6346): %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, exclusions, map[string]any{"contract_id": mia + 1000, "service": "care", "reason": "x"})
	assert.Equal(t, http.StatusConflict, rec.Code, "only an invoice of the preview: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, exclusions,
		map[string]any{"contract_id": mia, "service": "care", "reason": "Kündigung wird geprüft"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	review := decodeSchoolPortalData[careReviewBody](t, rec.Body.Bytes())
	assert.Equal(t, 1, review.PreviewCount)
	assert.Equal(t, 1, review.ExcludedCount)
	assert.Equal(t, int64(7750), review.ExcludedCents)
	require.Len(t, review.Invoices, 2, "the excluded invoice stays in sight")
	excluded := review.Invoices[1]
	require.NotNil(t, excluded.Exclusion)
	assert.Equal(t, mia, excluded.ContractID)
	assert.Equal(t, "Kündigung wird geprüft", excluded.Exclusion.Reason)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, exclusions, map[string]any{"contract_id": mia, "service": "care", "reason": "noch mal"})
	assert.Equal(t, http.StatusConflict, rec.Code, "an excluded invoice is excluded once: %s", rec.Body.String())

	// The return appends to the trail; the second exclusion is in force.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/inclusions"),
		map[string]any{"contract_id": mia, "service": "care"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Zero(t, decodeSchoolPortalData[careReviewBody](t, rec.Body.Bytes()).ExcludedCount)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/inclusions"),
		map[string]any{"contract_id": mia, "service": "care"})
	assert.Equal(t, http.StatusConflict, rec.Code, "only an excluded invoice returns: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, exclusions,
		map[string]any{"contract_id": mia, "service": "care", "reason": "Vertrag endet"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var trail []struct {
		Action  string         `bun:"action"`
		Actor   *int64         `bun:"actor_account_id"`
		Details map[string]any `bun:"details"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT action, actor_account_id, details FROM parent_billing.care_run_events
		WHERE run_id = ? AND action IN ('exclude', 'include') ORDER BY occurred_at, id`, run.ID).Scan(context.Background(), &trail))
	require.Len(t, trail, 3, "every exclusion and return is appended")
	assert.Equal(t, []string{"exclude", "include", "exclude"}, []string{trail[0].Action, trail[1].Action, trail[2].Action})
	assert.Equal(t, "Vertrag endet", trail[2].Details["reason"])
	require.NotNil(t, trail[2].Actor)
	review = f.review(t, run.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
	require.Len(t, released.Invoices, 1)
	assert.Equal(t, lea, released.Invoices[0].ContractID, "the excluded invoice is not issued")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, exclusions, map[string]any{"contract_id": lea, "service": "care", "reason": "zu spät"})
	assert.Equal(t, http.StatusConflict, rec.Code, "a released run takes no exclusion: %s", rec.Body.String())

	next := f.review(t, f.startRun(t, careMonth).ID)
	require.Len(t, next.Invoices, 1, "the next run of the month bills the excluded child")
	assert.Equal(t, mia, next.Invoices[0].ContractID)
	assert.Nil(t, next.Invoices[0].Exclusion, "the exclusion belongs to its run")
}

// The release checks the preview again in its transaction. A change since
// the review that keeps count and sum (here the recipient) is refused by
// the fingerprint; so is an exclusion made after the review.
func TestCareReviewReleaseRefusesAPreviewChangedSinceTheReview(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Prüfphase Stand")
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	careContract(t, f.db, f.office.organization, school, "Lea", careMonth, sjrServices()...)
	pricedSchools(t, f)
	run := f.startRun(t, careMonth)
	reviewed := f.review(t, run.ID)
	require.NotEmpty(t, reviewed.Fingerprint)

	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = 'neu@example.org' WHERE contract_id = ?`, mia)
	require.NoError(t, err)
	assert.Equal(t, reviewed.PreviewCents, f.review(t, run.ID).PreviewCents, "count and sum stay the same")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(reviewed.careRunBody))
	assert.Equal(t, http.StatusConflict, rec.Code, "a new recipient since the review: %s", rec.Body.String())
	body := careRelease(reviewed.careRunBody)
	body["fingerprint"] = ""
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), body)
	assert.Equal(t, http.StatusConflict, rec.Code, "a release without the reviewed state: %s", rec.Body.String())

	reviewed = f.review(t, run.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/exclusions"),
		map[string]any{"contract_id": mia, "service": "care", "reason": "Rückfrage"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(reviewed.careRunBody))
	assert.Equal(t, http.StatusConflict, rec.Code, "an exclusion since the review: %s", rec.Body.String())
	var invoices int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_invoices WHERE run_id = ?`, run.ID).
		Scan(context.Background(), &invoices))
	assert.Zero(t, invoices, "a refused release issues nothing")
}

// The review list shows every position of the preview with the invoice
// date, marks the excluded invoice and keeps it out of the sum, and gives
// no number; its download is in the trail with the contracts it names.
func TestCareReviewListExportsThePreviewWithoutNumbers(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Prüfphase Prüfliste")
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	careContract(t, f.db, f.office.organization, school, "Lea", careMonth, sjrServices()...)
	pricedSchools(t, f)
	_, run := f.startRunWith(t, map[string]any{"billing_month": careMonth, "invoice_date": "2030-01-30"})
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/exclusions"),
		map[string]any{"contract_id": mia, "service": "care", "reason": "Kündigung wird geprüft"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, "/review-export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "pruefliste-monatsbeitrag-2030-01")
	csv := rec.Body.String()
	assert.Contains(t, csv, "Rechnungsdatum;Schule;Kind;Empfänger;E-Mail;Zahlungsweg;Fällig am;Position;Wochentage;Zeitraum;Betrag in Euro;Status")
	assert.Contains(t, csv, ";Überweisung;13.02.2030;", "the payment term counts from the invoice date of the run (#4689)")
	assert.Equal(t, 4, strings.Count(csv, "30.01.2030;"), "one row per position, each with the invoice date")
	assert.Contains(t, csv, "Ausgeschlossen: Kündigung wird geprüft")
	assert.Contains(t, csv, "Summe, die ausgestellt wird;;;77,50;", "the excluded invoice stays out of the sum")
	assert.NotContains(t, csv, "MB-", "the review list gives no number")
	var numbers int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_invoices WHERE run_id = ?`, run.ID).
		Scan(context.Background(), &numbers))
	assert.Zero(t, numbers)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, "/review-export?format=xlsx"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "spreadsheetml")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, "/review-export?format=pdf"), nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	var details []json.RawMessage
	require.NoError(t, f.db.NewRaw(`SELECT details FROM parent_billing.care_run_events WHERE run_id = ? AND action = 'export'`, run.ID).
		Scan(context.Background(), &details))
	require.Len(t, details, 2, "every download is in the trail")
	assert.Contains(t, string(details[0]), fmt.Sprintf("%d", mia), "the trail names the contracts the list shows")
}

// The review needs the right "Abrechnung" and stays with its Träger.
func TestCareReviewNeedsTheRightAndStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Prüfphase Grenze")
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	pricedSchools(t, f)
	run := f.startRun(t, careMonth)

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	requests := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, careRunsPath + "/schools?month=" + careMonth, nil},
		{http.MethodPut, careRunPath(run.ID, "/invoice-date"), map[string]any{"invoice_date": "2030-01-30"}},
		{http.MethodPost, careRunPath(run.ID, "/exclusions"), map[string]any{"contract_id": mia, "service": "care", "reason": "x"}},
		{http.MethodPost, careRunPath(run.ID, "/inclusions"), map[string]any{"contract_id": mia, "service": "care"}},
		{http.MethodGet, careRunPath(run.ID, "/review-export?format=csv"), nil},
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
	}
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Prüfphase Grenze Fremd", identityaccess.TraegerRoleBilling)
	for _, request := range requests[1:] {
		rec := traegerJSON(t, f.traeger, foreign.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	rec := traegerJSON(t, f.traeger, foreign.token, http.MethodGet, careRunsPath+"/schools?month="+careMonth, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, school := range decodeSchoolPortalData[[]careRunSchoolBody](t, rec.Body.Bytes()) {
		assert.NotEqual(t, f.office.schools[0], school.ID, "no school of another Träger")
		assert.Nil(t, school.OpenRunID, "no open run of another Träger")
	}
}

// Closing an all-excluded run keeps its trail, reserves no numbers and
// releases its schools so a later run can bill those contracts again.
func TestCareReviewAllExcludedRunClosesWithoutIssuing(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Prüfphase ohne Ausstellung")
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	pricedSchools(t, f)
	code, run := f.startRunWith(t, map[string]any{"billing_month": careMonth, "school_ids": []int64{school}})
	require.Equal(t, http.StatusCreated, code)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/exclusions"),
		map[string]any{"contract_id": contract, "service": "care", "reason": "Vertrag wird geprüft"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	review := decodeSchoolPortalData[careReviewBody](t, rec.Body.Bytes())
	require.Zero(t, review.PreviewCount)
	require.Equal(t, 1, review.ExcludedCount)
	stale := careRelease(review.careRunBody)
	stale["fingerprint"] = ""
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), stale)
	assert.Equal(t, http.StatusConflict, rec.Code, "even an empty release must match the reviewed state")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
	assert.Equal(t, "released", released.Status)
	assert.Empty(t, released.Invoices)
	var exclusions int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_run_events
		WHERE run_id = ? AND action = 'exclude' AND details->>'reason' = 'Vertrag wird geprüft'`, run.ID).
		Scan(context.Background(), &exclusions))
	assert.Equal(t, 1, exclusions, "the reason stays in the trail")
	code, next := f.startRunWith(t, map[string]any{"billing_month": careMonth, "school_ids": []int64{school}})
	require.Equal(t, http.StatusCreated, code, "closing the run releases the school")
	nextReview := f.review(t, next.ID)
	require.Len(t, nextReview.Invoices, 1)
	assert.Equal(t, contract, nextReview.Invoices[0].ContractID)
	assert.Nil(t, nextReview.Invoices[0].Exclusion)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(next.ID, "/release"), careRelease(nextReview.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	issued := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes())
	require.Len(t, issued.Invoices, 1)
	assert.Regexp(t, `^MB-\d{4}-0001$`, issued.Invoices[0].Number, "the empty release consumed no number")
}

// changedSinceReview reviews a run with Lea (issued) and Mia (excluded),
// lets change alter the month, and asserts that the release refuses the
// review: everything the review shows is part of its fingerprint, not only
// the invoices it issues (#5916).
func changedSinceReview(t *testing.T, name string, change func(f careBillingFixture, school, mia int64)) {
	t.Helper()
	f := newCareBillingFixture(t, name)
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Lea", careMonth, sjrServices()...)
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	pricedSchools(t, f)
	f.saveRule(t, careRule("income_level", "Stufe 2", []int64{school}, map[string]any{"discount_basis_points": 2000}))
	run := f.startRun(t, careMonth)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/exclusions"),
		map[string]any{"contract_id": mia, "service": "care", "reason": "Rückfrage"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reviewed := f.review(t, run.ID)
	require.Equal(t, 1, reviewed.PreviewCount)

	change(f, school, mia)
	after := f.review(t, run.ID)
	assert.Equal(t, reviewed.PreviewCount, after.PreviewCount, "the invoices to issue stay the same")
	assert.Equal(t, reviewed.PreviewCents, after.PreviewCents, "their sum too")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(reviewed.careRunBody))
	assert.Equal(t, http.StatusConflict, rec.Code, "the release refuses a review that changed: %s", rec.Body.String())
}

// A new contract without a price between review and release adds a booking
// without a price to the review.
func TestCareReviewReleaseRefusesANewBookingWithoutPrice(t *testing.T) {
	t.Parallel()
	changedSinceReview(t, "Träger Prüfphase neuer Vertrag ohne Preis", func(f careBillingFixture, school, _ int64) {
		careContract(t, f.db, f.office.organization, school, "Noah", careMonth,
			careService{"wed", 9199, "Spätbetreuung bis 17:00 Uhr", "17:00"})
		require.Len(t, f.review(t, f.openRunID(t)).Unpriced, 1)
	})
}

// Two new contracts of one child for the same module and day add a child
// the release holds back.
func TestCareReviewReleaseRefusesANewDuplicate(t *testing.T) {
	t.Parallel()
	changedSinceReview(t, "Träger Prüfphase neues Duplikat", func(f careBillingFixture, school, _ int64) {
		careContract(t, f.db, f.office.organization, school, "Noah", careMonth, sjrServices()...)
		careContract(t, f.db, f.office.organization, school, "Noah", careMonth, sjrServices()[0])
		require.NotEmpty(t, f.review(t, f.openRunID(t)).Duplicates)
	})
}

// A new evidence changes the reduction of the excluded invoice; the review
// shows it, so the release must see it.
func TestCareReviewReleaseRefusesAChangedReductionOfAnExcludedInvoice(t *testing.T) {
	t.Parallel()
	changedSinceReview(t, "Träger Prüfphase neuer Nachweis", func(f careBillingFixture, _, mia int64) {
		var rule int64
		require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.contribution_rules
			WHERE organization_id = ? AND name = 'Stufe 2'`, f.office.organization).Scan(context.Background(), &rule))
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, careEvidence(mia, "income_level", &rule))
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	})
}

// openRunID names the open run of the fixture's month.
func (f careBillingFixture) openRunID(t *testing.T) int64 {
	t.Helper()
	var id int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_runs
		WHERE organization_id = ? AND status = 'calculated' ORDER BY id DESC LIMIT 1`, f.office.organization).
		Scan(context.Background(), &id))
	return id
}
