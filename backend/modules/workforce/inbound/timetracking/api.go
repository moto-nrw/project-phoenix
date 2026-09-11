package timetracking

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	staffshifts "github.com/moto-nrw/project-phoenix/api/staff-shifts"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Error message constants
const errInvalidSessionID = "invalid session ID"

// Resource defines the time-tracking API resource: the staff member's own
// stamps, history, absences and planning views under /api/time-tracking.
type Resource struct {
	WorkSessionService  workforce.WorkSessions
	StaffAbsenceService workforce.StaffAbsences
	PersonService       workforce.StaffDirectory
	StaffShiftService   scheduleSvc.StaffShiftService
	// StaffAssignmentService backs GET /assignments — the staff member's own
	// Betreuungsplan blocks (Ort/Aufgabe) for a day (#1844).
	StaffAssignmentService scheduleSvc.StaffAssignmentService
	// WorkTimeMonthService backs GET /month-summary — the Monatskarte (#1842).
	WorkTimeMonthService workforce.WorkTimeMonths
	// Calendar backs GET /holidays and GET /closing-days — the tenant's public
	// holidays (#1418 3a) and closure periods (#1418 3b) for calendar marking.
	Calendar workforce.PlanningCalendar
	// accountStartDate resolves the tenant's time-account start day ("" when
	// unset) for GET /config; nil means the setting is not wired.
	accountStartDate func(context.Context) (string, error)
	identity         IdentityFunc
	db               *bun.DB
}

// Dependencies are the collaborators of the time-tracking resource.
type Dependencies struct {
	WorkSessions  workforce.WorkSessions
	StaffAbsences workforce.StaffAbsences
	Staff         workforce.StaffDirectory
	WorkTimeMonth workforce.WorkTimeMonths
	StaffShifts   scheduleSvc.StaffShiftService
	Assignments   scheduleSvc.StaffAssignmentService
	Calendar      workforce.PlanningCalendar
	// AccountStartDate resolves the tenant's time-account start day; nil
	// renders an empty start date.
	AccountStartDate func(context.Context) (string, error)
	Identity         IdentityFunc
	DB               *bun.DB
}

// NewResource creates a new time-tracking resource.
func NewResource(deps Dependencies) *Resource {
	if deps.WorkSessions == nil || deps.StaffAbsences == nil || deps.Staff == nil || deps.Identity == nil || deps.DB == nil {
		panic("time tracking resource: work sessions, absences, staff directory, identity and db are required")
	}
	return &Resource{
		WorkSessionService:     deps.WorkSessions,
		StaffAbsenceService:    deps.StaffAbsences,
		PersonService:          deps.Staff,
		StaffShiftService:      deps.StaffShifts,
		StaffAssignmentService: deps.Assignments,
		WorkTimeMonthService:   deps.WorkTimeMonth,
		Calendar:               deps.Calendar,
		accountStartDate:       deps.AccountStartDate,
		identity:               deps.Identity,
		db:                     deps.DB,
	}
}

// Router returns a configured router for time-tracking endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// All time-tracking endpoints require TimeTrackingOwn permission
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/check-in", rs.checkIn)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/check-out", rs.checkOut)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/current", rs.getCurrent)
		// The Stundenkonto anchor is a tenant-wide setting, not caller-scoped
		// data. Admin views resolve a staff member's Monatskarte from it, so a
		// manage-only role must be able to read it — the staff summary
		// endpoints it pairs with gate on TimeTrackingManage alone.
		r.With(common.RequiresAnyPermission(permissions.TimeTrackingOwn, permissions.TimeTrackingManage), withTx).Get("/config", rs.getConfig)
		r.With(common.RequiresAnyPermission(permissions.TimeTrackingOwn, permissions.TimeTrackingManage), withTx).Get("/holidays", rs.getHolidays)
		r.With(common.RequiresAnyPermission(permissions.TimeTrackingOwn, permissions.TimeTrackingManage), withTx).Get("/closing-days", rs.getClosingDays)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/history", rs.getHistory)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Put("/{id}", rs.updateSession)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/{id}/edits", rs.getSessionEdits)

		// Break management
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/break/start", rs.startBreak)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/break/end", rs.endBreak)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/breaks/{sessionId}", rs.getBreaks)

		// Export
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/export", rs.exportSessions)

		// Own planned shifts (Dienstplan, #1376/#1798)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/shifts", rs.getOwnShifts)
		// Own Monatskarte (#1842)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/month-summary", rs.getOwnMonthSummary)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/schedule-targets", rs.getOwnScheduleTargets)
		// Own Betreuungsplan blocks for the day (Ort/Aufgabe + Vertretungen, #1844)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/assignments", rs.getOwnAssignments)

		// Absence management
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/absences", rs.listAbsences)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/absences", rs.createAbsence)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Put("/absences/{id}", rs.updateAbsence)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Delete("/absences/{id}", rs.deleteAbsence)

		// Vacation workflow (Tranche 4), MA-side
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/vacation/request", rs.requestVacation)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/absences/{id}/cancel", rs.cancelAbsence)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Post("/absences/{id}/resubmit", rs.resubmitAbsence)
		r.With(common.RequiresPermission(permissions.TimeTrackingOwn), withTx).Get("/vacation/quota", rs.getOwnVacationQuota)

		// Presence map - for internal use by staff page
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/presence-map", rs.getPresenceMap)
	})

	return r
}

// CheckInRequest represents a check-in request. Reason is the optional F9
// deviation reason; it is only required when the backend answered a previous
// attempt with the "deviation_reason_required" conflict.
type CheckInRequest struct {
	Status string `json:"status"` // "present" or "home_office"
	Reason string `json:"reason"`
}

// CheckOutRequest is the optional check-out body. Existing clients send no
// body at all; the handler treats an empty body as an empty request.
type CheckOutRequest struct {
	Reason string `json:"reason"`
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
func (rs *Resource) getStaffIDFromClaims(ctx context.Context, claims Identity) (int64, error) {
	if claims.AccountID == 0 {
		return 0, errors.New("invalid token")
	}

	person, err := rs.PersonService.PersonByAccountID(ctx, claims.AccountID)
	if err != nil {
		return 0, errors.New("person not found for account")
	}

	staff, err := rs.PersonService.StaffByPersonID(ctx, person.ID)
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
		return timezone.Date(""), timezone.Date(""), false
	}

	from, err := timezone.ParseDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return timezone.Date(""), timezone.Date(""), false
	}

	to, err = timezone.ParseDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return timezone.Date(""), timezone.Date(""), false
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
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Call service to check in
	tenantID := tenant.FromContext(r.Context())
	var session *workforce.WorkSession
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		session, txErr = rs.WorkSessionService.CheckIn(ctx, staffID, req.Status, workforce.WorkSessionSourceApp, req.Reason)
		return txErr
	}); err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, workforce.WorkSessionWire{WorkSession: session}, "Check-in successful")
}

// checkOut handles POST /api/time-tracking/check-out
func (rs *Resource) checkOut(w http.ResponseWriter, r *http.Request) {
	// The body is optional (legacy clients POST without one); an empty body
	// is an empty request, anything else must be valid JSON.
	req := &CheckOutRequest{}
	if err := render.DecodeJSON(r.Body, req); err != nil && !errors.Is(err, io.EOF) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get staff ID from JWT claims
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Call service to check out
	tenantID := tenant.FromContext(r.Context())
	var session *workforce.WorkSession
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		session, txErr = rs.WorkSessionService.CheckOut(ctx, staffID, req.Reason)
		return txErr
	}); err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, workforce.WorkSessionWire{WorkSession: session}, "Check-out successful")
}

// getCurrent handles GET /api/time-tracking/current
func (rs *Resource) getCurrent(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// The running block, whichever day it was opened on. Filtering on today
	// would hide a block that crossed Berlin midnight: the page would offer
	// "Einstempeln", the check-in would refuse it as already checked in, and
	// no button would close the block that is actually running.
	session, err := rs.WorkSessionService.LatestOpenSession(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return null if no active session (not an error)
	common.Respond(w, r, http.StatusOK, workforce.WorkSessionWire{WorkSession: session}, "Current session retrieved successfully")
}

// getConfig handles GET /api/time-tracking/config
func (rs *Resource) getConfig(w http.ResponseWriter, r *http.Request) {
	accountStartDate := ""
	if rs.accountStartDate != nil {
		value, err := rs.accountStartDate(r.Context())
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		accountStartDate = value
	}

	common.Respond(w, r, http.StatusOK, ConfigResponse{
		AccountStartDate: accountStartDate,
	}, "Time tracking config retrieved successfully")
}

// getHistory handles GET /api/time-tracking/history?from=2026-01-01&to=2026-01-31
func (rs *Resource) getHistory(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	// Get history with weekly aggregation. Intersecting for the same reason as
	// the admin-side history (api/staff): a night block that began before
	// `from` still carries minutes inside the range (#2402).
	historyResp, err := rs.WorkSessionService.HistoryIntersecting(r.Context(), staffID, from.String(), to.String())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, historyResp, "Session history retrieved successfully")
}

// updateSession handles PUT /api/time-tracking/{id}
func (rs *Resource) updateSession(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	sessionID, ok := common.ParseInt64IDWithError(w, r, "id", errInvalidSessionID)
	if !ok {
		return
	}

	// Parse update request
	var updates workforce.SessionUpdateRequest
	if err := render.DecodeJSON(r.Body, &updates); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Call service to update session
	tenantID := tenant.FromContext(r.Context())
	var session *workforce.WorkSession
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		session, txErr = rs.WorkSessionService.UpdateSession(ctx, staffID, sessionID, updates)
		return txErr
	}); err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, workforce.WorkSessionWire{WorkSession: session}, "Session updated successfully")
}

// StartBreakRequest represents a request to start a break
type StartBreakRequest struct {
	PlannedDurationMinutes *int `json:"planned_duration_minutes,omitempty"`
}

// startBreak handles POST /api/time-tracking/break/start
func (rs *Resource) startBreak(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := rs.identity(r.Context())
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
	var brk *workforce.WorkSessionBreak
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
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	// Call service to end break
	tenantID := tenant.FromContext(r.Context())
	var session *workforce.WorkSession
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		session, txErr = rs.WorkSessionService.EndBreak(ctx, staffID)
		return txErr
	}); err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, workforce.WorkSessionWire{WorkSession: session}, "Break ended")
}

// getBreaks handles GET /api/time-tracking/breaks/{sessionId}
func (rs *Resource) getBreaks(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims for ownership verification
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	sessionID, ok := common.ParseInt64IDWithError(w, r, "sessionId", errInvalidSessionID)
	if !ok {
		return
	}

	// Get breaks (service verifies ownership)
	breaks, err := rs.WorkSessionService.SessionBreaks(r.Context(), staffID, sessionID)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, breaks, "Breaks retrieved successfully")
}

// getSessionEdits handles GET /api/time-tracking/{id}/edits
func (rs *Resource) getSessionEdits(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims for ownership verification
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	sessionID, ok := common.ParseInt64IDWithError(w, r, "id", errInvalidSessionID)
	if !ok {
		return
	}

	// Get edits (service verifies ownership)
	edits, err := rs.WorkSessionService.SessionEdits(r.Context(), staffID, sessionID)
	if err != nil {
		common.RenderError(w, r, classifyServiceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, edits, "Session edits retrieved successfully")
}

// exportSessions handles GET /api/time-tracking/export?from=...&to=...&format=csv|xlsx
func (rs *Resource) exportSessions(w http.ResponseWriter, r *http.Request) {
	// Get staff ID from JWT claims
	userClaims := rs.identity(r.Context())
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
	if format != "csv" && format != "xlsx" && format != "pdf" {
		format = "csv"
	}

	file, err := rs.WorkSessionService.ExportSessions(r.Context(), staffID, from.String(), to.String(), format)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Set response headers for file download
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))

	if _, err := w.Write(file.Data); err != nil {
		// Response already started, just log the error
		slog.Default().Error("failed to write export response", slog.String("error", err.Error()))
		return
	}
}

// listAbsences handles GET /api/time-tracking/absences with an overlapping
// date range, a status filter, or both.
func (rs *Resource) listAbsences(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	query := r.URL.Query()
	fromStr := query.Get("from")
	toStr := query.Get("to")
	status := strings.TrimSpace(query.Get("status"))
	if (fromStr == "") != (toStr == "") {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters must be provided together")))
		return
	}

	filter := workforce.StaffAbsenceListFilter{Status: status}
	if fromStr != "" {
		from, to, ok := parseDateRange(w, r)
		if !ok {
			return
		}
		filter.From = from.String()
		filter.To = to.String()
	} else if status == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters or status are required")))
		return
	}

	absences, err := rs.StaffAbsenceService.ListAbsences(r.Context(), staffID, filter)
	if err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, absences, "Absences retrieved successfully")
}

// createAbsence handles POST /api/time-tracking/absences
func (rs *Resource) createAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	var req workforce.CreateAbsenceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Self-service passes the caller as subject AND creator; the account id
	// makes the #1843 sick-cascade protocol entries carry a real actor.
	actorAccountID := userClaims.AccountID
	tenantID := tenant.FromContext(r.Context())
	var absence *workforce.StaffAbsenceResponse
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		absence, txErr = rs.StaffAbsenceService.CreateOwnAbsence(ctx, staffID, &actorAccountID, req)
		return txErr
	}); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, absence, "Absence created successfully")
}

// updateAbsence handles PUT /api/time-tracking/absences/{id}
func (rs *Resource) updateAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	absenceID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid absence ID")
	if !ok {
		return
	}

	var req workforce.UpdateAbsenceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	actorAccountID := userClaims.AccountID
	var absence *workforce.StaffAbsenceResponse
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		absence, txErr = rs.StaffAbsenceService.UpdateAbsence(ctx, staffID, &actorAccountID, absenceID, req)
		return txErr
	}); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, absence, "Absence updated successfully")
}

// deleteAbsence handles DELETE /api/time-tracking/absences/{id}
func (rs *Resource) deleteAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	absenceID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid absence ID")
	if !ok {
		return
	}

	// Deleting one's own sick report reverses its plan cascade (#1843); the
	// account id stamps the sick_cleared protocol entries.
	actorAccountID := userClaims.AccountID
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.StaffAbsenceService.DeleteOwnAbsence(ctx, staffID, &actorAccountID, absenceID)
	}); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}

	common.RespondNoContent(w, r)
}

// requestVacation handles POST /api/time-tracking/vacation/request
func (rs *Resource) requestVacation(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	var req workforce.RequestVacationRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	var resp *workforce.StaffAbsenceResponse
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
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	absenceID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid absence ID")
	if !ok {
		return
	}
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.StaffAbsenceService.CancelAbsence(ctx, staffID, userClaims.AccountID, absenceID)
	}); err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}
	common.RespondNoContent(w, r)
}

// resubmitAbsence handles POST /api/time-tracking/absences/{id}/resubmit —
// the MA answers a Rückfrage by amending their note and re-requesting (#1419).
func (rs *Resource) resubmitAbsence(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	absenceID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid absence ID")
	if !ok {
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	tenantID := tenant.FromContext(r.Context())
	var resp *workforce.StaffAbsenceResponse
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		var txErr error
		resp, txErr = rs.StaffAbsenceService.ResubmitAbsence(ctx, staffID, userClaims.AccountID, absenceID, req.Note)
		return txErr
	}); err != nil {
		common.RenderError(w, r, classifyAbsenceError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Absence resubmitted")
}

// getOwnVacationQuota handles GET /api/time-tracking/vacation/quota?year=YYYY
func (rs *Resource) getOwnVacationQuota(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
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
	summary, err := rs.StaffAbsenceService.VacationQuotaSummary(r.Context(), staffID, year)
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
	presenceMap, err := rs.WorkSessionService.TodayPresenceMap(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, presenceMap, "Presence map retrieved successfully")
}

// getOwnShifts handles GET /api/time-tracking/shifts — the staff member's
// own planned shifts (Dienstplan) in a from/to date range.
func (rs *Resource) getOwnShifts(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
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

	common.Respond(w, r, http.StatusOK, staffshifts.ToShiftResponses(plannedShifts(shifts)), "Shifts retrieved successfully")
}

// plannedShifts maps the retained shift rows onto the Workforce planning
// shape the shared wire format is rendered from (#2689).
func plannedShifts(shifts []*scheduleModels.StaffShift) []workforce.PlannedShift {
	result := make([]workforce.PlannedShift, 0, len(shifts))
	for _, shift := range shifts {
		if shift == nil {
			continue
		}
		planned := workforce.PlannedShift{StaffShift: workforce.StaffShift{
			ID: shift.ID, TenantID: shift.TenantID, StaffID: shift.StaffID, Date: shift.Date.String(),
			StartTime:    timezone.NormalizeWallClock(shift.StartTime).Format(workforce.ClockLayout),
			EndTime:      timezone.NormalizeWallClock(shift.EndTime).Format(workforce.ClockLayout),
			BreakMinutes: shift.BreakMinutes, ShiftTypeID: shift.ShiftTypeID, Notes: shift.Notes,
			SeriesID: shift.SeriesID, Detached: shift.Detached, Cancelled: shift.Cancelled,
			ChangeReason: shift.ChangeReason, OriginShiftID: shift.OriginShiftID, SickAbsenceID: shift.SickAbsenceID,
			CreatedBy: shift.CreatedBy, UpdatedBy: shift.UpdatedBy, CreatedAt: shift.CreatedAt, UpdatedAt: shift.UpdatedAt,
		}}
		if shift.SeriesOccurrenceDate != nil {
			planned.SeriesOccurrenceDate = shift.SeriesOccurrenceDate.String()
		}
		if shift.ShiftType != nil {
			planned.ShiftType = &workforce.ShiftTypeLabel{Name: shift.ShiftType.Name, Color: shift.ShiftType.Color}
		}
		result = append(result, planned)
	}
	return result
}

// assignmentResponse is the wire format for one Betreuungsplan block a staff
// member is planned into (#1844). It deliberately carries no student data
// (GDPR: no child names in time-tracking lists).
type assignmentResponse struct {
	InstanceID      int64   `json:"instance_id"`
	Title           string  `json:"title"`
	GroupName       *string `json:"group_name,omitempty"`
	RoomName        string  `json:"room_name,omitempty"`
	Date            string  `json:"date"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	Status          string  `json:"status"`
	Cancelled       bool    `json:"cancelled"`
	IsPrimary       bool    `json:"is_primary"`
	IsSubstitute    bool    `json:"is_substitute"`
	IsAbsent        bool    `json:"is_absent"`
	AbsenceReason   *string `json:"absence_reason,omitempty"`
	CancelReason    *string `json:"cancel_reason,omitempty"`
	UnderstaffedAck bool    `json:"understaffed_ack"`
}

func toAssignmentResponses(assignments []*scheduleSvc.StaffAssignment) []assignmentResponse {
	out := make([]assignmentResponse, 0, len(assignments))
	for _, a := range assignments {
		out = append(out, assignmentResponse{
			InstanceID:      a.InstanceID,
			Title:           a.Title,
			GroupName:       a.GroupName,
			RoomName:        a.RoomName,
			Date:            a.Date.String(),
			StartTime:       timezone.NormalizeWallClock(a.StartTime).Format("15:04"),
			EndTime:         timezone.NormalizeWallClock(a.EndTime).Format("15:04"),
			Status:          a.Status,
			Cancelled:       a.Cancelled,
			IsPrimary:       a.IsPrimary,
			IsSubstitute:    a.IsSubstitute,
			IsAbsent:        a.IsAbsent,
			AbsenceReason:   a.AbsenceReason,
			CancelReason:    a.CancelReason,
			UnderstaffedAck: a.UnderstaffedAck,
		})
	}
	return out
}

// getOwnAssignments handles GET /api/time-tracking/assignments — the staff
// member's own Betreuungsplan blocks (room + activity + Vertretungsplan state)
// in a from/to date range. Self-scoped: the staff id comes from the caller's
// JWT, never a parameter, so no one reads another person's plan (#1844).
func (rs *Resource) getOwnAssignments(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}

	assignments, err := rs.StaffAssignmentService.ListAssignmentsForStaff(r.Context(), staffID, from, to)
	if err != nil {
		if errors.Is(err, scheduleSvc.ErrShiftInvalid) || errors.Is(err, scheduleSvc.ErrShiftRangeTooLarge) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, toAssignmentResponses(assignments), "Assignments retrieved successfully")
}
