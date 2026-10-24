package staffcalendarhttp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	calendarService "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SchoolRouter mounts the calendar of moto schule (#4798) under
// /school/calendar: the Lehrkraft's appointments with the closing days and
// holidays, the own Schultermine, RSVPs and the subscription feed. Every
// route sits behind calendar:school.
func (rs *Resource) SchoolRouter(protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	protectedSchoolRoutes(r, func(r chi.Router, withTx common.Middleware) {
		school := common.RequiresPermission(permissions.CalendarSchool)
		r.With(school, withTx).Get("/my", rs.listSchoolCalendar)
		r.With(school, withTx).Get("/feed", rs.schoolCalendarFeed)
		r.With(school, withTx).Post("/feed/rotate", rs.rotateSchoolCalendarFeed)
		r.With(school, withTx).Post("/appointments", rs.createSchoolAppointment)
		r.With(school, withTx).Put("/appointments/{appointmentId}", rs.updateSchoolAppointment)
		r.With(school, withTx).Delete("/appointments/{appointmentId}", rs.deleteSchoolAppointment)
		r.With(school, withTx).Post("/recipients/{recipientId}/response", rs.respondSchoolInvitation)
	})
	return r
}

type schoolAppointmentRequest struct {
	Title           string        `json:"title"`
	Description     *string       `json:"description,omitempty"`
	Location        *string       `json:"location,omitempty"`
	StartDate       timezone.Date `json:"start_date"`
	EndDate         timezone.Date `json:"end_date"`
	StartTime       string        `json:"start_time"`
	EndTime         string        `json:"end_time"`
	AllDay          bool          `json:"all_day"`
	SchoolEventType string        `json:"school_event_type"`
	// Parents is "none", "classes" or "school"; read on creation only.
	Parents string   `json:"parents"`
	Classes []string `json:"classes,omitempty"`
}

func decodeSchoolAppointment(w http.ResponseWriter, r *http.Request) (calendarService.SchoolAppointmentRequest, bool) {
	var req schoolAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return calendarService.SchoolAppointmentRequest{}, false
	}
	startTime, err := parseClock(req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return calendarService.SchoolAppointmentRequest{}, false
	}
	endTime, err := parseClock(req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return calendarService.SchoolAppointmentRequest{}, false
	}
	return calendarService.SchoolAppointmentRequest{
		Title:       req.Title,
		Description: req.Description,
		Location:    req.Location,
		StartDate:   calendarService.Date(req.StartDate),
		EndDate:     calendarService.Date(req.EndDate),
		StartTime:   startTime,
		EndTime:     endTime,
		AllDay:      req.AllDay,
		EventType:   req.SchoolEventType,
		Parents:     req.Parents,
		Classes:     req.Classes,
	}, true
}

func (rs *Resource) listSchoolCalendar(w http.ResponseWriter, r *http.Request) {
	from, to, ok := parseRange(w, r)
	if !ok {
		return
	}
	view, err := rs.service.ListSchoolCalendar(r.Context(), calendarService.Date(from), calendarService.Date(to))
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"from":         from.String(),
		"to":           to.String(),
		"events":       view.Events,
		"closing_days": view.ClosingDays,
		"holidays":     view.Holidays,
		"classes":      view.Classes,
	}, "School calendar retrieved")
}

func (rs *Resource) schoolCalendarFeed(w http.ResponseWriter, r *http.Request) {
	httpsURL, webcalURL, err := rs.service.SchoolCalendarFeedURL(r.Context())
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	common.Respond(w, r, http.StatusOK, map[string]string{"url": httpsURL, "webcal_url": webcalURL}, "Calendar feed URL retrieved")
}

func (rs *Resource) rotateSchoolCalendarFeed(w http.ResponseWriter, r *http.Request) {
	httpsURL, webcalURL, err := rs.service.RotateSchoolCalendarFeed(r.Context())
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	common.Respond(w, r, http.StatusOK, map[string]string{"url": httpsURL, "webcal_url": webcalURL}, "Calendar feed URL rotated")
}

func (rs *Resource) createSchoolAppointment(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeSchoolAppointment(w, r)
	if !ok {
		return
	}
	detail, err := rs.service.CreateSchoolAppointment(r.Context(), req)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, detail, "School appointment created")
}

func (rs *Resource) updateSchoolAppointment(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	req, ok := decodeSchoolAppointment(w, r)
	if !ok {
		return
	}
	detail, err := rs.service.UpdateSchoolAppointment(r.Context(), appointmentID, req)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "School appointment updated")
}

func (rs *Resource) deleteSchoolAppointment(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	if err := rs.service.DeleteSchoolAppointment(r.Context(), appointmentID); err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": "deleted"}, "School appointment deleted")
}

func (rs *Resource) respondSchoolInvitation(w http.ResponseWriter, r *http.Request) {
	recipientID, ok := common.ParsePositiveInt64IDWithError(w, r, "recipientId", "invalid recipient ID")
	if !ok {
		return
	}
	var req responseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.service.RespondToSchoolInvitation(r.Context(), recipientID, req.Status); err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": req.Status}, "Invitation response updated")
}
