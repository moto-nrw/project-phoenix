package careplanhttp

import (
	context "context"
	errors "errors"
	http "net/http"
	strings "strings"
	utf8 "unicode/utf8"

	chi "github.com/go-chi/chi/v5"
	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// DepartureExceptionRequest is the body of
// PUT /students/{id}/departure-exceptions/{date}.
type DepartureExceptionRequest struct {
	DepartureMode string  `json:"departure_mode"`
	Reason        *string `json:"reason,omitempty"`
}

func (req *DepartureExceptionRequest) Bind(_ *http.Request) error {
	req.DepartureMode = strings.TrimSpace(req.DepartureMode)
	if !careplan.ValidDepartureMode(req.DepartureMode) {
		return errors.New("departure_mode must be alone, bus, pickup or accompanied")
	}
	if req.Reason != nil {
		trimmed := strings.TrimSpace(*req.Reason)
		if utf8.RuneCountInString(trimmed) > 255 {
			return errors.New("reason cannot exceed 255 characters")
		}
		if trimmed == "" {
			req.Reason = nil
		} else {
			req.Reason = &trimmed
		}
	}
	return nil
}

// DepartureExceptionResponse is the Abholart of one child on one day.
type DepartureExceptionResponse struct {
	ID            int64   `json:"id"`
	StudentID     int64   `json:"student_id"`
	ExceptionDate string  `json:"exception_date"`
	DepartureMode string  `json:"departure_mode"`
	Reason        *string `json:"reason,omitempty"`
	CreatedBy     *int64  `json:"created_by,omitempty"`
	UpdatedAt     string  `json:"updated_at"`
}

func mapDepartureExceptionToResponse(value careplan.DepartureException) DepartureExceptionResponse {
	return DepartureExceptionResponse{
		ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(),
		DepartureMode: value.DepartureMode, Reason: value.Reason, CreatedBy: value.CreatedBy,
		UpdatedAt: value.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// listDepartureExceptions reads the day values the pickup data response
// carries next to the pickup exceptions: the requested range, or today and
// later without one, matching the pickup exceptions of the same response.
func (rs *Resource) listDepartureExceptions(ctx context.Context, studentID int64, from, to calendar.Date, hasRange bool) ([]DepartureExceptionResponse, error) {
	out := []DepartureExceptionResponse{}
	filter := careplan.StudentScheduleFilter{StudentIDs: []int64{studentID}}
	if hasRange {
		filter.From, filter.To = careplan.Date(from), careplan.Date(to)
	} else {
		filter.UpcomingFrom = careplan.Date(calendar.TodayDate())
	}
	rows, err := rs.PickupScheduleService.ListDayDepartures(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, mapDepartureExceptionToResponse(row))
	}
	return out, nil
}

// putStudentDepartureException handles
// PUT /students/{id}/departure-exceptions/{date}: the OGS sets the Abholart
// of one day directly, without a parent request (#3945).
func (rs *Resource) putStudentDepartureException(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "set departure exceptions")
	if student == nil {
		return
	}
	date, ok := departureExceptionDate(w, r)
	if !ok {
		return
	}
	req := &DepartureExceptionRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}
	saved, err := rs.PickupScheduleService.SetDayDeparture(r.Context(), student.ID, date, req.DepartureMode, req.Reason, staffID)
	if err != nil {
		if errors.Is(err, careplan.ErrDepartureExceptionNotCareDay) {
			renderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	rs.announceDepartureChange(r, student.ID)
	common.Respond(w, r, http.StatusOK, mapDepartureExceptionToResponse(*saved), "Departure exception saved successfully")
}

// deleteStudentDepartureException handles
// DELETE /students/{id}/departure-exceptions/{date}: the day falls back to
// the weekly plan's Abholart. Deleting a day without a value is a no-op.
func (rs *Resource) deleteStudentDepartureException(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "delete departure exceptions")
	if student == nil {
		return
	}
	date, ok := departureExceptionDate(w, r)
	if !ok {
		return
	}
	deleted, err := rs.PickupScheduleService.DeleteDayDeparture(r.Context(), student.ID, date)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if deleted {
		rs.announceDepartureChange(r, student.ID)
	}
	common.Respond(w, r, http.StatusOK, nil, "Departure exception deleted successfully")
}

func departureExceptionDate(w http.ResponseWriter, r *http.Request) (calendar.Date, bool) {
	date, err := calendar.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date, expected YYYY-MM-DD")))
		return "", false
	}
	return date, true
}

// announceDepartureChange wakes the child's guardians and the staff views
// after the request transaction commits, like the pickup exception writes.
func (rs *Resource) announceDepartureChange(r *http.Request, studentID int64) {
	tenantID := tenant.FromContext(r.Context())
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, studentID)
		rs.broadcastPickupScheduleChanged(tenantID, studentID)
		rs.broadcastStudentUpdated(tenantID, studentID)
	})
}
