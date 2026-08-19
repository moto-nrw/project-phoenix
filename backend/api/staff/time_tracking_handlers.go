package staff

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// getStaffHistory handles GET /api/staff/{id}/time-tracking/history?from=YYYY-MM-DD&to=YYYY-MM-DD
func (rs *Resource) getStaffHistory(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Verify staff exists
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	// Parse date range from query parameters
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return
	}

	from, err := timezone.ParseDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}

	to, err := timezone.ParseDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}

	historyResp, err := rs.WorkSessionService.GetHistory(r.Context(), staffID, from, to)
	if err != nil {
		rs.getLogger().Error("failed to get staff history",
			"staff_id", staffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, historyResp, "Staff session history retrieved successfully")
}

// exportStaffSessions handles GET /api/staff/{id}/time-tracking/export?from=...&to=...&format=csv|xlsx|pdf
func (rs *Resource) exportStaffSessions(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return
	}
	from, err := timezone.ParseDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}
	to, err := timezone.ParseDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}

	format := r.URL.Query().Get("format")
	if format != "csv" && format != "xlsx" && format != "pdf" {
		format = "csv"
	}

	file, err := rs.WorkSessionService.ExportSessions(r.Context(), staffID, from, to, format)
	if err != nil {
		rs.getLogger().Error("failed to export staff sessions",
			"staff_id", staffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	if _, err := w.Write(file.Data); err != nil {
		rs.getLogger().Error("failed to write export response", "error", err.Error())
	}
}

// listPendingAbsenceRequests handles GET /api/staff/absences/pending
func (rs *Resource) listPendingAbsenceRequests(w http.ResponseWriter, r *http.Request) {
	resp, err := rs.StaffAbsenceService.ListPendingRequests(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Pending absence requests retrieved")
}

// approveAbsence handles POST /api/staff/absences/{absenceId}/approve
func (rs *Resource) approveAbsence(w http.ResponseWriter, r *http.Request) {
	absenceID, err := parseInt64Param(r, "absenceId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	decidedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	var req activeSvc.VacationDecisionRequest
	if r.ContentLength > 0 {
		if err := render.DecodeJSON(r.Body, &req); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}
	resp, err := rs.StaffAbsenceService.ApproveAbsence(r.Context(), absenceID, int64(claims.ID), decidedBy, req.DecisionNote)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Absence approved")
}

// denyAbsence handles POST /api/staff/absences/{absenceId}/deny
func (rs *Resource) denyAbsence(w http.ResponseWriter, r *http.Request) {
	absenceID, err := parseInt64Param(r, "absenceId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	decidedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	var req activeSvc.VacationDecisionRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	resp, err := rs.StaffAbsenceService.DenyAbsence(r.Context(), absenceID, int64(claims.ID), decidedBy, req.DecisionNote)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Absence declined")
}

// questionAbsenceErrorRules classifies QuestionAbsence service errors (#1419).
var questionAbsenceErrorRules = []common.ErrorRule{
	// School-defined Abwesenheitsarten (#2403): a retired or unknown art is a
	// bad selection, not a server fault.
	{Target: activeSvc.ErrAbsenceTypeInactive, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "absence_type_inactive")
	}},
	{Target: activeSvc.ErrAbsenceTypeNotFound, Render: common.ErrorInvalidRequest},
	{Match: absenceMsgIs("absence not found"), Render: common.ErrorNotFound},
	{Match: absenceMsgIs("question note is required"), Render: common.ErrorInvalidRequest},
	{Match: absenceMsgIs("only requested absences can be questioned"), Render: common.ErrorInvalidRequest},
}

// questionAbsence handles POST /api/staff/absences/{absenceId}/question —
// the Leitung's Rückfrage with a mandatory note (#1419 4d).
func (rs *Resource) questionAbsence(w http.ResponseWriter, r *http.Request) {
	absenceID, err := parseInt64Param(r, "absenceId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	var req activeSvc.VacationDecisionRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	resp, err := rs.StaffAbsenceService.QuestionAbsence(r.Context(), absenceID, int64(claims.ID), req.DecisionNote)
	if err != nil {
		common.RenderError(w, r, common.RenderWithRules(err, questionAbsenceErrorRules, common.ErrorInternalServer))
		return
	}
	common.Respond(w, r, http.StatusOK, resp, "Absence question sent")
}

// getStaffVacationQuota handles GET /api/staff/{id}/vacation/quota?year=YYYY
func (rs *Resource) getStaffVacationQuota(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	year, err := parseYearQuery(r)
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

// setStaffVacationQuota handles PUT /api/staff/{id}/vacation/quota
func (rs *Resource) setStaffVacationQuota(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var body struct {
		Year          int     `json:"year"`
		EntitledDays  float64 `json:"entitled_days"`
		CarryoverDays float64 `json:"carryover_days"`
	}
	if err := render.DecodeJSON(r.Body, &body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if body.Year == 0 {
		body.Year = timezone.TodayDate().Year
	}
	if err := rs.StaffAbsenceService.UpsertVacationQuota(r.Context(), staffID, body.Year, body.EntitledDays, body.CarryoverDays); err != nil {
		if errors.Is(err, activeSvc.ErrVacationQuotaInvalid) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
		} else {
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	summary, err := rs.StaffAbsenceService.GetVacationQuotaSummary(r.Context(), staffID, body.Year)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, summary, "Vacation quota saved")
}

func parseInt64Param(r *http.Request, name string) (int64, error) {
	v, err := common.ParseIDParam(r, name)
	if err != nil {
		return 0, errors.New("invalid " + name + " parameter")
	}
	return v, nil
}

func parseYearQuery(r *http.Request) (int, error) {
	yearStr := r.URL.Query().Get("year")
	if yearStr == "" {
		return timezone.TodayDate().Year, nil
	}
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		return 0, errors.New("invalid year parameter")
	}
	return year, nil
}

// adminUpdateStaffSession handles PUT /api/staff/{id}/time-tracking/sessions/{sessionId}
// Admin counterpart to /api/time-tracking/{id}, gated on
// time_tracking:manage at the router level, so a Betreuer with only
// time_tracking:own gets 403 before reaching the handler.
func (rs *Resource) adminUpdateStaffSession(w http.ResponseWriter, r *http.Request) {
	targetStaffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	editorStaffID, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid session ID")))
		return
	}

	var updates activeSvc.SessionUpdateRequest
	if err := render.DecodeJSON(r.Body, &updates); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	session, err := rs.WorkSessionService.UpdateSessionAsAdmin(r.Context(), editorStaffID, targetStaffID, sessionID, updates)
	if err != nil {
		rs.getLogger().Error("admin session update failed",
			"target_staff_id", targetStaffID,
			"editor_staff_id", editorStaffID,
			"session_id", sessionID,
			"error", err.Error(),
		)
		common.RenderError(w, r, classifyAdminEditError(err))
		return
	}

	common.Respond(w, r, http.StatusOK, session, "Session updated")
}

// adminCreateStaffSession handles POST /api/staff/{id}/time-tracking/sessions
// the "nachtragen" flow when an MA forgot to stamp.
func (rs *Resource) adminCreateStaffSession(w http.ResponseWriter, r *http.Request) {
	targetStaffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	editorStaffID, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	var req activeSvc.AdminCreateSessionRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	session, err := rs.WorkSessionService.CreateSessionAsAdmin(r.Context(), editorStaffID, targetStaffID, req)
	if err != nil {
		rs.getLogger().Error("admin session create failed",
			"target_staff_id", targetStaffID,
			"editor_staff_id", editorStaffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, classifyAdminEditError(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, session, "Session created")
}

// classifyAdminEditError maps service errors to HTTP responses. Validation
// problems (notes missing, range invalid) land as 400; ownership mismatches
// and "not found" as 404 to avoid leaking session existence.
func classifyAdminEditError(err error) render.Renderer {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "notes required"),
		strings.Contains(msg, "must be"),
		strings.Contains(msg, "must not"),
		strings.Contains(msg, "is required"):
		return common.ErrorInvalidRequest(err)
	case strings.Contains(msg, "session not found"),
		strings.Contains(msg, "does not belong"):
		return common.ErrorNotFound(err)
	default:
		return common.ErrorInternalServer(err)
	}
}

// getStaffSessionEdits handles GET /api/staff/{id}/time-tracking/sessions/{sessionId}/edits
// Admin-facing counterpart to /api/time-tracking/{id}/edits. The MA endpoint
// verifies the session belongs to the JWT subject; here we verify it belongs
// to the staff named in the URL (RLS still scopes the tenant).
func (rs *Resource) getStaffSessionEdits(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	sessionIDStr := chi.URLParam(r, "sessionId")
	sessionID, err := strconv.ParseInt(sessionIDStr, 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid session ID")))
		return
	}

	edits, err := rs.WorkSessionService.GetSessionEditsForStaff(r.Context(), staffID, sessionID)
	if err != nil {
		rs.getLogger().Error("failed to get staff session edits",
			"staff_id", staffID,
			"session_id", sessionID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, edits, "Session edits retrieved successfully")
}

// getStaffAbsences handles GET /api/staff/{id}/absences?from=YYYY-MM-DD&to=YYYY-MM-DD
// Admin-facing counterpart to /api/time-tracking/absences (which is scoped to
// the caller). Lets the staff-detail view render Krank/Urlaub badges for any
// staff in the tenant.
func (rs *Resource) getStaffAbsences(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return
	}

	from, err := timezone.ParseDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}

	to, err := timezone.ParseDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}

	absences, err := rs.StaffAbsenceService.GetAbsencesForRange(r.Context(), staffID, from, to)
	if err != nil {
		rs.getLogger().Error("failed to get staff absences",
			"staff_id", staffID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, absences, "Staff absences retrieved successfully")
}

// adminAbsenceErrorRules classifies StaffAbsenceService errors for the admin
// absence endpoints (#1843). The absence service raises message-shaped errors
// (no sentinels), so most rules match on the message; the sick cascade wraps
// schedule sentinels, matched via errors.Is.
var adminAbsenceErrorRules = []common.ErrorRule{
	// 409 with a stable code so the frontend can show the overdraft message
	// (#1420 review) — mirrors balance_adjustment_exceeds_balance.
	{Target: activeSvc.ErrCompTimeExceedsBalance, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "comp_time_exceeds_balance")
	}},
	// School-defined Abwesenheitsarten (#2403): a retired or unknown art is a
	// bad selection, not a server fault.
	{Target: activeSvc.ErrAbsenceTypeInactive, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "absence_type_inactive")
	}},
	{Target: activeSvc.ErrAbsenceTypeNotFound, Render: common.ErrorInvalidRequest},
	{Match: absenceMsgIs("absence not found"), Render: common.ErrorNotFound},
	{Match: absenceMsgIs("can only delete own absences"), Render: common.ErrorForbidden},
	{Match: absenceMsgPrefix("absence overlaps"), Render: common.ErrorConflict},
	{Target: scheduleSvc.ErrShiftOverlap, Render: common.ErrorConflict},
	{Match: absenceMsgPrefix("invalid"), Render: common.ErrorInvalidRequest},
	{Match: absenceMsgPrefix("vacation"), Render: common.ErrorInvalidRequest},
}

func absenceMsgIs(msg string) func(error) bool {
	return func(err error) bool { return err.Error() == msg }
}

func absenceMsgPrefix(prefix string) func(error) bool {
	return func(err error) bool { return strings.HasPrefix(err.Error(), prefix) }
}

// adminCreateStaffAbsence handles POST /api/staff/{id}/absences (#1843).
// Files an absence on the staff member's behalf ("Kollegin ruft morgens an,
// Leitung trägt ein"); a full-day sick absence cascades into Dienstplan and
// Betreuungsplan inside this request's tenant tx. Service errors mark the tx
// for rollback (they render as non-5xx, which would otherwise commit the
// partially applied cascade).
func (rs *Resource) adminCreateStaffAbsence(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}
	editorStaffID, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	var req activeSvc.CreateAbsenceRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	actorAccountID := int64(claims.ID)
	resp, err := rs.StaffAbsenceService.CreateAbsenceFor(r.Context(), staffID, editorStaffID, &actorAccountID, req)
	if err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.RenderWithRules(err, adminAbsenceErrorRules, common.ErrorInternalServer))
		return
	}
	common.Respond(w, r, http.StatusCreated, resp, "Absence created successfully")
}

// adminDeleteStaffAbsence handles DELETE /api/staff/{id}/absences/{absenceId}
// (#1843). Deleting a sick report reverses its plan cascade first (same tx) —
// without this endpoint an admin could not undo their own mis-entry.
func (rs *Resource) adminDeleteStaffAbsence(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	absenceID, err := parseInt64Param(r, "absenceId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.PersonService.GetStaffByID(r.Context(), staffID); err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("staff not found")))
		return
	}
	editorStaffID, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	actorAccountID := int64(claims.ID)
	if err := rs.StaffAbsenceService.DeleteAbsenceFor(r.Context(), staffID, editorStaffID, &actorAccountID, absenceID); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.RenderWithRules(err, adminAbsenceErrorRules, common.ErrorInternalServer))
		return
	}
	common.RespondNoContent(w, r)
}
