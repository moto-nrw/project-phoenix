package students

import (
	"context"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	activeModel "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// HTTP-side wrappers around auth/authorize/student_access.go.

// getPermissionsFromRequest extracts permissions from request context.
func getPermissionsFromRequest(r *http.Request) []string {
	return jwt.PermissionsFromCtx(r.Context())
}

// CallerContext is the slice of the Identity & Access caller context the
// student routes read: the verified staff record, the caller's staff member
// (found is false, without an error, for a caller who is no staff member)
// and the student access decision.
type CallerContext interface {
	HasCurrentStaff(ctx context.Context) (bool, error)
	CurrentStaffID(ctx context.Context) (staffID int64, found bool, err error)
	common.StudentAccessSource
}

func canUpdateStudent(ctx context.Context, userPermissions []string, student *users.Student, ucs CallerContext) (bool, error) {
	return authorize.CanUpdateStudent(ctx, userPermissions, student, ucs)
}

func canDeleteStudent(ctx context.Context, userPermissions []string, student *users.Student, ucs CallerContext) (bool, error) {
	return authorize.CanDeleteStudent(ctx, userPermissions, student, ucs)
}

// canManageStudentAbsence is the action-scoped write gate for the absence
// statuses (krank, entschuldigt, Klassenfahrt): CanUpdateStudent plus the
// users:absence read-pair prerequisite, so a custom role limited to absence
// writes never reaches further than its users:read footing (#2232, #2329).
func (rs *Resource) canManageStudentAbsence(ctx context.Context, userPermissions []string, student *users.Student) (bool, error) {
	return authorize.CanManageStudentAbsence(ctx, userPermissions, student, rs.UserContextService)
}

// canManageStudentStatus applies the direct-report setting only to sick and
// excused statuses. Parent review and class trips retain their separate gates.
func (rs *Resource) canManageStudentStatus(ctx context.Context, userPermissions []string, student *users.Student, status string) (bool, error) {
	allowed, err := rs.canManageStudentAbsence(ctx, userPermissions, student)
	if !allowed || err != nil || (status != activeModel.StudentStatusDaySick && status != activeModel.StudentStatusDayExcused) {
		return allowed, err
	}
	if authorize.HasAdminWildcard(userPermissions) {
		return true, nil
	}
	if !authorize.HasPermission(permissions.UsersUpdate, userPermissions) &&
		(!authorize.HasPermission(permissions.UsersAbsence, userPermissions) || !authorize.HasPermission(permissions.UsersRead, userPermissions)) {
		return false, errors.New("absence write permission required")
	}
	claims := jwt.ClaimsFromCtx(ctx)
	if (claims.Scope != "" && claims.Scope != "tenant" && claims.Scope != "org") ||
		claims.ID <= 0 || claims.TenantID <= 0 || claims.TenantID != tenant.FromContext(ctx) ||
		student.TenantID != claims.TenantID {
		return false, errors.New("OGS staff required for direct absence reports")
	}
	if rs.SettingsService == nil {
		return false, errors.New("absence edit settings unavailable")
	}
	scope, err := rs.SettingsService.ResolveString(ctx, configModel.KeyStudentAbsenceEditScope)
	if err != nil {
		return false, err
	}
	if scope != configModel.StudentAbsenceEditScopeAllStaff {
		return false, errors.New("direct absence reports require an OGS admin")
	}
	return true, nil
}

// checkStudentAbsenceWriteAccess is the boolean form for the detail response's
// has_absence_write_access flag, which the frontend uses to show or hide the
// Krankmeldung / Entschuldigung / Klassenfahrt actions independently of the
// Stammdaten edit affordances (has_write_access).
func (rs *Resource) checkStudentAbsenceWriteAccess(r *http.Request, student *users.Student) bool {
	ok, _ := rs.canManageStudentAbsence(r.Context(), jwt.PermissionsFromCtx(r.Context()), student)
	return ok
}

func (rs *Resource) checkStudentSickExcusedWriteAccess(r *http.Request, student *users.Student) bool {
	ok, _ := rs.canManageStudentStatus(r.Context(), jwt.PermissionsFromCtx(r.Context()), student, activeModel.StudentStatusDaySick)
	return ok
}

// checkStudentFullAccess determines if the current user has full access to
// a student's data for write operations (update, privacy consent, pickup
// exceptions, etc.): admin, or verified staff holding users:update (#2329).
//
// The permission is part of the answer on purpose: this predicate also feeds
// the detail response's has_write_access flag, and a flag that says "yes" to a
// caller the users:update route gate then refuses would light up Stammdaten
// edit affordances that can only 403 (e.g. a custom role holding just
// users:read + users:absence).
func (rs *Resource) checkStudentFullAccess(r *http.Request, student *users.Student) bool {
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	if !authorize.HasAdminWildcard(userPermissions) &&
		!authorize.HasPermission(permissions.UsersUpdate, userPermissions) {
		return false
	}
	ok, _ := authorize.CanUpdateStudent(
		r.Context(),
		userPermissions,
		student,
		rs.UserContextService,
	)
	return ok
}

// checkStudentReadAccess determines if the current user has full read access
// to a student's data (profile, location, visit info, privacy details, pickup
// schedules): admin or verified staff. Other roles (guest, guardian) stay
// redacted.
//
// Delegates to authorize.CanReadStudent so the same predicate is reusable
// from other handlers (timetable, per-student day view).
func (rs *Resource) checkStudentReadAccess(r *http.Request, student *users.Student) bool {
	return authorize.CanReadStudent(
		r.Context(),
		jwt.PermissionsFromCtx(r.Context()),
		student,
		rs.UserContextService,
	)
}

// buildSupervisorContacts creates supervisor contact list from group teachers
func (rs *Resource) buildSupervisorContacts(ctx context.Context, groupID int64) []SupervisorContact {
	teachers, err := rs.EducationService.GetGroupTeachers(ctx, groupID)
	if err != nil {
		return nil
	}

	supervisors := make([]SupervisorContact, 0, len(teachers))
	for _, teacher := range teachers {
		if supervisor := teacherToSupervisorContact(teacher); supervisor != nil {
			supervisors = append(supervisors, *supervisor)
		}
	}
	return supervisors
}

// canModifyStudentPhoto and canReadStudentPhoto are the child-data gates of
// the photo routes. They live here, not with the owner: People Directory owns
// the photo, this adapter owns the decision about who may reach it — the same
// split every other student route already follows (#3349).
//
// Both are caller-only (#2329): admin or a verified staff member of the
// tenant, every other authenticated role out. The route's own permission
// middleware still decides WHICH photo operation the caller may reach.
func (rs *Resource) canModifyStudentPhoto(ctx context.Context) bool {
	ok, _ := authorize.CanUpdateStudent(
		ctx, jwt.PermissionsFromCtx(ctx), photoAuthorizationStudent{}, rs.UserContextService)
	return ok
}

func (rs *Resource) canReadStudentPhoto(ctx context.Context) bool {
	return authorize.CanReadStudent(
		ctx, jwt.PermissionsFromCtx(ctx), photoAuthorizationStudent{}, rs.UserContextService)
}

// photoAuthorizationStudent satisfies the marker the child-data gates take.
// Both gates decide on the caller alone, so the row carries no information the
// decision needs.
type photoAuthorizationStudent struct{}

func (photoAuthorizationStudent) IsAuthorizationStudent() bool { return true }
