package calendar

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	calendarService "github.com/moto-nrw/project-phoenix/services/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Resource struct {
	service calendarService.Service
	db      *bun.DB
	logger  *slog.Logger
}

func NewResource(service calendarService.Service, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{service: service, db: db, logger: logger}
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	tokenAuth := jwt.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		own := authorize.RequiresPermission(permissions.CalendarOwn)
		manage := authorize.RequiresPermission(permissions.CalendarManage)
		r.With(own, withTx).Get("/my", rs.listMy)
		r.With(own, withTx).Get("/appointments/{appointmentId}/overview", rs.appointmentOverview)
		r.With(own, withTx).Get("/appointments/{appointmentId}/ics", rs.appointmentICS)
		r.With(manage, withTx).Post("/appointments", rs.createAppointment)
		r.With(manage, withTx).Get("/appointments/{appointmentId}", rs.getAppointmentDetail)
		r.With(manage, withTx).Put("/appointments/{appointmentId}", rs.updateAppointment)
		r.With(manage, withTx).Delete("/appointments/{appointmentId}", rs.deleteAppointment)
		r.With(manage, withTx).Post("/appointments/{appointmentId}/cancel", rs.cancelAppointment)
		r.With(manage, withTx).Post("/appointments/{appointmentId}/occurrences/{occurrenceDate}/cancel", rs.cancelOccurrence)
		r.With(own, withTx).Post("/recipients/{recipientId}/response", rs.respond)
		r.With(manage, withTx).Get("/recipient-options", rs.recipientOptions)
	})
	return r
}

type createAppointmentRequest struct {
	Title              string                             `json:"title"`
	Description        *string                            `json:"description,omitempty"`
	Location           *string                            `json:"location,omitempty"`
	StartDate          timezone.Date                      `json:"start_date"`
	EndDate            timezone.Date                      `json:"end_date"`
	StartTime          string                             `json:"start_time"`
	EndTime            string                             `json:"end_time"`
	AllDay             bool                               `json:"all_day"`
	DeliveryMode       string                             `json:"delivery_mode"`
	OverviewVisibility string                             `json:"overview_visibility"`
	Recurrence         *calendarService.RecurrenceRequest `json:"recurrence,omitempty"`
	Targets            []appointmentTargetRequest         `json:"targets"`
	SendEmail          bool                               `json:"send_email"`
}

type updateAppointmentRequest struct {
	Title              string                             `json:"title"`
	Description        *string                            `json:"description,omitempty"`
	Location           *string                            `json:"location,omitempty"`
	StartDate          timezone.Date                      `json:"start_date"`
	EndDate            timezone.Date                      `json:"end_date"`
	StartTime          string                             `json:"start_time"`
	EndTime            string                             `json:"end_time"`
	AllDay             bool                               `json:"all_day"`
	OverviewVisibility string                             `json:"overview_visibility"`
	Recurrence         *calendarService.RecurrenceRequest `json:"recurrence,omitempty"`
	SendEmail          *bool                              `json:"send_email"`
}

type requestID int64

func (id *requestID) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		return nil
	}
	if strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`) {
		unquoted, err := strconv.Unquote(raw)
		if err != nil {
			return err
		}
		raw = strings.TrimSpace(unquoted)
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid id %q", raw)
	}
	*id = requestID(parsed)
	return nil
}

type appointmentTargetRequest struct {
	Type  string     `json:"type"`
	ID    *requestID `json:"id,omitempty"`
	Value *string    `json:"value,omitempty"`
}

func (target appointmentTargetRequest) serviceTarget() calendarService.AppointmentTarget {
	var id *int64
	if target.ID != nil {
		value := int64(*target.ID)
		id = &value
	}
	return calendarService.AppointmentTarget{
		Type:  target.Type,
		ID:    id,
		Value: target.Value,
	}
}

func serviceTargets(targets []appointmentTargetRequest) []calendarService.AppointmentTarget {
	out := make([]calendarService.AppointmentTarget, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.serviceTarget())
	}
	return out
}

type responseRequest struct {
	Status string `json:"status"`
}

func (rs *Resource) listMy(w http.ResponseWriter, r *http.Request) {
	from, to, ok := parseRange(w, r)
	if !ok {
		return
	}
	events, err := rs.service.ListMyStaffEvents(r.Context(), from, to)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{
		"from":   from.String(),
		"to":     to.String(),
		"events": events,
	}, "Calendar events retrieved")
}

func (rs *Resource) createAppointment(w http.ResponseWriter, r *http.Request) {
	var req createAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	startTime, err := parseClock(req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	endTime, err := parseClock(req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.service.CreateStaffAppointment(r.Context(), calendarService.CreateAppointmentRequest{
		Title:              req.Title,
		Description:        req.Description,
		Location:           req.Location,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		StartTime:          startTime,
		EndTime:            endTime,
		AllDay:             req.AllDay,
		DeliveryMode:       req.DeliveryMode,
		OverviewVisibility: req.OverviewVisibility,
		Recurrence:         req.Recurrence,
		Targets:            serviceTargets(req.Targets),
		SendEmail:          req.SendEmail,
	})
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, detail, "Appointment created")
}

func (rs *Resource) getAppointmentDetail(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	detail, err := rs.service.GetStaffAppointmentDetail(r.Context(), appointmentID)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Appointment detail retrieved")
}

func (rs *Resource) updateAppointment(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	var req updateAppointmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	startTime, err := parseClock(req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	endTime, err := parseClock(req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	detail, err := rs.service.UpdateStaffAppointment(r.Context(), appointmentID, calendarService.UpdateAppointmentRequest{
		Title:              req.Title,
		Description:        req.Description,
		Location:           req.Location,
		StartDate:          req.StartDate,
		EndDate:            req.EndDate,
		StartTime:          startTime,
		EndTime:            endTime,
		AllDay:             req.AllDay,
		OverviewVisibility: req.OverviewVisibility,
		Recurrence:         req.Recurrence,
		SendEmail:          req.SendEmail != nil && *req.SendEmail,
		SendEmailSet:       req.SendEmail != nil,
	})
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Appointment updated")
}

func (rs *Resource) cancelAppointment(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	detail, err := rs.service.CancelStaffAppointment(r.Context(), appointmentID)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Appointment cancelled")
}

func (rs *Resource) deleteAppointment(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	if err := rs.service.DeleteStaffAppointment(r.Context(), appointmentID); err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": "deleted"}, "Appointment deleted")
}

func (rs *Resource) cancelOccurrence(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	occurrenceDate, err := timezone.ParseDate(chi.URLParam(r, "occurrenceDate"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid occurrence date")))
		return
	}
	if err := rs.service.CancelStaffAppointmentOccurrence(r.Context(), appointmentID, occurrenceDate); err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": "cancelled"}, "Occurrence cancelled")
}

func (rs *Resource) appointmentOverview(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	overview, err := rs.service.GetStaffAppointmentOverview(r.Context(), appointmentID)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Appointment overview retrieved")
}

func (rs *Resource) appointmentICS(w http.ResponseWriter, r *http.Request) {
	appointmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "appointmentId", "invalid appointment ID")
	if !ok {
		return
	}
	filename, content, err := rs.service.StaffAppointmentICS(r.Context(), appointmentID)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	writeICS(w, filename, content)
}

// writeICS streams an iCalendar document as a downloadable attachment.
func writeICS(w http.ResponseWriter, filename, content string) {
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}

func (rs *Resource) respond(w http.ResponseWriter, r *http.Request) {
	recipientID, ok := common.ParsePositiveInt64IDWithError(w, r, "recipientId", "invalid recipient ID")
	if !ok {
		return
	}
	var req responseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.service.RespondToStaffInvitation(r.Context(), recipientID, req.Status); err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": req.Status}, "Invitation response updated")
}

func (rs *Resource) recipientOptions(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	options, err := rs.service.RecipientOptions(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		renderCalendarError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, options, "Recipient options retrieved")
}

func parseRange(w http.ResponseWriter, r *http.Request) (timezone.Date, timezone.Date, bool) {
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

func parseClock(raw string) (time.Time, error) {
	t, err := time.Parse("15:04", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q, expected HH:mm", raw)
	}
	return timezone.NormalizeWallClock(t), nil
}

func renderCalendarError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, calendarService.ErrInvalidRequest):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, calendarService.ErrForbidden):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, calendarService.ErrNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, calendarService.ErrConflict):
		common.RenderError(w, r, common.ErrorConflict(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("calendar operation failed", err))
	}
}
