package students

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
)

// HTTP-side wrappers around auth/authorize/student_access.go.

// getPermissionsFromRequest extracts permissions from request context.
func getPermissionsFromRequest(r *http.Request) []string {
	return jwt.PermissionsFromCtx(r.Context())
}

func canUpdateStudent(ctx context.Context, userPermissions []string, student *users.Student, ucs userContextService.UserContextService) (bool, error) {
	return authorize.CanUpdateStudent(ctx, userPermissions, student, ucs)
}

func canDeleteStudent(ctx context.Context, userPermissions []string, student *users.Student, ucs userContextService.UserContextService) (bool, error) {
	return authorize.CanDeleteStudent(ctx, userPermissions, student, ucs)
}

// canManageStudentAbsence is the action-scoped write gate for the absence
// statuses (krank, entschuldigt, Klassenfahrt): CanUpdateStudent plus the
// users:absence read-pair prerequisite, so a custom role limited to absence
// writes never reaches further than its users:read footing (#2232, #2329).
func (rs *Resource) canManageStudentAbsence(ctx context.Context, userPermissions []string, student *users.Student) (bool, error) {
	return authorize.CanManageStudentAbsence(ctx, userPermissions, student, rs.UserContextService)
}

// checkStudentAbsenceWriteAccess is the boolean form for the detail response's
// has_absence_write_access flag, which the frontend uses to show or hide the
// Krankmeldung / Entschuldigung / Klassenfahrt actions independently of the
// Stammdaten edit affordances (has_write_access).
func (rs *Resource) checkStudentAbsenceWriteAccess(r *http.Request, student *users.Student) bool {
	ok, _ := rs.canManageStudentAbsence(r.Context(), jwt.PermissionsFromCtx(r.Context()), student)
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
