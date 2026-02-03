package timetracking

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Error message constants
const errInvalidSessionID = "invalid session ID"

// Resource defines the time-tracking API resource
type Resource struct {
	WorkSessionService  activeSvc.WorkSessionService
	StaffAbsenceService activeSvc.StaffAbsenceService
	PersonService       usersSvc.PersonService
}

// NewResource creates a new time-tracking resource
func NewResource(workSessionService activeSvc.WorkSessionService, staffAbsenceService activeSvc.StaffAbsenceService, personService usersSvc.PersonService) *Resource {
	return &Resource{
		WorkSessionService:  workSessionService,
		StaffAbsenceService: staffAbsenceService,
		PersonService:       personService,
	}
}

// Router returns a configured router for time-tracking endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// All time-tracking endpoints require TimeTrackingOwn permission
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/check-in", rs.checkIn)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/check-out", rs.checkOut)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/current", rs.getCurrent)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/history", rs.getHistory)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Put("/{id}", rs.updateSession)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/{id}/edits", rs.getSessionEdits)

		// Break management
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/break/start", rs.startBreak)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/break/end", rs.endBreak)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/breaks/{sessionId}", rs.getBreaks)

		// Export
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/export", rs.exportSessions)

		// Absence management
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Get("/absences", rs.listAbsences)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Post("/absences", rs.createAbsence)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Put("/absences/{id}", rs.updateAbsence)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn)).Delete("/absences/{id}", rs.deleteAbsence)

		// Presence map - for internal use by staff page
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/presence-map", rs.getPresenceMap)
	})

	return r
}

// CheckInRequest represents a check-in request
type CheckInRequest struct {
	Status string `json:"status"` // "present" or "home_office"
}

// Bind validates the check-in request
func (req *CheckInRequest) Bind(_ *http.Request) error {
	if req.Status != "present" && req.Status != "home_office" {
		return errors.New("status must be 'present' or 'home_office'")
	}
	return nil
}

// getStaffIDFromClaims resolves JWT account ID to staff ID through PersonService
func (rs *Resource) getStaffIDFromClaims(ctx context.Context, claims jwt.AppClaims) (int64, error) {
	if claims.ID == 0 {
		return 0, errors.New("invalid token")
	}

	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil {
		return 0, errors.New("person not found for account")
	}

	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil {
		return 0, errors.New("staff record not found")
	}

	return staff.ID, nil
}

// parseDateRange extracts and validates "from" and "to" query parameters as dates.
// Returns the parsed times or renders an error and returns false.
func parseDateRange(w http.ResponseWriter, r *http.Request) (from, to time.Time, ok bool) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return time.Time{}, time.Time{}, false
	}

	from, err := time.Parse(common.DateFormatISO, fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return time.Time{}, time.Time{}, false
	}

	to, err = time.Parse(common.DateFormatISO, toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return time.Time{}, time.Time{}, false
	}

	return from, to, true
}

// checkIn handles POST /api/time-tracking/check-in
func (rs *Resource) checkIn(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &CheckInRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Call service to check in
	session, err := rs.WorkSessionService.CheckIn(r.Context(), staffID, req.Status)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Check-in successful")
}

// checkOut handles POST /api/time-tracking/check-out
func (rs *Resource) checkOut(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Call service to check out
	session, err := rs.WorkSessionService.CheckOut(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Check-out successful")
}

// getCurrent handles GET /api/time-tracking/current
func (rs *Resource) getCurrent(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Get current session
	session, err := rs.WorkSessionService.GetCurrentSession(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return null if no active session (not an error)
	common.Respond(w, r, http.StatusOK, session, "Current session retrieved successfully")
}

// getHistory handles GET /api/time-tracking/history?from=2026-01-01&to=2026-01-31
func (rs *Resource) getHistory(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	// Get history
	sessions, err := rs.WorkSessionService.GetHistory(r.Context(), staffID, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, sessions, "Session history retrieved successfully")
}

// updateSession handles PUT /api/time-tracking/{id}
func (rs *Resource) updateSession(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Parse session ID from URL
	idStr := chi.URLParam(r, "id")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidSessionID)))
		return
	}

	// Parse update request
	var updates activeSvc.SessionUpdateRequest
	if err := render.DecodeJSON(r.Body, &updates); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Call service to update session
	session, err := rs.WorkSessionService.UpdateSession(r.Context(), staffID, sessionID, updates)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Session updated successfully")
}

// StartBreakRequest represents a request to start a break
type StartBreakRequest struct {
	PlannedDurationMinutes *int `json:"planned_duration_minutes,omitempty"`
}

// startBreak handles POST /api/time-tracking/break/start
func (rs *Resource) startBreak(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Parse optional request body for planned_duration_minutes
	var req StartBreakRequest
	if r.ContentLength > 0 {
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}

	// Call service to start break
	brk, err := rs.WorkSessionService.StartBreak(r.Context(), staffID, req.PlannedDurationMinutes)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, brk, "Break started")
}

// endBreak handles POST /api/time-tracking/break/end
func (rs *Resource) endBreak(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Call service to end break
	session, err := rs.WorkSessionService.EndBreak(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Break ended")
}

// getBreaks handles GET /api/time-tracking/breaks/{sessionId}
func (rs *Resource) getBreaks(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims for ownership verification
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Parse session ID from URL
	idStr := chi.URLParam(r, "sessionId")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidSessionID)))
		return
	}

	// Get breaks (service verifies ownership)
	breaks, err := rs.WorkSessionService.GetSessionBreaks(r.Context(), staffID, sessionID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, breaks, "Breaks retrieved successfully")
}

// getSessionEdits handles GET /api/time-tracking/{id}/edits
func (rs *Resource) getSessionEdits(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims for ownership verification
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Parse session ID from URL
	idStr := chi.URLParam(r, "id")
	sessionID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidSessionID)))
		return
	}

	// Get edits (service verifies ownership)
	edits, err := rs.WorkSessionService.GetSessionEdits(r.Context(), staffID, sessionID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, edits, "Session edits retrieved successfully")
}

// exportSessions handles GET /api/time-tracking/export?from=...&to=...&format=csv|xlsx
func (rs *Resource) exportSessions(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	format := r.URL.Query().Get("format")
	if format != "csv" && format != "xlsx" {
		format = "csv"
	}

	fileBytes, filename, err := rs.WorkSessionService.ExportSessions(r.Context(), staffID, from, to, format)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Set response headers for file download
	switch format {
	case "xlsx":
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	default:
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(fileBytes)))

	if _, err := w.Write(fileBytes); err != nil {
		// Response already started, just log
		return
	}
}

// listAbsences handles GET /api/time-tracking/absences?from=&to=
func (rs *Resource) listAbsences(w http.ResponseWriter, r *http.Request) {
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	absences, err := rs.StaffAbsenceService.GetAbsencesForRange(r.Context(), staffID, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, absences, "Absences retrieved successfully")
}

// createAbsence handles POST /api/time-tracking/absences
func (rs *Resource) createAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	var req activeSvc.CreateAbsenceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	absence, err := rs.StaffAbsenceService.CreateAbsence(r.Context(), staffID, req)
	if err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, absence, "Absence created successfully")
}

// updateAbsence handles PUT /api/time-tracking/absences/{id}
func (rs *Resource) updateAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	idStr := chi.URLParam(r, "id")
	absenceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid absence ID")))
		return
	}

	var req activeSvc.UpdateAbsenceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	absence, err := rs.StaffAbsenceService.UpdateAbsence(r.Context(), staffID, absenceID, req)
	if err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, absence, "Absence updated successfully")
}

// deleteAbsence handles DELETE /api/time-tracking/absences/{id}
func (rs *Resource) deleteAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	idStr := chi.URLParam(r, "id")
	absenceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid absence ID")))
		return
	}

	if err := rs.StaffAbsenceService.DeleteAbsence(r.Context(), staffID, absenceID); err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getPresenceMap handles GET /api/time-tracking/presence-map
func (rs *Resource) getPresenceMap(w http.ResponseWriter, r *http.Request) {
	// Get today's presence map
	presenceMap, err := rs.WorkSessionService.GetTodayPresenceMap(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, presenceMap, "Presence map retrieved successfully")
}
