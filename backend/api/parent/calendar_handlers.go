package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calendarService "github.com/moto-nrw/project-phoenix/services/calendar"
)

type parentCalendarResponseRequest struct {
	Status string `json:"status"`
}

func (rs *Resource) listMyCalendar(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	if rs.CalendarService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("calendar service is not configured")))
		return
	}
	from, to, ok := parseParentCalendarRange(w, r)
	if !ok {
		return
	}
	events, err := rs.CalendarService.ListMyParentEvents(r.Context(), accountID, from, to)
	if err != nil {
		renderParentCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"from":   from.String(),
		"to":     to.String(),
		"events": events,
	}, "Calendar events retrieved")
}

func (rs *Resource) respondToCalendarInvitation(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	if rs.CalendarService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("calendar service is not configured")))
		return
	}
	recipientID, err := strconv.ParseInt(chi.URLParam(r, "recipientId"), 10, 64)
	if err != nil || recipientID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid recipient ID")))
		return
	}
	var req parentCalendarResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if err := rs.CalendarService.RespondToParentInvitation(r.Context(), accountID, recipientID, req.Status); err != nil {
		renderParentCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": req.Status}, "Invitation response updated")
}

func (rs *Resource) calendarAppointmentOverview(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	if rs.CalendarService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("calendar service is not configured")))
		return
	}
	appointmentID, err := strconv.ParseInt(chi.URLParam(r, "appointmentId"), 10, 64)
	if err != nil || appointmentID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid appointment ID")))
		return
	}
	overview, err := rs.CalendarService.GetParentAppointmentOverview(r.Context(), accountID, appointmentID)
	if err != nil {
		renderParentCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Appointment overview retrieved")
}

func parseParentCalendarRange(w http.ResponseWriter, r *http.Request) (timezone.Date, timezone.Date, bool) {
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")
	if fromRaw == "" || toRaw == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query params are required")))
		return timezone.Date{}, timezone.Date{}, false
	}
	from, err := timezone.ParseDate(fromRaw)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date")))
		return timezone.Date{}, timezone.Date{}, false
	}
	to, err := timezone.ParseDate(toRaw)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date")))
		return timezone.Date{}, timezone.Date{}, false
	}
	return from, to, true
}

func renderParentCalendarError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, calendarService.ErrInvalidRequest):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, calendarService.ErrForbidden):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, calendarService.ErrNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("calendar operation failed", err))
	}
}
