// Router-level tests for the billing run of a holiday period (#4668, ADR
// 0119): the preview before the release, corrections with a reason and their
// history, the number range per service, the release that issues numbered
// invoices without a gap even when releases run in parallel, invoices that
// never change, the right "Abrechnung", the Träger boundary and the trail.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const (
	billingRunsPath  = "/traeger/billing-runs"
	numberRangesPath = "/traeger/number-ranges"
)

type billingPositionBody struct {
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents"`
	Note        string `json:"note"`
}

type billingChildBody struct {
	ChildID     int64                 `json:"child_id"`
	FirstName   string                `json:"first_name"`
	Positions   []billingPositionBody `json:"positions"`
	Corrections []struct {
		AmountCents        int64  `json:"amount_cents"`
		TotalBeforeCents   int64  `json:"total_before_cents"`
		TotalAfterCents    int64  `json:"total_after_cents"`
		Reason             string `json:"reason"`
		CreatedByAccountID *int64 `json:"created_by_account_id"`
	} `json:"corrections"`
	TotalCents int64 `json:"total_cents"`
}

type billingDocumentBody struct {
	Number             string             `json:"number"`
	IssuedOn           *string            `json:"issued_on"`
	RegistrationID     int64              `json:"registration_id"`
	RecipientFirstName string             `json:"recipient_first_name"`
	RecipientEmail     string             `json:"recipient_email"`
	ServiceFrom        string             `json:"service_from"`
	ServiceUntil       string             `json:"service_until"`
	Children           []billingChildBody `json:"children"`
	TotalCents         int64              `json:"total_cents"`
}

type billingRunBody struct {
	ID              int64                 `json:"id"`
	HolidayPeriodID int64                 `json:"holiday_period_id"`
	PeriodName      string                `json:"period_name"`
	ServiceFrom     string                `json:"service_from"`
	Status          string                `json:"status"`
	DocumentCount   *int                  `json:"document_count"`
	TotalCents      *int64                `json:"total_cents"`
	Documents       []billingDocumentBody `json:"documents"`
	PreviewCount    int                   `json:"preview_count"`
	PreviewCents    int64                 `json:"preview_cents"`
	Unbilled        int                   `json:"unbilled"`
	NumberRangeSet  bool                  `json:"number_range_set"`
	BilledInAdvance bool                  `json:"billed_in_advance"`
}

type billingPeriodBody struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	StartsOn string `json:"starts_on"`
	EndsOn   string `json:"ends_on"`
}

func billingRunPath(id int64, suffix string) string {
	return fmt.Sprintf("%s/%d%s", billingRunsPath, id, suffix)
}

func startBillingRun(t *testing.T, f reviewFixture, token string, periodID int64) billingRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, billingRunsPath, map[string]any{"holiday_period_id": periodID})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[billingRunBody](t, rec.Body.Bytes())
}

func billingRun(t *testing.T, f reviewFixture, token string, id int64) billingRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, billingRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[billingRunBody](t, rec.Body.Bytes())
}

func releaseBody(run billingRunBody) map[string]any {
	return map[string]any{"document_count": run.PreviewCount, "total_cents": run.PreviewCents}
}

func setNumberRange(t *testing.T, f reviewFixture, token, pattern string) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodPut, numberRangesPath+"/holiday", map[string]any{"pattern": pattern})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// runActions lists the trail of a run, oldest first.
func runActions(t *testing.T, f reviewFixture, runID int64) []string {
	t.Helper()
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT action FROM parent_billing.run_events WHERE run_id = ? AND actor_account_id IS NOT NULL ORDER BY id`, runID).
		Scan(context.Background(), &actions))
	return actions
}

func TestHolidayBillingRunPreviewsCorrectsAndIssuesNumberedInvoices(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Lauf Ferien")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	frozen := f.contribution(t, f.siblings)

	rec := traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]billingRunBody](t, rec.Body.Bytes()))
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunsPath+"/periods", nil)
	require.Equal(t, http.StatusOK, rec.Code, "the Abrechnung lists the periods without the right Anmeldung verwalten: %s", rec.Body.String())
	periods := decodeSchoolPortalData[[]billingPeriodBody](t, rec.Body.Bytes())
	require.Len(t, periods, 1)
	assert.Equal(t, f.period.ID, periods[0].ID)
	assert.Equal(t, "2027-01-04", periods[0].StartsOn)

	run := startBillingRun(t, f, billing, f.period.ID)
	assert.Equal(t, "calculated", run.Status)
	assert.Equal(t, "Winterferien 2027", run.PeriodName)
	assert.Equal(t, "2027-01-04", run.ServiceFrom)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunsPath, map[string]any{"holiday_period_id": f.period.ID})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_run_open", code, "a period has one run in preview at a time")

	// The preview bills the confirmed family with the frozen contribution;
	// the registration still received is not billed.
	preview := billingRun(t, f, billing, run.ID)
	assert.True(t, preview.BilledInAdvance, "holidays are billed in advance, before the first week")
	assert.False(t, preview.NumberRangeSet)
	require.Len(t, preview.Documents, 1)
	family := preview.Documents[0]
	assert.Empty(t, family.Number, "a draft has no number")
	assert.Equal(t, f.siblings, family.RegistrationID)
	assert.Equal(t, "Eva", family.RecipientFirstName)
	assert.Equal(t, "eva.muster@example.org", family.RecipientEmail)
	assert.Equal(t, frozen.TotalCents, family.TotalCents, "the run bills what the confirmation froze")
	assert.Equal(t, frozen.TotalCents, preview.PreviewCents)
	require.Len(t, family.Children, 2)
	lia := family.Children[0]
	assert.Equal(t, "Lia", lia.FirstName)

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(preview))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.number_range_missing", code)

	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, numberRangesPath+"/holiday", map[string]any{"pattern": "FE-{JJJJ}"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.number_range_invalid", code)
	assert.Equal(t, []string{"pattern"}, fields)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, numberRangesPath+"/holiday", map[string]any{"pattern": "FE-{NNNN}"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "FE-0001", decodeSchoolPortalData[struct {
		Example string `json:"example"`
	}](t, rec.Body.Bytes()).Example)

	// A correction needs a reason and a child the run bills.
	correction := map[string]any{"registration_id": f.siblings, "child_id": lia.ChildID, "total_cents": 3000, "reason": " "}
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/corrections"), correction)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_correction_invalid", code)
	assert.Equal(t, []string{"reason"}, fields)
	correction["reason"] = "Erste Woche krank, halber Beitrag"
	correction["registration_id"] = f.again
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/corrections"), correction)
	require.Equal(t, http.StatusNotFound, rec.Code, "a registration the run does not bill: %s", rec.Body.String())
	correction["registration_id"] = f.siblings
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/corrections"), correction)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	corrected := decodeSchoolPortalData[billingRunBody](t, rec.Body.Bytes()).Documents[0].Children[0]
	assert.Equal(t, int64(3000), corrected.TotalCents)
	last := corrected.Positions[len(corrected.Positions)-1]
	assert.Equal(t, "correction", last.Kind)
	assert.Equal(t, 3000-lia.TotalCents, last.AmountCents, "a correction is a position of the difference")
	assert.Equal(t, "Erste Woche krank, halber Beitrag", last.Note)
	require.Len(t, corrected.Corrections, 1)
	assert.Equal(t, lia.TotalCents, corrected.Corrections[0].TotalBeforeCents)
	assert.NotNil(t, corrected.Corrections[0].CreatedByAccountID, "the history names who corrected")
	correction["total_cents"], correction["reason"] = 3500, "Doch nur ein Tag krank"
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/corrections"), correction)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	checked := decodeSchoolPortalData[billingRunBody](t, rec.Body.Bytes())
	assert.Len(t, checked.Documents[0].Children[0].Corrections, 2, "the history keeps every correction")
	assert.Equal(t, frozen.TotalCents-lia.TotalCents+3500, checked.PreviewCents)

	// A family confirmed after the check changes the run: the release
	// refuses and issues nothing.
	require.Equal(t, http.StatusOK, f.act(t, f.again, "confirm").Code)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(checked))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_run_changed", code)

	checked = billingRun(t, f, billing, run.ID)
	require.Len(t, checked.Documents, 2)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(checked))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[billingRunBody](t, rec.Body.Bytes())
	assert.Equal(t, "released", released.Status)
	require.NotNil(t, released.DocumentCount)
	assert.Equal(t, 2, *released.DocumentCount)
	assert.Equal(t, checked.PreviewCents, *released.TotalCents)
	require.Len(t, released.Documents, 2)
	assert.Equal(t, "FE-0001", released.Documents[0].Number, "numbers in the order of the registrations")
	assert.Equal(t, f.siblings, released.Documents[0].RegistrationID)
	assert.Equal(t, "FE-0002", released.Documents[1].Number)
	assert.NotNil(t, released.Documents[0].IssuedOn)
	issuedLia := released.Documents[0].Children[0]
	assert.Equal(t, int64(3500), issuedLia.TotalCents)
	assert.Len(t, issuedLia.Corrections, 2, "the issued invoice shows the history of its corrections")
	assert.Equal(t, "correction", issuedLia.Positions[len(issuedLia.Positions)-1].Kind)

	// A released run never changes.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/corrections"), correction)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_run_released", code)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(released))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, released.Documents, billingRun(t, f, billing, run.ID).Documents, "the issued invoices read back unchanged")

	// A later run of the period bills only what no invoice bills yet.
	next := startBillingRun(t, f, billing, f.period.ID)
	assert.Empty(t, billingRun(t, f, billing, next.ID).Documents, "every confirmed family has its invoice")
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(next.ID, "/release"), map[string]any{"document_count": 0, "total_cents": 0})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_run_empty", code)

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, decodeSchoolPortalData[[]billingRunBody](t, rec.Body.Bytes()), 2)

	// Every read of personal data and every change stands in the trail, the
	// read of the registrations also in Enrollment's.
	actions := runActions(t, f, run.ID)
	assert.Equal(t, "create", actions[0])
	for _, action := range []string{"read", "correct", "release"} {
		assert.Contains(t, actions, action)
	}
	assert.Contains(t, f.auditActions(t), "list", "the preview reads the registrations through Enrollment's review")
}

func TestHolidayBillingRunNeedsTheBillingRightAndStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Lauf Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Lauf Fremd", identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	run := startBillingRun(t, f, billing, f.period.ID)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	preview := billingRun(t, f, billing, run.ID)
	child := preview.Documents[0].Children[0]
	correction := map[string]any{"registration_id": f.siblings, "child_id": child.ChildID, "total_cents": 0, "reason": "Härtefall"}

	for _, request := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, billingRunsPath, nil},
		{http.MethodGet, billingRunsPath + "/periods", nil},
		{http.MethodPost, billingRunsPath, map[string]any{"holiday_period_id": f.period.ID}},
		{http.MethodGet, billingRunPath(run.ID, ""), nil},
		{http.MethodPost, billingRunPath(run.ID, "/corrections"), correction},
		{http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(preview)},
		{http.MethodGet, numberRangesPath, nil},
		{http.MethodPut, numberRangesPath + "/holiday", map[string]any{"pattern": "X-{N}"}},
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right Abrechnung: %s", request.method, request.path, rec.Body.String())
	}

	// Another Träger sees no run, no invoice and no number range of this
	// Träger and cannot start a run for its period.
	rec := traegerJSON(t, f.traeger, foreign.token, http.MethodGet, billingRunsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]billingRunBody](t, rec.Body.Bytes()))
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, billingRunsPath+"/periods", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]billingPeriodBody](t, rec.Body.Bytes()), "another Träger lists none of these periods")
	for _, request := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, billingRunsPath, map[string]any{"holiday_period_id": f.period.ID}},
		{http.MethodGet, billingRunPath(run.ID, ""), nil},
		{http.MethodPost, billingRunPath(run.ID, "/corrections"), correction},
		{http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(preview)},
	} {
		rec := traegerJSON(t, f.traeger, foreign.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, numberRangesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ranges := decodeSchoolPortalData[[]struct {
		Pattern string `json:"pattern"`
	}](t, rec.Body.Bytes())
	require.Len(t, ranges, 1)
	assert.Empty(t, ranges[0].Pattern, "the number range of another Träger stays its own")

	var corrections, documents int
	require.NoError(t, f.db.NewRaw(`SELECT
			(SELECT count(*) FROM parent_billing.run_corrections WHERE run_id = ?),
			(SELECT count(*) FROM parent_billing.documents WHERE run_id = ?)`, run.ID, run.ID).
		Scan(context.Background(), &corrections, &documents))
	assert.Zero(t, corrections, "refused corrections wrote nothing")
	assert.Zero(t, documents, "refused releases issued nothing")
	assert.Equal(t, "calculated", billingRun(t, f, billing, run.ID).Status)
}

// TestParallelReleasesNumberInvoicesWithoutAGap releases the runs of two
// periods of one Träger at the same time, each twice. Every run is
// released once, and the invoices of both share one number range without a
// gap and without a duplicate.
func TestParallelReleasesNumberInvoicesWithoutAGap(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Lauf Parallel")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	second := openHolidays(t, f.holidayRegistrationFixture, f.office)
	offer := holidayOffer(t, f.holidayRegistrationFixture, second.PublicToken)
	var registrations []int64
	for _, email := range []string{"a.muster@example.org", "b.muster@example.org", "c.muster@example.org"} {
		rec := publicJSON(t, f.public, http.MethodPost, registrationPath(second.PublicToken), siblingsRegistration(offer, email))
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		registrations = append(registrations, decodeSchoolPortalData[struct {
			ID int64 `json:"id"`
		}](t, rec.Body.Bytes()).ID)
	}
	for _, id := range []int64{f.siblings, f.again} {
		require.Equal(t, http.StatusOK, f.act(t, id, "confirm").Code)
	}
	for _, id := range registrations {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(second.ID, id), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	setNumberRange(t, f, billing, "FE-{NNNN}")
	runs := []billingRunBody{startBillingRun(t, f, billing, f.period.ID), startBillingRun(t, f, billing, second.ID)}
	for i := range runs {
		runs[i] = billingRun(t, f, billing, runs[i].ID)
	}

	bodies := make([][]byte, len(runs))
	for i, run := range runs {
		body, err := json.Marshal(releaseBody(run))
		require.NoError(t, err)
		bodies[i] = body
	}
	codes := make([]int, 2*len(runs))
	var wait sync.WaitGroup
	for i := range codes {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			run := runs[i%len(runs)]
			req := httptest.NewRequest(http.MethodPost, billingRunPath(run.ID, "/release"), bytes.NewReader(bodies[i%len(runs)]))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+billing)
			rec := httptest.NewRecorder()
			f.traeger.ServeHTTP(rec, req)
			codes[i] = rec.Code
		}(i)
	}
	wait.Wait()
	sort.Ints(codes)
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusConflict, http.StatusConflict}, codes, "every run is released once")

	var numbers []string
	require.NoError(t, f.db.NewRaw(`SELECT number FROM parent_billing.documents WHERE organization_id = ? ORDER BY number_sequence`, f.office.organization).
		Scan(context.Background(), &numbers))
	assert.Equal(t, []string{"FE-0001", "FE-0002", "FE-0003", "FE-0004", "FE-0005"}, numbers, "one number range for both runs, without gap or duplicate")
	var last int64
	require.NoError(t, f.db.NewRaw(`SELECT last_number FROM parent_billing.number_range_counters WHERE organization_id = ?`, f.office.organization).
		Scan(context.Background(), &last))
	assert.EqualValues(t, len(numbers), last, "the refused releases used no number")
}

func TestBillingRunListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Budget Läufe")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	list := func(path string) []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+billing)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	startBillingRun(t, f, billing, f.period.ID)
	runs, periods := list(billingRunsPath), list(billingRunsPath+"/periods")
	second := openHolidays(t, f.holidayRegistrationFixture, f.office)
	startBillingRun(t, f, billing, second.ID)
	assert.Len(t, list(billingRunsPath), len(runs), "the list reads the same statements for any number of runs")
	assert.Len(t, list(billingRunsPath+"/periods"), len(periods), "the periods read the same statements for any number of periods")
	querybudgets.AssertQueryBudget(t, "api.traeger.billing_runs", runs)
	querybudgets.AssertQueryBudget(t, "api.traeger.billing_run_periods", periods)
}
