package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

func TestHolidayPlacesRebookingKeepsTheFrozenContribution(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Plätze Beitrag Umbuchung")
	registration := f.submitPlaces(t, "umbuchung.beitrag@example.org", map[string][]int{"Tom": {0}})
	require.Equal(t, http.StatusOK, f.confirmWith(t, registration.ID, nil).Code)
	child := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(registration.ID, ""), nil)).Children[0].ID
	second := f.offer.Weeks[1]
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, registration.ID, child), map[string]any{
		"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Zero(t, f.mails(t, "holiday_place_granted", registration.ID), "a direct rebooking is not a promotion")
	assert.Equal(t, f.offer.Weeks[0].TimeModels[0].PriceCents, f.contributionTotal(t, registration.ID), "a direct rebooking keeps the frozen contribution instead of billing both weeks")
}

func TestHolidayPlacesPromotionUpdatesAnInvoicedContribution(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Plätze Beitrag Rechnung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	first, second := f.offer.Weeks[0], f.offer.Weeks[1]
	f.setCapacity(t, first.ID, 2)
	registration := f.submitPlaces(t, "nachruecken.rechnung@example.org", map[string][]int{"Tom": {0, 1}})
	require.Equal(t, http.StatusOK, f.confirmWith(t, registration.ID, nil).Code)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	before := correctedRun(t, f, billing, run.ID).Documents
	require.Equal(t, []int64{registration.ID}, f.setCapacity(t, first.ID, 3).MovedUp)
	contribution := f.cancelledContribution(t, registration.ID)
	assert.Equal(t, first.TimeModels[0].PriceCents+second.TimeModels[0].PriceCents, contribution.TotalCents)
	assert.True(t, contribution.CorrectionNeeded, "the new contribution must expose the difference to the issued invoice")
	assert.Equal(t, before, correctedRun(t, f, billing, run.ID).Documents, "issued invoice positions stay immutable")
	var invoice int64
	for _, document := range before {
		if document.Kind == "invoice" && document.RegistrationID == registration.ID {
			invoice = document.ID
		}
	}
	require.NotZero(t, invoice)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, invoice), map[string]any{"reason": "Weitere Ferienwoche zugeteilt"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	next := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	assert.Equal(t, contribution.TotalCents, next.PreviewCents, "the replacement invoice includes the newly placed week")
}

func TestHolidayPlacesNotifyEachPromotionOfARebookedWeek(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Plätze Erneut Nachrücken")
	first, second := f.offer.Weeks[0], f.offer.Weeks[1]
	f.setCapacity(t, first.ID, 2)
	registration := f.submitPlaces(t, "erneut.nachruecken@example.org", map[string][]int{"Tom": {0}})
	f.setCapacity(t, first.ID, 3)
	require.Equal(t, 1, f.mails(t, "holiday_place_granted", registration.ID))
	child := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(registration.ID, ""), nil)).Children[0].ID
	rebook := func(week, model int64) {
		t.Helper()
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, registration.ID, child), map[string]any{
			"bookings": []map[string]any{{"week_id": week, "time_model_id": model}},
		})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	rebook(second.ID, second.TimeModels[0].ID)
	var promotionRecorded bool
	require.NoError(t, f.db.NewRaw(`SELECT details->'before'->0->>'promoted_at' IS NOT NULL
  FROM enrollment.holiday_registration_audit WHERE holiday_registration_id=? AND action='rebook' ORDER BY id DESC LIMIT 1`, registration.ID).Scan(t.Context(), &promotionRecorded))
	assert.True(t, promotionRecorded, "rebooking preserves the previous promotion timestamp for historical care")
	f.setCapacity(t, first.ID, 2)
	rebook(first.ID, first.TimeModels[0].ID)
	before := f.mails(t, "holiday_place_granted", registration.ID)
	f.setCapacity(t, first.ID, 3)
	assert.Equal(t, before+1, f.mails(t, "holiday_place_granted", registration.ID), "each new booking generation gets its own promotion notice")
	f.setCapacity(t, first.ID, 3)
	assert.Equal(t, before+1, f.mails(t, "holiday_place_granted", registration.ID), "repeating the capacity does not duplicate the notice")
}

func TestHolidayPlacesRebookingStillPromotesAWaitingSibling(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Plätze Umbuchung Geschwister")
	first, second := f.offer.Weeks[0], f.offer.Weeks[1]
	f.setCapacity(t, first.ID, 3)
	registration := f.submitPlaces(t, "umbuchung.geschwister@example.org", map[string][]int{"Tom": {0}, "Ida": {0}})
	require.Equal(t, http.StatusOK, f.confirmWith(t, registration.ID, nil).Code)
	detail := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(registration.ID, ""), nil))
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, registration.ID, detail.Children[0].ID), map[string]any{
		"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, f.places(t)[first.ID].Waitlist, "the sibling receives the freed place")
	assert.Equal(t, 1, f.mails(t, "holiday_place_granted", registration.ID))
	assert.Equal(t, 2*first.TimeModels[0].PriceCents, f.contributionTotal(t, registration.ID), "only the waiting sibling adds a week; the rebooked child keeps its frozen price")
}

func TestHolidayPlacesDelayedReplacementKeepsTheFrozenCharge(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Plätze Ersatz Warteliste")
	first, second := f.offer.Weeks[0], f.offer.Weeks[1]
	registration := f.submitPlaces(t, "ersatz.warteliste@example.org", map[string][]int{"Tom": {0}})
	require.Equal(t, http.StatusOK, f.confirmWith(t, registration.ID, nil).Code)
	original := f.contributionTotal(t, registration.ID)
	full := f.places(t)[second.ID].Placed
	f.setCapacity(t, second.ID, full)
	child := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(registration.ID, ""), nil)).Children[0].ID
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, registration.ID, child), map[string]any{
		"bookings": []map[string]any{{"week_id": second.ID, "time_model_id": second.TimeModels[0].ID}},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, original, f.contributionTotal(t, registration.ID), "a replacement on the waiting list keeps the frozen charge")
	require.Equal(t, []int64{registration.ID}, f.setCapacity(t, second.ID, full+1).MovedUp)
	assert.Equal(t, original, f.contributionTotal(t, registration.ID), "promotion must not charge both the original and its replacement")
	assert.Equal(t, 1, f.mails(t, "holiday_place_granted", registration.ID))
	// A later rebooking back through the same lineage also replaces a paid week.
	f.setCapacity(t, first.ID, 2)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayRebookPath(f.period.ID, registration.ID, child), map[string]any{
		"bookings": []map[string]any{{"week_id": first.ID, "time_model_id": first.TimeModels[0].ID}},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	f.setCapacity(t, first.ID, 3)
	assert.Equal(t, original, f.contributionTotal(t, registration.ID))
}

func TestHolidayPlacesFullyWaitingFamilyCanKeepItsExemption(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Plätze Warteliste Befreiung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, benefitsExemption())
	first := f.offer.Weeks[0]
	f.setCapacity(t, first.ID, 2)
	body := placesRegistration(f, "warteliste.befreit@example.org", map[string][]int{"Tom": {0}})
	body["benefits_declared"] = true
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	registration := decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes()).ID
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registration), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	assert.True(t, preview.ExemptionAvailable)
	assert.True(t, preview.BenefitsExemption)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, registration), map[string]any{"benefits_exemption": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Zero(t, f.contributionTotal(t, registration))
	f.setCapacity(t, first.ID, 3)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registration), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	promoted := decodeSchoolPortalData[contributionBody](t, rec.Body.Bytes())
	assert.True(t, promoted.BenefitsExemption)
	assert.Zero(t, promoted.TotalCents, "promotion keeps the exemption agreed at confirmation")
}

func TestHolidayPlacesQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Plätze Query-Budget")
	f.setCapacity(t, f.offer.Weeks[0].ID, 2)
	f.submitPlaces(t, "budget.erstes@example.org", map[string][]int{"Tom": {0}})
	measure := func() testpkg.QueryMeasurement {
		t.Helper()
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, holidayPlacesPath(f.period.ID), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	small := measure()
	// Grow both the weeks and the waiting list within this test's own period.
	for n := 0; n < 3; n++ {
		_, err := f.db.ExecContext(t.Context(), `INSERT INTO enrollment.holiday_period_weeks
   (organization_id,holiday_period_id,starts_on,ends_on,host_school_id)
   SELECT organization_id,holiday_period_id,MAX(starts_on)+7,MAX(ends_on)+7,MAX(host_school_id)
   FROM enrollment.holiday_period_weeks WHERE organization_id=? AND holiday_period_id=? GROUP BY organization_id,holiday_period_id`, f.office.organization, f.period.ID)
		require.NoError(t, err)
		f.submitPlaces(t, fmt.Sprintf("budget.weitere%d@example.org", n), map[string][]int{"Tom": {0}, "Ida": {0}})
	}
	large := measure()
	assert.Len(t, large.Statements, len(small.Statements), "weeks and waiting children do not add statements")
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_places", small)
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_places", large)
}

func TestHolidayWaitlistRanksKeepTheWholeWeekWhenReadingOneRegistration(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Plätze Rang Einzelanmeldung")
	first := f.offer.Weeks[0]
	f.setCapacity(t, first.ID, 2)
	tom := f.submitPlaces(t, "rang.tom@example.org", map[string][]int{"Tom": {0}})
	ida := f.submitPlaces(t, "rang.ida@example.org", map[string][]int{"Ida": {0}})
	detail := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(ida.ID, ""), nil))
	require.NotNil(t, detail.Children[0].Bookings[0].WaitlistPosition)
	assert.Equal(t, 2, *detail.Children[0].Bookings[0].WaitlistPosition)
	rec := f.cancel(t, f.office.token, tom.ID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail = f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(ida.ID, ""), nil))
	require.NotNil(t, detail.Children[0].Bookings[0].WaitlistPosition)
	assert.Equal(t, 1, *detail.Children[0].Bookings[0].WaitlistPosition)
}
