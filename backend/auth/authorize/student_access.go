package authorize

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// StudentAccessUserContext is the narrow subset of the user-context service
// the student gates need: identify the caller's staff record (if any).
// Defined here (not imported) to keep this package a sibling of
// services/usercontext without a package cycle.
type StudentAccessUserContext interface {
	GetCurrentStaff(ctx context.Context) (*users.Staff, error)
}

// CanReadStudent decides whether the caller is allowed to see unredacted
// student data (profile, location, pickup/arrival schedules, day plan).
//
// Admin permissions (admin:* / *:*) or a verified staff record in the current
// tenant grant access; every other authenticated role (guest, guardian) with
// users:read stays redacted. Student access is tenant-wide for staff (#2329) —
// the former per-group scoping and its gdpr.student_data_scope setting are
// gone, the Betreuer/Admin split lives entirely in the permission catalog.
func CanReadStudent(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentAccessUserContext,
) bool {
	if student == nil {
		return false
	}
	if HasAdminWildcard(userPermissions) {
		return true
	}
	return isVerifiedStaff(ctx, userCtx)
}

// CanModifyStudent decides whether the caller may write to this student row
// (update, delete, photo change, ...): admin, or any verified staff member of
// the tenant (#2329). Route-level permission middleware (users:update,
// users:delete, ...) still decides WHICH writes the caller may reach; this
// gate only keeps non-staff accounts (guests, guardians) out. The `operation`
// parameter shapes the human-readable error so the caller can map it to the
// right HTTP message ("update", "delete", ...).
//
// Returns (true, nil) when allowed; (false, error) with a stable error
// message otherwise — handlers map the error string to the user-facing
// 403 reason.
func CanModifyStudent(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentAccessUserContext,
	operation string,
) (bool, error) {
	if HasAdminWildcard(userPermissions) {
		return true, nil
	}
	if student == nil {
		return false, fmt.Errorf("insufficient permissions to %s this student's data", operation)
	}
	if !isVerifiedStaff(ctx, userCtx) {
		return false, fmt.Errorf("only staff members can %s student data", operation)
	}
	return true, nil
}

// CanUpdateStudent is a convenience wrapper for update operations.
func CanUpdateStudent(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentAccessUserContext,
) (bool, error) {
	return CanModifyStudent(ctx, userPermissions, student, userCtx, "update")
}

// CanDeleteStudent is a convenience wrapper for delete operations.
func CanDeleteStudent(
	ctx context.Context,
	userPermissions []string,
	student *users.Student,
	userCtx StudentAccessUserContext,
) (bool, error) {
	return CanModifyStudent(ctx, userPermissions, student, userCtx, "delete")
}

// WritableStudentFilter returns a predicate reporting whether the caller may
// WRITE a given student, resolving the caller's staff record once so a
// caller-side loop (e.g. filtering a review queue) does not re-resolve it per
// student. The predicate is the set form of CanModifyStudent: admin or
// verified staff → every student; otherwise none.
func WritableStudentFilter(ctx context.Context, userPermissions []string, userCtx StudentAccessUserContext) func(*users.Student) bool {
	if HasAdminWildcard(userPermissions) || isVerifiedStaff(ctx, userCtx) {
		return func(student *users.Student) bool { return student != nil }
	}
	return func(*users.Student) bool { return false }
}

// isVerifiedStaff reports whether the caller has a staff record in the current
// tenant. Guests and guardians authenticate against the same tenant portal, so
// the gates check this rather than trusting a permission alone.
func isVerifiedStaff(ctx context.Context, userCtx StudentAccessUserContext) bool {
	if userCtx == nil {
		return false
	}
	staff, err := userCtx.GetCurrentStaff(ctx)
	return err == nil && staff != nil
}
