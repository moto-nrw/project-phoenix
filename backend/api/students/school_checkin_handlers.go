package students

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// schoolCheckinRequest is the payload for POST /api/students/{id}/school-checkin.
// Action is explicit (not a toggle) so the endpoint is idempotent: repeating
// "in" when the student is already checked in returns 200 without change.
type schoolCheckinRequest struct {
	Action string `json:"action"` // "in" | "out"
}

// schoolCheckinResponse mirrors the shape of AttendanceStatus so the frontend
// can render the new state directly without a follow-up fetch. The Location
// field is the resolved binary-mode label ("Anwesend" | "Schulhof" | "Abwesend")
// so PresenceBadge can update optimistically in the UI.
type schoolCheckinResponse struct {
	StudentID    int64      `json:"student_id"`
	Status       string     `json:"status"`
	CheckInTime  *time.Time `json:"check_in_time,omitempty"`
	CheckOutTime *time.Time `json:"check_out_time,omitempty"`
	YardSince    *time.Time `json:"yard_since,omitempty"`
	Location     string     `json:"location"`
	Changed      bool       `json:"changed"` // false when the call was a no-op (already in target state)
}

const (
	schoolCheckinActionIn  = "in"
	schoolCheckinActionOut = "out"
)

// schoolCheckinHandler marks a student as checked-in or checked-out of the
// school via the web UI. Mode-agnostic — always writes active.attendance only;
// never creates visits (those come from the IoT kiosk flow). In detailed mode
// a "out" action also ends any open visit for that student so the attendance
// and visit state stay consistent.
//
// Authorization layers:
//  1. JWT + users:checkin permission (route middleware)
//  2. Tenant scoping: student belongs to caller's tenant (middleware + repo)
//  3. attendance.web_checkin_access setting:
//     - "all_staff"          → any caller with the permission passes
//     - "group_supervisors"  → caller must supervise the student's educational group
func (rs *Resource) schoolCheckinHandler(w http.ResponseWriter, r *http.Request) {
	logger := rs.getLogger().With(slog.String("handler", "schoolCheckin"))

	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	var req schoolCheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.Action != schoolCheckinActionIn && req.Action != schoolCheckinActionOut {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(`action must be "in" or "out"`)))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	if err := rs.enforceWebCheckinAccess(r.Context(), staffID, student.ID); err != nil {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	current, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	resp, changeErr := rs.applySchoolCheckinAction(r.Context(), student, staffID, req.Action, current)
	if changeErr != nil {
		// A graduated (alumnus) student — reached via CheckInStudent's
		// ensureStudentCheckinAllowed guard on a stale request or graduation
		// race — is treated like an unknown/absent student (404), matching the
		// IoT and timetable mappers rather than surfacing a 500 (#405).
		if errors.Is(changeErr, activeService.ErrStudentGraduated) {
			common.RenderError(w, r, common.ErrorNotFound(changeErr))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(changeErr))
		return
	}

	logger.Info("student_school_checkin",
		slog.Int64("student_id", student.ID),
		slog.Int64("staff_id", staffID),
		slog.String("action", req.Action),
		slog.String("resulting_status", resp.Status),
		slog.Bool("changed", resp.Changed),
	)

	common.Respond(w, r, http.StatusOK, resp, "School checkin toggled successfully")
}

// evaluateWebCheckinAccess is the pure decision layer of the access gate:
// given the resolved mode and whether the caller supervises the student's
// educational group, decide whether to allow or deny. Extracted for
// hermetic unit testing without mocking a SettingsService or ActiveService.
// Returns nil on allow, an error on deny.
func evaluateWebCheckinAccess(mode string, supervisorHasAccess bool) error {
	if mode == configModel.WebCheckinAccessAllStaff {
		return nil
	}
	// group_supervisors (default) — caller must supervise the student's
	// educational group.
	if !supervisorHasAccess {
		return errors.New("caller is not a supervisor of this student's educational group")
	}
	return nil
}

// enforceWebCheckinAccess enforces the attendance.web_checkin_access setting
// for a given caller+student pair. Returns nil on allow, an error on deny.
//
// Admins (admin:* or *:* in JWT claims) bypass this gate entirely — same
// pattern as the sibling helpers (isGroupSupervisorOrAdmin, canModifyStudent)
// in this package, where tenant admins are trusted on every student-scoped
// path regardless of supervision. Without this short-circuit, an admin who
// happens not to teach the student's group would get a 403 in
// `group_supervisors` mode, which contradicts the rest of the admin model.
func (rs *Resource) enforceWebCheckinAccess(ctx context.Context, staffID, studentID int64) error {
	if authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx)) {
		return nil
	}

	mode := configSvc.ResolveStringOrDefault(
		ctx,
		rs.SettingsService,
		configModel.KeyWebCheckinAccess,
		configModel.WebCheckinAccessGroupSupervisors,
		rs.getLogger(),
	)
	if rs.SettingsService == nil {
		return errors.New("settings service is not configured")
	}
	groupMode, groupModeErr := rs.SettingsService.ResolveString(ctx, configModel.KeyGroupMode)
	if groupModeErr != nil {
		return fmt.Errorf("group mode resolution failed: %w", groupModeErr)
	}
	if groupMode == configModel.GroupModeOpenCare {
		return nil
	}
	if mode == configModel.WebCheckinAccessAllStaff {
		return evaluateWebCheckinAccess(mode, false)
	}

	// group_supervisors path requires the supervisor check. CheckTeacherStudentAccess
	// encapsulates the lookup (staff -> teacher -> teacher_groups -> student.group_id).
	hasAccess, err := rs.ActiveService.CheckTeacherStudentAccess(ctx, staffID, studentID)
	if err != nil {
		return fmt.Errorf("access check failed: %w", err)
	}
	return evaluateWebCheckinAccess(mode, hasAccess)
}

// isIdempotentSchoolCheckin reports whether the requested action is already
// satisfied by the student's current attendance status. Extracted so the
// no-op decision can be unit-tested without a mock ActiveService.
func isIdempotentSchoolCheckin(action, currentStatus string) bool {
	alreadyPresent := currentStatus == "checked_in" || currentStatus == "on_yard"
	alreadyAbsent := currentStatus == "not_checked_in" || currentStatus == "checked_out"
	switch action {
	case schoolCheckinActionIn:
		return alreadyPresent
	case schoolCheckinActionOut:
		return alreadyAbsent
	default:
		return false
	}
}

// applySchoolCheckinAction is the state-transition core. It short-circuits on
// idempotent no-ops (Changed=false) and otherwise writes active.attendance.
// In detailed mode a successful "out" transition also ends any open visit so
// the room-location view doesn't drift from attendance. In binary mode
// ActiveService.EndVisit is a no-op (L3.1 gate), so the code path is identical.
func (rs *Resource) applySchoolCheckinAction(
	ctx context.Context,
	student *users.Student,
	staffID int64,
	action string,
	current *activeService.AttendanceStatus,
) (*schoolCheckinResponse, error) {
	if isIdempotentSchoolCheckin(action, current.Status) {
		return buildSchoolCheckinResponse(student.ID, current, false), nil
	}

	// Action-explicit writes — never route through ToggleStudentAttendance
	// here. The toggle re-reads state and flips, which under concurrency can
	// turn a desired "in" into an "out" (two parallel "in" requests for an
	// absent student: the second sees the first's commit and flips). The
	// CheckIn/CheckOut methods are race-safe individually (ON CONFLICT for
	// in, state-checked UPDATE for out) so the action contract is preserved.
	switch action {
	case schoolCheckinActionIn:
		if _, err := rs.ActiveService.CheckInStudent(ctx, student.ID, staffID, 0, true); err != nil {
			return nil, err
		}
	case schoolCheckinActionOut:
		// CheckOutStudent also ends any open room visit in the same request
		// transaction (issue #895 — see services/active.performCheckOut), so
		// detailed-mode supervisor views never show "still in Room X" after
		// a web checkout. No separate EndVisit call is needed here.
		if _, err := rs.ActiveService.CheckOutStudent(ctx, student.ID, staffID, true); err != nil {
			return nil, err
		}
	}

	// Re-fetch so the response reflects the freshly-written row.
	updated, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, student.ID)
	if err != nil {
		return nil, err
	}
	return buildSchoolCheckinResponse(student.ID, updated, true), nil
}

// buildSchoolCheckinResponse formats an AttendanceStatus into the HTTP response
// shape, including the resolved presence label so the client can render
// PresenceBadge without a follow-up snapshot fetch.
func buildSchoolCheckinResponse(studentID int64, status *activeService.AttendanceStatus, changed bool) *schoolCheckinResponse {
	resp := &schoolCheckinResponse{
		StudentID:    studentID,
		Status:       status.Status,
		CheckInTime:  status.CheckInTime,
		CheckOutTime: status.CheckOutTime,
		YardSince:    status.YardSince,
		Changed:      changed,
	}
	resp.Location = labelForAttendanceStatus(status.Status)
	return resp
}

// labelForAttendanceStatus maps the tri-state status to the German presence
// label used by the frontend PresenceBadge component.
func labelForAttendanceStatus(status string) string {
	switch status {
	case "checked_in":
		return "Anwesend"
	case "on_yard":
		return "Schulhof"
	default: // "checked_out", "not_checked_in", or unknown
		return "Abwesend"
	}
}

// getLogger returns a nil-safe logger for handler use.
func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.Logger, slog.Default())
}
