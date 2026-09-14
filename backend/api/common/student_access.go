// Package common - student_access.go
//
// Thin re-export shims over the student-data-access policy, which lives in
// services/usercontext (StudentAccessContext + ResolveStudentAccess). Handlers
// in different packages (students, active, …) keep calling
// common.DetermineStudentAccess so the api layer does not own the
// student-access decision.
package common

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// StudentAccessContext re-exports the usercontext type so existing callers'
// method calls (HasFullAccess / HasFullAccessToStudent) keep working.
type StudentAccessContext = usercontext.StudentAccessContext

// DetermineStudentAccess resolves the access context for the caller of r by
// delegating to the usercontext policy.
func DetermineStudentAccess(
	r *http.Request,
	userContextSvc usercontext.CurrentStaff,
) *StudentAccessContext {
	return usercontext.ResolveStudentAccess(r.Context(), userContextSvc)
}

// DetermineStudentAccessWithStaffLookup accepts a projected staff lookup.
func DetermineStudentAccessWithStaffLookup(r *http.Request, lookup func(context.Context) (bool, error)) *StudentAccessContext {
	return usercontext.ResolveStudentAccessWithStaffLookup(r.Context(), lookup)
}
