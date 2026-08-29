package timetable

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// SchoolSupervisionRouter is the assignment-bound supervision surface of the
// school portal ("moto schule", #2527), mounted at /school/supervisions.
//
// It is the SAME operations service and the SAME handlers the OGS portal
// drives — a Lehrkraft runs her Lernzeit exactly the way a Betreuungskraft
// runs hers. What differs is the mantle, and it differs in four ways:
//
//  1. jwt.SchoolMiddleware (via ProtectedSchoolGroup) admits only school-scope
//     tokens, so a tenant token cannot reach these routes and a school token
//     still reaches nothing under /api (TestSchoolScopeRejectedOnAllAPIRoutes).
//  2. permissions.SupervisionOwn instead of SchedulesRead: the permission says
//     "own", and the planner stays shut.
//  3. The day list runs in PlannedNowScopeDay, which filters to the caller's
//     own instance_staff assignments unconditionally — the #2380 school-wide
//     overview never applies to a Lehrkraft.
//  4. Per-child writes are additionally bound to the block's roster
//     (requireRosterStudent), because a Lehrkraft has no student directory to
//     have legitimately obtained another child's id from.
//
// Deliberately NOT mounted here: the spontaneous-start route (a Lehrkraft runs
// what the Betreuungsplan gave her, she does not invent blocks), reopen (a
// correction on a finished block is office work), and active-sessions /
// roster-by-active-group (both answer for the whole school).
func (rs *Resource) SchoolSupervisionRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedSchoolGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {
		own := common.RequiresPermission(permissions.SupervisionOwn)
		attendance := common.RequireWebAttendanceEnabled(rs.SettingsService)

		r.With(own, withTx).Get("/", rs.schoolMySupervisions)
		r.With(own, withTx).Get("/{id}/roster", rs.operationsRoster)
		r.With(own, withTx).Get("/{id}/students/{student_id}/sheet", rs.schoolStudentSheet)
		r.With(own, withTx).Post("/{id}/start", rs.operationsStart)
		r.With(own, withTx, attendance).Post("/{id}/complete", rs.operationsComplete)
		r.With(own, withTx, attendance).Post("/{id}/students/{student_id}/check-in", rs.operationsCheckInStudent)
		r.With(own, withTx, attendance).Post("/{id}/students/{student_id}/check-out", rs.operationsCheckOutStudent)
		r.With(own, withTx, attendance).Patch("/{id}/students/{student_id}/attendance", rs.operationsPatchAttendance)
	})

	return r
}

// schoolMySupervisions serves the Lehrkraft's own Betreuungsplan blocks for
// TODAY, in every lifecycle state.
//
// The date is not a parameter on purpose: this list answers "was mache ich
// jetzt", and a Lehrkraft's access to a child's data follows the day she is
// planned into. Letting her page through the week would widen that access to
// every day she has ever been assigned, for no benefit at the point of use.
func (rs *Resource) schoolMySupervisions(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	accountID, isAdmin := operationActor(r.Context())
	result, err := rs.OperationsService.PlannedNow(
		r.Context(), accountID, isAdmin, timezone.TodayDate(), rs.Now(),
		scheduleSvc.PlannedNowOptions{Scope: scheduleSvc.PlannedNowScopeDay},
	)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"instances": result}, "Eigene Aufsichten des Tages")
}

// schoolStudentSheet serves the pickup and emergency information of ONE child
// of a block the caller runs (#2527).
//
// Authorization is the roster itself: Roster() re-checks the caller's
// assignment to the block, and the child must appear on the rows it returns.
// That makes the disclosure boundary of this sheet exactly the group standing
// in front of the person — no wider, and it shrinks the moment the assignment
// is removed.
func (rs *Resource) schoolStudentSheet(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil || rs.ReportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("supervision sheet resource not fully wired")))
		return
	}
	instanceID, studentID, ok := parseOperationInstanceStudentIDs(w, r)
	if !ok {
		return
	}
	accountID, isAdmin := operationActor(r.Context())
	roster, err := rs.OperationsService.Roster(r.Context(), accountID, isAdmin, instanceID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}

	boundary := make([]int64, 0, len(roster.Rows))
	onRoster := false
	for _, row := range roster.Rows {
		boundary = append(boundary, row.StudentID)
		if row.StudentID == studentID {
			onRoster = true
		}
	}
	if !onRoster {
		common.RenderError(w, r, common.ErrorForbidden(
			errors.New("Dieses Kind gehört nicht zu dieser Aufsicht"), //nolint:staticcheck // ST1005: user-facing German message
		))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	sheet, err := rs.ReportService.SupervisionStudentSheet(r.Context(), enrollmentSvc.SupervisionSheetInput{
		StudentID:         studentID,
		Date:              timezone.TodayDate(),
		CompanionBoundary: boundary,
		ActorAccountID:    accountID,
		ActorRole:         strings.Join(claims.Roles, ","),
	})
	if err != nil {
		if errors.Is(err, enrollmentSvc.ErrReportInvalidFilter) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		rs.getLogger().Error("supervision sheet failed",
			"instance_id", instanceID,
			"student_id", studentID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, sheet, "Kind-Informationen abgerufen")
}
