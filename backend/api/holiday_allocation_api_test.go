// Router-level tests of the allocation of holiday places by criteria
// (#4700): no place when the parents send the form, the proposal sorted by
// the Träger's criteria, the Büro's decision with a reason for every
// deviation, no overbooking, the waiting list and moving up of #4699 after
// the decision, and the mode that no longer changes once registrations came
// in.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type allocationCandidateBody struct {
	BookingID      int64  `json:"booking_id"`
	Rank           int    `json:"rank"`
	RegistrationID int64  `json:"registration_id"`
	ChildFirstName string `json:"child_first_name"`
	Proposal       string `json:"proposal"`
	Criteria       []struct {
		CriterionID int64  `json:"criterion_id"`
		Value       string `json:"value"`
		Source      string `json:"source"`
	} `json:"criteria"`
}

type allocationBoardBody struct {
	AllocationMode string `json:"allocation_mode"`
	Criteria       []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"criteria"`
	Weeks []struct {
		WeekID  int64                     `json:"week_id"`
		Placed  int                       `json:"placed"`
		Free    *int                      `json:"free"`
		Pending []allocationCandidateBody `json:"pending"`
	} `json:"weeks"`
	Decisions []struct {
		RegistrationID int64    `json:"registration_id"`
		Placement      string   `json:"placement"`
		Proposal       string   `json:"proposal"`
		Rank           int      `json:"rank"`
		CriteriaMet    []string `json:"criteria_met"`
		Reason         string   `json:"reason"`
	} `json:"decisions"`
}

type criteriaBody struct {
	Periods []struct {
		ID int64 `json:"id"`
	} `json:"periods"`
	Criteria []struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Scope    string `json:"scope"`
		Active   bool   `json:"active"`
		Position int    `json:"position"`
	} `json:"criteria"`
}

// allocationFixture is a published period by criteria whose first week has
// one place, with two criteria of the Träger: "Berufstätig" for the family
// before "Geschwister" for each child.
type allocationFixture struct {
	reviewFixture
	working, sibling int64
}

func newAllocationFixture(t *testing.T, name string) allocationFixture {
	t.Helper()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, name, identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPut, "/traeger/holiday-periods/criteria", map[string]any{"criteria": []map[string]any{
		{"name": "Berufstätig", "question": "Sind Sie berufstätig?", "scope": "family", "active": true},
		{"name": "Geschwister", "question": "Ist ein Geschwisterkind schon in der OGS?", "scope": "child", "active": true},
		{"name": "Alt", "question": "Nicht mehr gefragt?", "scope": "family", "active": false},
	}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	criteria := decodeSchoolPortalData[criteriaBody](t, rec.Body.Bytes()).Criteria
	require.Len(t, criteria, 3)
	body := januaryHolidays(office)
	body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2099-12-31"
	body.AllocationMode = "criteria"
	created := createHolidayPeriod(t, f.traeger, office, body)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	a := allocationFixture{
		reviewFixture: reviewFixture{holidayRegistrationFixture: f, office: office, period: period, offer: holidayOffer(t, f, period.PublicToken)},
		working:       criteria[0].ID, sibling: criteria[1].ID,
	}
	a.setCapacity(t, a.offer.Weeks[0].ID, 1)
	return a
}

// submitCriteria registers one child for the first week with the family's
// and the child's answers.
func (a allocationFixture) submitCriteria(t *testing.T, email string, working, sibling bool) receiptBody {
	t.Helper()
	body := placesRegistration(a.reviewFixture, email, map[string][]int{"Tom": {0}})
	body["criteria"] = map[string]bool{fmt.Sprint(a.working): working}
	body["children"].([]map[string]any)[0]["criteria"] = map[string]bool{fmt.Sprint(a.sibling): sibling}
	rec := publicJSON(t, a.public, http.MethodPost, registrationPath(a.period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[receiptBody](t, rec.Body.Bytes())
}

type allocationOfferBody struct {
	AllocationMode string `json:"allocation_mode"`
	Criteria       []struct {
		ID    int64  `json:"id"`
		Scope string `json:"scope"`
	} `json:"criteria"`
}

func (a allocationFixture) publicOffer(t *testing.T) allocationOfferBody {
	t.Helper()
	rec := publicJSON(t, a.public, http.MethodGet, registrationPath(a.period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[allocationOfferBody](t, rec.Body.Bytes())
}

func (a allocationFixture) board(t *testing.T) allocationBoardBody {
	t.Helper()
	rec := traegerJSON(t, a.traeger, a.office.token, http.MethodGet, holidayPeriodPath(a.period.ID)+"/allocation", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[allocationBoardBody](t, rec.Body.Bytes())
}

func (a allocationFixture) decide(t *testing.T, decisions ...map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, a.traeger, a.office.token, http.MethodPost, holidayPeriodPath(a.period.ID)+"/allocation/decisions",
		map[string]any{"decisions": decisions})
}

func decision(candidate allocationCandidateBody, placement, reason string) map[string]any {
	return map[string]any{"booking_id": candidate.BookingID, "placement": placement, "proposal": candidate.Proposal, "reason": reason}
}

func (a allocationFixture) errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	code, _ := registrationRefusalOf(t, rec)
	return code
}

// TestHolidayAllocationByCriteriaWaitsForTheOfficeDecision pins the
// invariants of the allocation end to end. Each guard goes red without its
// protection: no place at sending (pending placement), no place by a higher
// capacity or a confirmation while pending, the order by the criteria, the
// reason for a deviation, the capacity check of a decision, the record of
// every decision, and the waiting list and moving up of #4699 afterwards.
func TestHolidayAllocationByCriteriaWaitsForTheOfficeDecision(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger Vergabe Kriterien")
	week := a.offer.Weeks[0]

	offer := a.publicOffer(t)
	assert.Equal(t, "criteria", offer.AllocationMode)
	require.Len(t, offer.Criteria, 2, "the form asks only the active criteria")
	assert.Equal(t, []int64{a.working, a.sibling}, []int64{offer.Criteria[0].ID, offer.Criteria[1].ID})

	missing := placesRegistration(a.reviewFixture, "ohne.angabe@example.org", map[string][]int{"Tom": {0}})
	rec := publicJSON(t, a.public, http.MethodPost, registrationPath(a.period.PublicToken), missing)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, fields := registrationRefusalOf(t, rec)
	assert.ElementsMatch(t, []string{fmt.Sprintf("criteria.%d", a.working), fmt.Sprintf("children.0.criteria.%d", a.sibling)}, fields,
		"every question the active criteria ask needs an answer")

	early := a.submitCriteria(t, "frueh@example.org", false, false)
	late := a.submitCriteria(t, "spaet@example.org", true, false)
	assert.Equal(t, []placementBody{{ChildPosition: 1, WeekID: week.ID, Placement: "pending"}}, early.Placements, "no place when the form is sent")
	assert.Equal(t, []placementBody{{ChildPosition: 1, WeekID: week.ID, Placement: "pending"}}, late.Placements)
	assert.Equal(t, 0, a.places(t)[week.ID].Placed)
	var receipt string
	require.NoError(t, a.db.NewRaw(`SELECT payload::text FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'holiday_registration_received' AND related_entity_id = ?`, a.office.organization, early.ID).
		Scan(context.Background(), &receipt))
	assert.Contains(t, receipt, `"pending_weeks"`, "the receipt names the weeks the Büro still decides on")
	assert.NotContains(t, receipt, "Berufstätig", "the receipt names no criterion")

	assert.Empty(t, a.setCapacity(t, week.ID, 2).MovedUp, "a higher capacity gives a pending booking no place")
	a.setCapacity(t, week.ID, 1)
	rec = a.act(t, early.ID, "confirm")
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.holiday_allocation_pending", a.errorCode(t, rec), "no confirmation before the decision")

	board := a.board(t)
	require.Len(t, board.Weeks, 2)
	pending := board.Weeks[0].Pending
	require.Len(t, pending, 2)
	assert.Equal(t, late.ID, pending[0].RegistrationID, "the family that meets the first criterion comes first despite arriving later")
	assert.Equal(t, []string{"placed", "waitlisted"}, []string{pending[0].Proposal, pending[1].Proposal}, "the proposal fills the one place")
	assert.Equal(t, "yes", pending[0].Criteria[0].Value)
	assert.Equal(t, "parent", pending[0].Criteria[0].Source)
	require.NotNil(t, board.Weeks[0].Free)
	assert.Equal(t, 1, *board.Weeks[0].Free)

	rec = a.decide(t, decision(pending[1], "placed", ""))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_invalid", code, "a deviation needs a reason")
	assert.Equal(t, []string{"decisions.0.reason"}, fields)
	stale := decision(pending[0], "placed", "")
	stale["proposal"], stale["reason"] = "waitlisted", "Alter Stand"
	assert.Equal(t, "enrollment.holiday_allocation_changed", a.errorCode(t, a.decide(t, stale)), "a proposal the Büro did not see is refused")
	rec = a.decide(t, decision(pending[0], "placed", ""), decision(pending[1], "placed", "Härtefall nach Gespräch"))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.holiday_allocation_full", a.errorCode(t, rec), "no decision overbooks a week")
	assert.Equal(t, 0, a.places(t)[week.ID].Placed, "a refused decision changes nothing")

	rec = a.decide(t, decision(pending[0], "waitlisted", "Kein Nachweis der Berufstätigkeit"), decision(pending[1], "placed", "Härtefall nach Gespräch"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	board = decodeSchoolPortalData[allocationBoardBody](t, rec.Body.Bytes())
	assert.Empty(t, board.Weeks[0].Pending)
	require.Len(t, board.Decisions, 2)
	reasons := map[int64]string{}
	for _, decided := range board.Decisions {
		reasons[decided.RegistrationID] = decided.Reason
		if decided.RegistrationID == late.ID {
			assert.Equal(t, []string{"Berufstätig"}, decided.CriteriaMet, "the decision keeps the criteria met at that moment")
			assert.Equal(t, 1, decided.Rank)
			assert.Equal(t, "placed", decided.Proposal)
		}
	}
	assert.Equal(t, map[int64]string{early.ID: "Härtefall nach Gespräch", late.ID: "Kein Nachweis der Berufstätigkeit"}, reasons)
	places := a.places(t)[week.ID]
	assert.Equal(t, 1, places.Placed)
	require.Len(t, places.Waitlist, 1)
	assert.Equal(t, late.ID, places.Waitlist[0].RegistrationID)
	assert.Equal(t, 1, a.mails(t, "holiday_place_granted", early.ID), "the parents hear of the decision")
	assert.Equal(t, 1, a.mails(t, "holiday_place_granted", late.ID))
	history := a.detail(t, traegerJSON(t, a.traeger, a.office.token, http.MethodGet, a.registrationPath(early.ID, ""), nil)).History
	assert.Equal(t, "allocate", history[len(history)-1].Action, "the decision is in the registration's trail")
	assert.Contains(t, a.auditActions(t), "allocation", "the read of the list is recorded")

	// After the decision the waiting list of #4699 holds: confirmed, then
	// cancelled, the placed child frees its place for the waiting one.
	confirmed := a.act(t, early.ID, "confirm")
	require.Equal(t, http.StatusOK, confirmed.Code, confirmed.Body.String())
	rec = a.act(t, early.ID, "cancel")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{late.ID}, decodeSchoolPortalData[placeChangeBody](t, rec.Body.Bytes()).MovedUp)
	assert.Equal(t, 2, a.mails(t, "holiday_place_granted", late.ID))
}

// TestHolidayAllocationHoldsFreePlacesForThePendingBookings: while a week has
// a booking that waits for the Büro, a freed place goes to nobody
// automatically, not even to the waiting list, so a booking that meets the
// criteria is not passed by one that came in earlier. Without the hold the
// waiting booking moves up.
func TestHolidayAllocationHoldsFreePlacesForThePendingBookings(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger Vergabe Halten")
	week := a.offer.Weeks[0]
	first := a.submitCriteria(t, "erste@example.org", false, false)
	second := a.submitCriteria(t, "zweite@example.org", false, false)
	pending := a.board(t).Weeks[0].Pending
	rec := a.decide(t, decision(pending[0], "placed", ""), decision(pending[1], "waitlisted", ""))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	third := a.submitCriteria(t, "dritte@example.org", false, false)

	rec = a.act(t, first.ID, "cancel")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[placeChangeBody](t, rec.Body.Bytes()).MovedUp, "nobody moves up while a booking waits for the Büro")
	places := a.places(t)[week.ID]
	assert.Equal(t, 0, places.Placed)
	require.Len(t, places.Waitlist, 1)
	assert.Equal(t, second.ID, places.Waitlist[0].RegistrationID)
	board := a.board(t)
	require.Len(t, board.Weeks[0].Pending, 1)
	assert.Equal(t, third.ID, board.Weeks[0].Pending[0].RegistrationID)
	assert.Equal(t, "placed", board.Weeks[0].Pending[0].Proposal, "the freed place is proposed to the pending booking")

	// The existing waiting child has priority over the last pending child.
	rec = traegerJSON(t, a.traeger, a.office.token, http.MethodPut, a.registrationPath(second.ID, "/criteria"),
		map[string]any{"checks": []map[string]any{{"criterion_id": a.working, "checked": "yes"}}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = a.decide(t, decision(board.Weeks[0].Pending[0], "waitlisted", "Familie benötigt vorerst keinen Platz"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	places = a.places(t)[week.ID]
	assert.Equal(t, 1, places.Placed, "the last pending decision releases the hold")
	require.Len(t, places.Waitlist, 1)
	assert.Equal(t, third.ID, places.Waitlist[0].RegistrationID)
	assert.Equal(t, 2, a.mails(t, "holiday_place_granted", second.ID), "decision and later promotion each notify the parents")
}

// TestHolidayAllocationOfficeCheckCountsBeforeTheStatement: the Büro's check
// of an answer counts instead of the parents' statement and moves the
// booking in the proposal; it is in the detail and the trail.
func TestHolidayAllocationOfficeCheckCountsBeforeTheStatement(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger Vergabe Prüfung")
	early := a.submitCriteria(t, "geprueft@example.org", false, false)
	late := a.submitCriteria(t, "ungeprueft@example.org", true, false)
	assert.Equal(t, late.ID, a.board(t).Weeks[0].Pending[0].RegistrationID)

	rec := traegerJSON(t, a.traeger, a.office.token, http.MethodPut, a.registrationPath(late.ID, "/criteria"),
		map[string]any{"checks": []map[string]any{{"criterion_id": a.working, "child_id": nil, "checked": "no"}}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"checked":"no"`)
	pending := a.board(t).Weeks[0].Pending
	assert.Equal(t, early.ID, pending[0].RegistrationID, "the checked answer counts: arrival decides again")
	assert.Equal(t, "office", pending[1].Criteria[0].Source)

	rec = traegerJSON(t, a.traeger, a.office.token, http.MethodPut, a.registrationPath(late.ID, "/criteria"),
		map[string]any{"checks": []map[string]any{{"criterion_id": a.working, "child_id": nil, "checked": nil}}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, late.ID, a.board(t).Weeks[0].Pending[0].RegistrationID, "taking the check back restores the statement")
}

// TestHolidayAllocationModeAndRights: the mode no longer changes once a
// registration came in, a period by arrival has no allocation list, a role
// without the right to manage registrations gets 403 and another Träger
// sees nothing of the period.
func TestHolidayAllocationModeAndRights(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger Vergabe Rechte")
	current := traegerJSON(t, a.traeger, a.office.token, http.MethodGet, holidayPeriodPath(a.period.ID), nil)
	require.Equal(t, http.StatusOK, current.Code)
	body := decodeSchoolPortalData[holidayPeriodBody](t, current.Body.Bytes())
	assert.Equal(t, "criteria", body.AllocationMode)
	a.submitCriteria(t, "sperre@example.org", true, true)

	body.AllocationMode = "arrival"
	rec := traegerJSON(t, a.traeger, a.office.token, http.MethodPut, holidayPeriodPath(a.period.ID), body)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.holiday_allocation_mode_locked", a.errorCode(t, rec))
	body.AllocationMode = "criteria"
	rec = traegerJSON(t, a.traeger, a.office.token, http.MethodPut, holidayPeriodPath(a.period.ID), body)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String(), "keeping the mode is no change")
	assert.True(t, strings.Contains(rec.Body.String(), `"allocation_mode_locked":true`))

	byArrival := openHolidays(t, a.holidayRegistrationFixture, a.office)
	rec = traegerJSON(t, a.traeger, a.office.token, http.MethodGet, holidayPeriodPath(byArrival.ID)+"/allocation", nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "a period by arrival has no allocation list")

	readOnly := newTraegerOffice(t, a.db, a.traeger, "Vergabe Lesend", identityaccess.TraegerRoleReadOnly)
	for _, path := range []string{holidayPeriodPath(a.period.ID) + "/allocation", "/traeger/holiday-periods/criteria"} {
		assert.Equal(t, http.StatusForbidden, traegerJSON(t, a.traeger, readOnly.token, http.MethodGet, path, nil).Code, path)
	}
	foreign := newTraegerOffice(t, a.db, a.traeger, "Vergabe Fremd", identityaccess.TraegerRoleAdministration)
	assert.Equal(t, http.StatusNotFound, traegerJSON(t, a.traeger, foreign.token, http.MethodGet, holidayPeriodPath(a.period.ID)+"/allocation", nil).Code)
	rec = traegerJSON(t, a.traeger, foreign.token, http.MethodGet, "/traeger/holiday-periods/criteria", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, decodeSchoolPortalData[criteriaBody](t, rec.Body.Bytes()).Criteria, "another Träger sees none of these criteria")
}

// TestHolidayAllocationWaitingListFollowsTheCriteria: after the decision the
// waiting list of a period by criteria is sorted by the criteria before the
// arrival, and a freed place goes to its first booking. Sorted by arrival
// alone, the earlier booking without a criterion would move up.
func TestHolidayAllocationWaitingListFollowsTheCriteria(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger Vergabe Warteliste")
	week := a.offer.Weeks[0]
	first := a.submitCriteria(t, "zuerst@example.org", false, false)
	second := a.submitCriteria(t, "dann@example.org", false, false)
	third := a.submitCriteria(t, "zuletzt@example.org", true, false)
	byRegistration := map[int64]allocationCandidateBody{}
	for _, candidate := range a.board(t).Weeks[0].Pending {
		byRegistration[candidate.RegistrationID] = candidate
	}
	rec := a.decide(t,
		decision(byRegistration[first.ID], "placed", "Vereinbarung aus dem Vorjahr"),
		decision(byRegistration[second.ID], "waitlisted", ""),
		decision(byRegistration[third.ID], "waitlisted", "Nachweis fehlt noch"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	waitlist := a.places(t)[week.ID].Waitlist
	require.Len(t, waitlist, 2)
	assert.Equal(t, []int64{third.ID, second.ID}, []int64{waitlist[0].RegistrationID, waitlist[1].RegistrationID},
		"the family that meets the criterion waits first")
	rec = a.act(t, first.ID, "cancel")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []int64{third.ID}, decodeSchoolPortalData[placeChangeBody](t, rec.Body.Bytes()).MovedUp)
}

// TestHolidayAllocationQueryBudgetIsFlat: the allocation list and the
// criteria read a fixed number of statements, however many weeks, pending
// bookings, answers and decisions there are.
func TestHolidayAllocationQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Vergabe Query-Budget")
	a.submitCriteria(t, "budget.eins@example.org", true, false)
	measure := func(path string) testpkg.QueryMeasurement {
		t.Helper()
		counter := testpkg.CaptureQueriesForContext(t, a.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+a.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, a.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	board, criteria := holidayPeriodPath(a.period.ID)+"/allocation", "/traeger/holiday-periods/criteria"
	smallBoard, smallCriteria := measure(board), measure(criteria)
	for n := 0; n < 3; n++ {
		a.submitCriteria(t, fmt.Sprintf("budget.weitere%d@example.org", n), n%2 == 0, true)
	}
	pending := a.board(t).Weeks[0].Pending
	rec := a.decide(t, decision(pending[0], pending[0].Proposal, ""), decision(pending[1], pending[1].Proposal, ""))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	largeBoard, largeCriteria := measure(board), measure(criteria)
	assert.Len(t, largeBoard.Statements, len(smallBoard.Statements), "bookings, answers and decisions do not add statements")
	assert.Len(t, largeCriteria.Statements, len(smallCriteria.Statements))
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_allocation", smallBoard)
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_allocation", largeBoard)
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_criteria", smallCriteria)
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_criteria", largeCriteria)
}

// TestHolidayAllocationTakesTheHolidayWishAsTheFirstCriterion: in a period
// by criteria that takes the holiday care wish into account (#4703), the
// wish is the criterion of highest priority: a child with a known "no"
// waits behind every other, even one that meets a criterion of the Träger,
// and the list names the wish first. Without the wish in the order key the
// child that meets "Berufstätig" would come first.
func TestHolidayAllocationTakesTheHolidayWishAsTheFirstCriterion(t *testing.T) {
	t.Parallel()
	a := newAllocationFixture(t, "Träger Vergabe Ferienwunsch")
	without := a.submitCriteria(t, "ohne.wunsch@example.org", true, true)
	plain := a.submitCriteria(t, "ohne.kriterium@example.org", false, false)
	ctx := context.Background()
	_, err := a.db.ExecContext(ctx, `UPDATE enrollment.holiday_periods SET holiday_wish_rule = true WHERE id = ?`, a.period.ID)
	require.NoError(t, err)
	_, err = a.db.ExecContext(ctx, `UPDATE enrollment.holiday_registration_children SET holiday_wish = 'no', holiday_wish_source = 'office'
		WHERE holiday_registration_id = ?`, without.ID)
	require.NoError(t, err)

	board := a.board(t)
	require.NotEmpty(t, board.Criteria)
	assert.Equal(t, "Ferienwunsch aus der OGS-Anmeldung", board.Criteria[0].Name, "the wish is the first criterion")
	pending := board.Weeks[0].Pending
	require.Len(t, pending, 2)
	assert.Equal(t, []int64{plain.ID, without.ID}, []int64{pending[0].RegistrationID, pending[1].RegistrationID},
		"a known no waits behind every other child")
	assert.Equal(t, "no", pending[1].Criteria[0].Value)
	assert.Equal(t, "office", pending[1].Criteria[0].Source)
	assert.Equal(t, "placed", pending[0].Proposal)
}
