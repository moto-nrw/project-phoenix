// Package common - student_access.go
//
// The student-data-access decision (#2329) belongs to the Identity & Access
// caller context. Handlers in different packages (students, active, …) keep
// calling common.DetermineStudentAccess so the api layer does not own it.
package common

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// StudentAccessContext caches the per-request access decision so
// per-student checks during list iteration stay O(1).
//
// The rules are (#2329):
//   - Admin permissions (`admin:*` or `*:*`) → full access
//   - Verified staff record in the tenant    → full access
//   - Otherwise (guest, guardian)            → redacted
type StudentAccessContext struct {
	IsAdmin bool
	IsStaff bool
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

// StudentAccessSource answers the caller context's student access decision:
// the admin permission and the verified staff record.
type StudentAccessSource interface {
	StudentAccessFacts(ctx context.Context) (admin, staff bool)
}

// DetermineStudentAccess resolves the access context for the caller of r
// through the caller context.
func DetermineStudentAccess(r *http.Request, source StudentAccessSource) *StudentAccessContext {
	admin, staff := source.StudentAccessFacts(r.Context())
	return &StudentAccessContext{IsAdmin: admin, IsStaff: staff}
}

// DetermineStudentAccessWithStaffLookup applies the same rules to a projected
// staff lookup: admin status comes from the JWT permissions, and the staff
// record is looked up only for non-admin callers.
func DetermineStudentAccessWithStaffLookup(r *http.Request, lookup func(context.Context) (bool, error)) *StudentAccessContext {
	ctx := r.Context()
	access := &StudentAccessContext{IsAdmin: authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx))}
	if access.IsAdmin {
		return access
	}
	found, err := lookup(ctx)
	access.IsStaff = err == nil && found
	return access
}
