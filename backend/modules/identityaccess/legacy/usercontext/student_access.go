package usercontext

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// StudentAccessContext caches the per-request access decision so per-student
// checks during list iteration stay O(1).
//
// The rules are (#2329):
//   - Admin permissions (`admin:*` or `*:*`) → full access
//   - Verified staff record in the tenant    → full access
//   - Otherwise (guest, guardian)            → redacted
type StudentAccessContext struct {
	IsAdmin bool
	IsStaff bool
}

type CurrentStaff interface {
	GetCurrentStaff(context.Context) (*users.Staff, error)
}

// ResolveStudentAccess resolves the access context for the current caller.
// Admin status comes from JWT permissions; the staff record is looked up only
// for non-admin callers.
func ResolveStudentAccess(
	ctx context.Context,
	userCtx CurrentStaff,
) *StudentAccessContext {
	return ResolveStudentAccessWithStaffLookup(ctx, func(ctx context.Context) (bool, error) {
		staff, err := userCtx.GetCurrentStaff(ctx)
		return staff != nil, err
	})
}

// ResolveStudentAccessWithStaffLookup applies the same policy to a projected identity lookup.
func ResolveStudentAccessWithStaffLookup(ctx context.Context, lookup func(context.Context) (bool, error)) *StudentAccessContext {
	access := &StudentAccessContext{
		IsAdmin: authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx)),
	}

	if access.IsAdmin {
		return access
	}

	found, err := lookup(ctx)
	access.IsStaff = err == nil && found
	return access
}

// HasFullAccess returns true when the caller may see unredacted student data.
func (a *StudentAccessContext) HasFullAccess() bool {
	return a != nil && (a.IsAdmin || a.IsStaff)
}

// HasFullAccessToStudent is a nil-safe convenience wrapper for callers that
// hold the full *users.Student.
func (a *StudentAccessContext) HasFullAccessToStudent(student *users.Student) bool {
	if student == nil {
		return false
	}
	return a.HasFullAccess()
}
