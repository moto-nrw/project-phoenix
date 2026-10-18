package studentpresencehttp

import (
	context "context"
	errors "errors"
	http "net/http"

	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	absencerecords "github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	securityruntime "github.com/moto-nrw/project-phoenix/modules/securityruntime"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// CallerContext is the slice of the Identity & Access caller context the
// student routes read: the verified staff record, the caller's staff member
// (found is false, without an error, for a caller who is no staff member)
// and the student access decision.
type CallerContext interface {
	HasCurrentStaff(ctx context.Context) (bool, error)
	CurrentStaffID(ctx context.Context) (staffID int64, found bool, err error)
	common.StudentAccessSource
	// The note audience (#3632) asks which children this caller actually has:
	// their own and substituted OGS groups, the activities they supervise, and
	// the school classes assigned to them via education.class_teachers.
	MyGroupIDs(ctx context.Context) ([]int64, error)
	MyTeacherGroupIDs(ctx context.Context) ([]int64, error)
	MyActivityGroupIDs(ctx context.Context) ([]int64, error)
	MySchoolClasses(ctx context.Context) ([]string, error)
}

// canManageStudentAbsence is the action-scoped write gate for the absence
// statuses (krank, entschuldigt, Klassenfahrt): CanUpdateStudent plus the
// users:absence read-pair prerequisite, so a custom role limited to absence
// writes never reaches further than its users:read footing (#2232, #2329).
func (rs *Resource) canManageStudentAbsence(ctx context.Context, userPermissions []string, student *peopleModule.StudentState) (bool, error) {
	return securityruntime.CanManageStudentAbsence(ctx, userPermissions, student, rs.UserContextService)
}

// canManageStudentStatus applies the direct-report setting only to sick and
// excused statuses. Parent review and class trips retain their separate gates.
func (rs *Resource) canManageStudentStatus(ctx context.Context, userPermissions []string, student *peopleModule.StudentState, status string) (bool, error) {
	allowed, err := rs.canManageStudentAbsence(ctx, userPermissions, student)
	if !allowed || err != nil || (status != absencerecords.StudentStatusDaySick && status != absencerecords.StudentStatusDayExcused) {
		return allowed, err
	}
	if securityruntime.HasAdminWildcard(userPermissions) {
		return true, nil
	}
	if !securityruntime.HasPermission(permissions.UsersUpdate, userPermissions) &&
		(!securityruntime.HasPermission(permissions.UsersAbsence, userPermissions) || !securityruntime.HasPermission(permissions.UsersRead, userPermissions)) {
		return false, errors.New("absence write permission required")
	}
	claims := sessiontoken.ClaimsFromCtx(ctx)
	if (claims.Scope != "" && claims.Scope != "tenant" && claims.Scope != "org") ||
		claims.ID <= 0 || claims.TenantID <= 0 || claims.TenantID != tenant.FromContext(ctx) ||
		student.TenantID != claims.TenantID {
		return false, errors.New("OGS staff required for direct absence reports")
	}
	if rs.SettingsService == nil {
		return false, errors.New("absence edit settings unavailable")
	}
	scope, err := rs.SettingsService.ResolveString(ctx, settingStudentAbsenceEditScope)
	if err != nil {
		return false, err
	}
	if scope != studentAbsenceEditScopeAllStaff {
		return false, errors.New("direct absence reports require an OGS admin")
	}
	return true, nil
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
func (rs *Resource) checkStudentFullAccess(r *http.Request, student *peopleModule.StudentState) bool {
	userPermissions := sessiontoken.PermissionsFromCtx(r.Context())
	if !securityruntime.HasAdminWildcard(userPermissions) &&
		!securityruntime.HasPermission(permissions.UsersUpdate, userPermissions) {
		return false
	}
	ok, _ := securityruntime.CanUpdateStudent(
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
// Delegates to securityruntime.CanReadStudent so the same predicate is reusable
// from other handlers (timetable, per-student day view).
func (rs *Resource) checkStudentReadAccess(r *http.Request, student *peopleModule.StudentState) bool {
	return securityruntime.CanReadStudent(
		r.Context(),
		sessiontoken.PermissionsFromCtx(r.Context()),
		student,
		rs.UserContextService,
	)
}
