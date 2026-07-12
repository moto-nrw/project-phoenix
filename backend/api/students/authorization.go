package students

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
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

func isGroupSupervisor(ctx context.Context, groupID int64, ucs userContextService.UserContextService) bool {
	return authorize.IsGroupSupervisor(ctx, groupID, ucs)
}

// checkStudentFullAccess determines if the current user has full access to
// a student's data for write operations (update, delete, privacy consent, etc.).
// Returns true if the user is an admin or supervises the student's group.
//
// The gdpr.student_data_scope setting intentionally does NOT apply here.
// write operations remain restricted to group supervisors regardless of scope.
// For read access checks, use checkStudentReadAccess instead.
func (rs *Resource) checkStudentFullAccess(r *http.Request, student *users.Student) bool {
	return rs.isGroupSupervisorOrAdmin(r, student)
}

// checkStudentReadAccess determines if the current user has full read access
// to a student's data (profile, location, visit info, privacy details, pickup
// schedules). Returns true if the user is an admin, a verified staff member
// when the tenant's student_data_scope is set to all_staff, or a supervisor
// of the student's education group.
//
// This function MUST only be used on read paths. Write operations must use
// checkStudentFullAccess which ignores the scope setting.
//
// Delegates to authorize.CanReadStudent so the same predicate is reusable
// from other handlers (timetable, per-student day view) without duplicating
// the scope/admin/supervisor logic.
func (rs *Resource) checkStudentReadAccess(r *http.Request, student *users.Student) bool {
	return authorize.CanReadStudent(
		r.Context(),
		jwt.PermissionsFromCtx(r.Context()),
		student,
		rs.UserContextService,
		rs.SettingsService,
		rs.Logger,
	)
}

// isGroupSupervisorOrAdmin checks if the caller is an admin or supervises the
// student's education group. This is the core authorization logic shared by
// both read and write access paths (before scope overrides are applied).
func (rs *Resource) isGroupSupervisorOrAdmin(r *http.Request, student *users.Student) bool {
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	if authorize.HasAdminWildcard(userPermissions) {
		return true
	}

	if student.GroupID == nil {
		return false
	}

	educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
	if err != nil {
		return false
	}

	for _, group := range educationGroups {
		if group.ID == *student.GroupID {
			return true
		}
	}

	return false
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
