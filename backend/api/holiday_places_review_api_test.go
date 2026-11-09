package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
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
