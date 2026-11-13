package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

func TestHolidayAllocationCriterionScopeStaysAfterTheFirstAnswer(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger Kriterienbezug")
	// The editor opened before the first answer was submitted.
	stale := map[string]any{"criteria": []map[string]any{{"id": a.working, "name": "Berufstätig", "question": "Sind Sie berufstätig?", "scope": "child", "active": true}}}
	a.submitCriteria(t, "bezug@example.org", true, false)
	rec := traegerJSON(t, a.traeger, a.office.token, http.MethodPut, "/traeger/holiday-periods/criteria", stale)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, fields := registrationRefusalOf(t, rec)
	assert.Contains(t, fields, "criteria.0.scope")
	board := a.board(t)
	require.Len(t, board.Criteria, 2, "a refused save also rolls back archiving")
	assert.Equal(t, "yes", board.Weeks[0].Pending[0].Criteria[0].Value, "the answer keeps its allocation priority")
}

func TestHolidayAllocationNotifiesAgainAfterAddingDays(t *testing.T) {
	t.Parallel()
	d := newDayFixture(t, "Träger erneute Vergabe")
	rec := traegerJSON(t, d.traeger, d.office.token, http.MethodGet, holidayPeriodPath(d.period.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeSchoolPortalData[holidayPeriodBody](t, rec.Body.Bytes())
	body.AllocationMode = "criteria"
	rec = traegerJSON(t, d.traeger, d.office.token, http.MethodPut, holidayPeriodPath(d.period.ID), body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	a := allocationFixture{reviewFixture: reviewFixture{holidayRegistrationFixture: d.holidayRegistrationFixture, office: d.office, period: d.period}}
	registration := d.submit(t, d.registration("erneut@example.org", "Tom", d.days(dayMonday)))
	first := a.board(t).Weeks[0].Pending[0]
	rec = a.decide(t, decision(first, "placed", ""))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := a.detail(t, traegerJSON(t, a.traeger, a.office.token, http.MethodGet, a.registrationPath(registration.ID, ""), nil))
	rec = traegerJSON(t, d.traeger, d.office.token, http.MethodPut, holidayRebookPath(d.period.ID, registration.ID, detail.Children[0].ID),
		map[string]any{"bookings": []map[string]any{d.days(dayMonday, dayTuesday)}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	second := a.board(t).Weeks[0].Pending[0]
	require.Equal(t, first.BookingID, second.BookingID, "adding days preserves the booking identity")
	rec = a.decide(t, decision(second, "placed", ""))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 2, a.mails(t, "holiday_place_granted", registration.ID), "each persisted decision has its own mail")
	var payloads []string
	require.NoError(t, a.db.NewRaw(`SELECT payload::text FROM platform.email_outbox
  WHERE organization_id = ? AND kind = 'holiday_place_granted' AND related_entity_id = ? ORDER BY id`, a.office.organization, registration.ID).Scan(t.Context(), &payloads))
	require.Len(t, payloads, 2)
	assert.NotEqual(t, payloads[0], payloads[1], "the second mail includes the added day")
}

func TestHolidayAllocationKeepsTheLastWaitlistDecision(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger letzte Wartelistenentscheidung")
	registration := a.submitCriteria(t, "wartend@example.org", true, false)
	candidate := a.board(t).Weeks[0].Pending[0]
	rec := a.decide(t, decision(candidate, "waitlisted", "Familie benötigt vorerst keinen Platz"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	places := a.places(t)[a.offer.Weeks[0].ID]
	assert.Zero(t, places.Placed, "the decision must not immediately promote its own booking")
	require.Len(t, places.Waitlist, 1)
	assert.Equal(t, registration.ID, places.Waitlist[0].RegistrationID)
	assert.Equal(t, 1, a.mails(t, "holiday_place_granted", registration.ID), "only the office decision notifies the parents")
}

// A received child with a new direct place participates in its sibling group
// before confirmation, just like a child that moved up from the waiting list.
func TestHolidayAllocationDirectPlaceRefreezesSiblings(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger direkte Geschwistervergabe")
	billing := traegerColleague(t, a.reviewFixture, identityaccess.TraegerRoleBilling)
	createContributionRule(t, a.reviewFixture, billing, siblingHalf())
	first := a.submitCriteria(t, "erstes.geschwister@example.org", false, false)
	rec := a.decide(t, decision(a.board(t).Weeks[0].Pending[0], "placed", ""))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = a.act(t, first.ID, "confirm")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{4500}, a.contributionVersions(t, first.ID))

	body := placesRegistration(a.reviewFixture, "zweites.geschwister@example.org", map[string][]int{"Ida": {0}})
	body["criteria"] = map[string]bool{fmt.Sprint(a.working): true}
	child := body["children"].([]map[string]any)[0]
	child["criteria"] = map[string]bool{fmt.Sprint(a.sibling): true}
	child["bookings"].([]map[string]any)[0]["time_model_id"] = a.offer.Weeks[0].TimeModels[1].ID
	rec = publicJSON(t, a.public, http.MethodPost, registrationPath(a.period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	second := decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())
	rec = a.merge(t, a.office.token, first.ID, second.ID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, []int64{4500}, a.contributionVersions(t, first.ID), "a pending child does not affect the confirmed sibling")
	a.setCapacity(t, a.offer.Weeks[0].ID, 2)
	candidate := a.board(t).Weeks[0].Pending[0]
	require.Equal(t, "placed", candidate.Proposal)
	rec = a.decide(t, decision(candidate, "placed", ""))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, []int64{4500, 2250}, a.contributionVersions(t, first.ID), "the direct place changes the confirmed sibling's frozen discount before the new child is confirmed")
	contribution := a.siblingContribution(t, first.ID)
	assert.Equal(t, int64(2250), contribution.TotalCents)
	require.Len(t, contribution.Children, 1)
	assert.Equal(t, []string{"base:4500", "sibling:-2250"}, lineKinds(contribution.Children[0].Lines))
	assert.Empty(t, a.contributionVersions(t, second.ID), "the received registration does not freeze a contribution yet")
}
