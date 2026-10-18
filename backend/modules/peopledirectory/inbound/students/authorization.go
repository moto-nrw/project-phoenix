package students

import (
	context "context"
	http "net/http"

	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	securityruntime "github.com/moto-nrw/project-phoenix/modules/securityruntime"
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

// canModifyStudentPhoto and canReadStudentPhoto are the child-data gates of
// the photo routes. They live here, not with the owner: People Directory owns
// the photo, this adapter owns the decision about who may reach it — the same
// split every other student route already follows (#3349).
//
// Both are caller-only (#2329): admin or a verified staff member of the
// tenant, every other authenticated role out. The route's own permission
// middleware still decides WHICH photo operation the caller may reach. The
// staff lookup needs a tenant transaction in ctx (#3830).
func (rs *Resource) canModifyStudentPhoto(ctx context.Context) bool {
	ok, _ := securityruntime.CanUpdateStudent(
		ctx, sessiontoken.PermissionsFromCtx(ctx), photoAuthorizationStudent{}, rs.UserContextService)
	return ok
}

func (rs *Resource) canReadStudentPhoto(ctx context.Context) bool {
	return securityruntime.CanReadStudent(
		ctx, sessiontoken.PermissionsFromCtx(ctx), photoAuthorizationStudent{}, rs.UserContextService)
}

// photoAuthorizationStudent satisfies the marker the child-data gates take.
// Both gates decide on the caller alone, so the row carries no information the
// decision needs.
type photoAuthorizationStudent struct{}

func (photoAuthorizationStudent) IsAuthorizationStudent() bool { return true }

// StudentReadAccess resolves the directory row and its existing read authorization
// for an owner handler mounted by the host. Parsing failures are rendered here.
func (rs *Resource) StudentReadAccess(w http.ResponseWriter, r *http.Request) (int64, bool, bool) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return 0, false, false
	}
	return student.ID, true, rs.checkStudentReadAccess(r, student)
}
