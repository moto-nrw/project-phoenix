package timetracking

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	staffshifts "github.com/moto-nrw/project-phoenix/api/staff-shifts"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Error message constants
const errInvalidSessionID = "invalid session ID"

// Resource defines the time-tracking API resource
type Resource struct {
	WorkSessionService  activeSvc.WorkSessionService
	StaffAbsenceService activeSvc.StaffAbsenceService
	PersonService       usersSvc.PersonService
	SettingsService     configSvc.SettingsService
	StaffShiftService   scheduleSvc.StaffShiftService
	db                  *bun.DB
}

// NewResource creates a new time-tracking resource
func NewResource(workSessionService activeSvc.WorkSessionService, staffAbsenceService activeSvc.StaffAbsenceService, personService usersSvc.PersonService, settingsService configSvc.SettingsService, staffShiftService scheduleSvc.StaffShiftService, db *bun.DB) *Resource {
	return &Resource{
		WorkSessionService:  workSessionService,
		StaffAbsenceService: staffAbsenceService,
		PersonService:       personService,
		SettingsService:     settingsService,
		StaffShiftService:   staffShiftService,
		db:                  db,
	}
}

// Router returns a configured router for time-tracking endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		// All time-tracking endpoints require TimeTrackingOwn permission
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/check-in", rs.checkIn)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/check-out", rs.checkOut)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/current", rs.getCurrent)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/config", rs.getConfig)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/history", rs.getHistory)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Put("/{id}", rs.updateSession)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/{id}/edits", rs.getSessionEdits)

		// Break management
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/break/start", rs.startBreak)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/break/end", rs.endBreak)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/breaks/{sessionId}", rs.getBreaks)

		// Export
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/export", rs.exportSessions)

		// Own planned shifts (Dienstplan, #1376/#1798)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/shifts", rs.getOwnShifts)

		// Absence management
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/absences", rs.listAbsences)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/absences", rs.createAbsence)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Put("/absences/{id}", rs.updateAbsence)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Delete("/absences/{id}", rs.deleteAbsence)

		// Vacation workflow (Tranche 4), MA-side
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/vacation/request", rs.requestVacation)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/absences/{id}/cancel", rs.cancelAbsence)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/vacation/quota", rs.getOwnVacationQuota)

		// Presence map - for internal use by staff page
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/presence-map", rs.getPresenceMap)
	})

	return r
}

// CheckInRequest represents a check-in request
type CheckInRequest struct {
	Status string `json:"status"` // "present" or "home_office"
}

type ConfigResponse struct {
	AccountStartDate string `json:"account_start_date"`
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

	staff, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		return 0, errors.New("staff record not found")
	}

	return staff.ID, nil
}

// parseDateRange extracts and validates "from" and "to" query parameters as dates.
// Returns the parsed times or renders an error and returns false.
func parseDateRange(w http.ResponseWriter, r *http.Request) (from, to timezone.Date, ok bool) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return timezone.Date{}, timezone.Date{}, false
	}

	from, err := timezone.ParseDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, false
	}

	to, err = timezone.ParseDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, false
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
	tenantID := tenant.FromContext(r.Context())
	var session *activeModels.WorkSession
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		session, txErr = rs.WorkSessionService.CheckIn(ctx, staffID, req.Status, activeModels.WorkSessionSourceApp)
		return txErr
	}); err != nil {
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
	tenantID := tenant.FromContext(r.Context())
	var session *activeModels.WorkSession
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		session, txErr = rs.WorkSessionService.CheckOut(ctx, staffID)
		return txErr
	}); err != nil {
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

// getConfig handles GET /api/time-tracking/config
func (rs *Resource) getConfig(w http.ResponseWriter, r *http.Request) {
	accountStartDate := ""
	if rs.SettingsService != nil {
		value, err := rs.SettingsService.ResolveString(r.Context(), configModels.KeyTimeTrackingAccountStartDate)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		accountStartDate = value
	}

	common.Respond(w, r, http.StatusOK, ConfigResponse{AccountStartDate: accountStartDate}, "Time tracking config retrieved successfully")
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

	// Get history with weekly aggregation
	historyResp, err := rs.WorkSessionService.GetHistory(r.Context(), staffID, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, historyResp, "Session history retrieved successfully")
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
	tenantID := tenant.FromContext(r.Context())
	var session *activeModels.WorkSession
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		session, txErr = rs.WorkSessionService.UpdateSession(ctx, staffID, sessionID, updates)
		return txErr
	}); err != nil {
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
	tenantID := tenant.FromContext(r.Context())
	var brk *activeModels.WorkSessionBreak
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		brk, txErr = rs.WorkSessionService.StartBreak(ctx, staffID, req.PlannedDurationMinutes)
		return txErr
	}); err != nil {
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
	tenantID := tenant.FromContext(r.Context())
	var session *activeModels.WorkSession
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		session, txErr = rs.WorkSessionService.EndBreak(ctx, staffID)
		return txErr
	}); err != nil {
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
		common.RenderError(w, r, classifyServiceError(err))
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
		common.RenderError(w, r, classifyServiceError(err))
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
		// Response already started, just log the error
		slog.Default().Error("failed to write export response", slog.String("error", err.Error()))
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

	tenantID := tenant.FromContext(r.Context())
	var absence *activeSvc.StaffAbsenceResponse
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		absence, txErr = rs.StaffAbsenceService.CreateAbsence(ctx, staffID, req)
		return txErr
	}); err != nil {
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

	tenantID := tenant.FromContext(r.Context())
	var absence *activeSvc.StaffAbsenceResponse
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		absence, txErr = rs.StaffAbsenceService.UpdateAbsence(ctx, staffID, absenceID, req)
		return txErr
	}); err != nil {
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

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.StaffAbsenceService.DeleteAbsence(ctx, staffID, absenceID)
	}); err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.RespondNoContent(w, r)
}

// requestVacation handles POST /api/time-tracking/vacation/request
func (rs *Resource) requestVacation(w http.ResponseWriter, r *http.Request) {
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	var req activeSvc.RequestVacationRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	var resp *activeSvc.StaffAbsenceResponse
	err = tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		resp, txErr = rs.StaffAbsenceService.RequestVacation(ctx, staffID, req)
		return txErr
	})
	if err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, resp, "Vacation request created")
}

// cancelAbsence handles POST /api/time-tracking/absences/{id}/cancel
func (rs *Resource) cancelAbsence(w http.ResponseWriter, r *http.Request) {
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
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.StaffAbsenceService.CancelAbsence(ctx, staffID, int64(userClaims.ID), absenceID)
	}); err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}
	common.RespondNoContent(w, r)
}

// getOwnVacationQuota handles GET /api/time-tracking/vacation/quota?year=YYYY
func (rs *Resource) getOwnVacationQuota(w http.ResponseWriter, r *http.Request) {
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	year, err := parseYearParam(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	summary, err := rs.StaffAbsenceService.GetVacationQuotaSummary(r.Context(), staffID, year)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, summary, "Vacation quota retrieved")
}

func parseYearParam(r *http.Request) (int, error) {
	yearStr := r.URL.Query().Get("year")
	if yearStr == "" {
		return time.Now().Year(), nil
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		return 0, errors.New("invalid year parameter, expected 2000-2100")
	}
	return year, nil
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

// getOwnShifts handles GET /api/time-tracking/shifts — the staff member's
// own planned shifts (Dienstplan) in a from/to date range.
func (rs *Resource) getOwnShifts(w http.ResponseWriter, r *http.Request) {
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

	shifts, err := rs.StaffShiftService.ListShiftsForStaff(r.Context(), staffID, from, to)
	if err != nil {
		if errors.Is(err, scheduleSvc.ErrShiftInvalid) || errors.Is(err, scheduleSvc.ErrShiftRangeTooLarge) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, staffshifts.ToShiftResponses(shifts), "Shifts retrieved successfully")
}
