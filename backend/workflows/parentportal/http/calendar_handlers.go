package parent

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-ogs/project-phoenix/api/common"
	calendarService "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

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
	events, err := rs.CalendarService.ListMyParentEvents(r.Context(), accountID, calendarService.Date(from), calendarService.Date(to))
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
	recipientID, ok := common.ParsePositiveInt64IDWithError(w, r, "recipientId", "invalid recipient ID")
	if !ok {
		return
	}
	var req calendarService.InvitationAnswer
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	if err := rs.CalendarService.RespondToParentInvitation(r.Context(), accountID, recipientID, req); err != nil {
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
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	// occurrence_date asks for the answers of one date of a series (#6987).
	var occurrenceDate calendarService.Date
	if raw := r.URL.Query().Get("occurrence_date"); raw != "" {
		parsed, err := timezone.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid occurrence date")))
			return
		}
		occurrenceDate = calendarService.Date(parsed)
	}
	overview, err := rs.CalendarService.GetParentAppointmentOverview(r.Context(), accountID, appointmentID, occurrenceDate)
	if err != nil {
		renderParentCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Appointment overview retrieved")
}

func (rs *Resource) calendarAppointmentICS(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	if rs.CalendarService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("calendar service is not configured")))
		return
	}
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	filename, content, err := rs.CalendarService.ParentAppointmentICS(r.Context(), accountID, appointmentID)
	if err != nil {
		renderParentCalendarError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", textnorm.ContentDisposition("attachment", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func (rs *Resource) calendarFeedURL(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	if rs.CalendarService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("calendar service is not configured")))
		return
	}
	httpsURL, webcalURL, err := rs.CalendarService.ParentCalendarFeedURL(r.Context(), accountID)
	if err != nil {
		renderParentCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{
		"url":        httpsURL,
		"webcal_url": webcalURL,
	}, "Calendar feed URL retrieved")
}

func (rs *Resource) rotateCalendarFeed(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	if rs.CalendarService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("calendar service is not configured")))
		return
	}
	httpsURL, webcalURL, err := rs.CalendarService.RotateParentCalendarFeed(r.Context(), accountID)
	if err != nil {
		renderParentCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{
		"url":        httpsURL,
		"webcal_url": webcalURL,
	}, "Calendar feed URL rotated")
}

func parseParentCalendarRange(w http.ResponseWriter, r *http.Request) (timezone.Date, timezone.Date, bool) {
	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")
	if fromRaw == "" || toRaw == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query params are required")))
		return timezone.Date(""), timezone.Date(""), false
	}
	from, err := timezone.ParseDate(fromRaw)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date")))
		return timezone.Date(""), timezone.Date(""), false
	}
	to, err := timezone.ParseDate(toRaw)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date")))
		return timezone.Date(""), timezone.Date(""), false
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
	case errors.Is(err, calendarService.ErrConflict):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, calendarService.ErrOccurrenceUnavailable):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeCalendarOccurrenceUnavailable))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("calendar operation failed", err))
	}
}
