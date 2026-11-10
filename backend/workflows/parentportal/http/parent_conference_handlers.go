package parent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Elternsprechtag (#4801). The workflow resolves the child through its
// guardian relationship first: parent_portal.access to see the days,
// parent_portal.conference.book to book, move or cancel. The calendar then
// checks that the child belongs to the class of the day, in the transaction
// this flow opens for the child's school.

type parentConferenceBookingRequest struct {
	StartTime string `json:"start_time"`
}

func (rs *Resource) listChildParentConferences(w http.ResponseWriter, r *http.Request) {
	child, ok := rs.parentConferenceChild(w, r)
	if !ok {
		return
	}
	var offers []calendarService.ParentConferenceOffer
	err := withinTenant(r.Context(), child.TenantID, func(ctx context.Context) error {
		var listErr error
		offers, listErr = rs.Conferences.ListChildParentConferences(ctx, child)
		return listErr
	})
	if err != nil {
		renderParentConferenceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, offers, "Parent conferences retrieved")
}

func (rs *Resource) bookChildParentConference(w http.ResponseWriter, r *http.Request) {
	conferenceID, ok := common.ParsePositiveInt64IDWithError(w, r, "conferenceId", "invalid parent conference ID")
	if !ok {
		return
	}
	var req parentConferenceBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	var offer *calendarService.ParentConferenceOffer
	ok = rs.withParentConferenceBooking(w, r, func(ctx context.Context, child calendarService.ParentConferenceChild) error {
		var bookErr error
		offer, bookErr = rs.Conferences.BookParentConference(ctx, child, conferenceID, req.StartTime)
		return bookErr
	})
	if !ok {
		return
	}
	common.Respond(w, r, http.StatusOK, offer, "Parent conference booked")
}

func (rs *Resource) cancelChildParentConference(w http.ResponseWriter, r *http.Request) {
	conferenceID, ok := common.ParsePositiveInt64IDWithError(w, r, "conferenceId", "invalid parent conference ID")
	if !ok {
		return
	}
	ok = rs.withParentConferenceBooking(w, r, func(ctx context.Context, child calendarService.ParentConferenceChild) error {
		return rs.Conferences.CancelParentConferenceBooking(ctx, child, conferenceID)
	})
	if !ok {
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": "cancelled"}, "Parent conference booking cancelled")
}

// parentConferenceChild resolves the child of the path for the account of
// the session with parent_portal.access, for reading the days; booking
// changes go through withParentConferenceBooking.
func (rs *Resource) parentConferenceChild(w http.ResponseWriter, r *http.Request) (calendarService.ParentConferenceChild, bool) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return calendarService.ParentConferenceChild{}, false
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return calendarService.ParentConferenceChild{}, false
	}
	if rs.Conferences == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent conferences are not configured")))
		return calendarService.ParentConferenceChild{}, false
	}
	child, err := rs.ParentService.ParentConferenceChild(r.Context(), accountID, studentID, false)
	if err != nil {
		renderParentWriteError(w, r, err)
		return calendarService.ParentConferenceChild{}, false
	}
	return calendarService.ParentConferenceChild{
		TenantID: child.TenantID, StudentID: studentID, GuardianProfileID: child.GuardianProfileID,
	}, true
}

// withParentConferenceBooking runs a booking change of the path's child in
// the child's school unit behind the workflow's child write guard, and
// renders a refusal: the guard's as a parent write error, the calendar's as
// a conference error.
func (rs *Resource) withParentConferenceBooking(w http.ResponseWriter, r *http.Request, write func(context.Context, calendarService.ParentConferenceChild) error) bool {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return false
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return false
	}
	if rs.Conferences == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent conferences are not configured")))
		return false
	}
	var calendarErr error
	err := rs.ParentService.WithParentConferenceBooking(r.Context(), accountID, studentID, func(ctx context.Context, child *parentService.Child) error {
		calendarErr = write(ctx, calendarService.ParentConferenceChild{
			TenantID: child.TenantID, StudentID: studentID, GuardianProfileID: child.GuardianProfileID,
		})
		return calendarErr
	})
	switch {
	case err == nil:
		return true
	case calendarErr != nil && errors.Is(err, calendarErr):
		renderParentConferenceError(w, r, err)
	default:
		renderParentWriteError(w, r, err)
	}
	return false
}

func renderParentConferenceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, calendarService.ErrParentConferenceSlotTaken):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeSchoolParentConferenceSlotTaken))
	case errors.Is(err, calendarService.ErrParentConferenceChangeClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeSchoolParentConferenceChangeClosed))
	default:
		renderParentCalendarError(w, r, err)
	}
}
