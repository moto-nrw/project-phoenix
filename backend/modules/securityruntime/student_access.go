package securityruntime

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
)

// The permissions guarding the categories of a child's documents: health
// documents (Art. 9 GDPR), custody paperwork, and everything else, which the
// directory maintainers edit. They restate the permission registry's names,
// which this public package may not import; a test pins them to it.
const (
	PermissionStudentDocumentsHealth = "student_documents:health"
	PermissionStudentDocumentsLegal  = "student_documents:legal"
	PermissionUsersUpdate            = "users:update"
)

// StudentAccessUserContext answers whether the caller is verified staff of
// the current tenant.
type StudentAccessUserContext = authorize.StudentAccessUserContext

// CanReadStudent decides whether the caller may see an existing child's
// unredacted data: an administrator, or verified staff of the tenant (#2329).
func CanReadStudent(ctx context.Context, granted []string, student interface{ IsAuthorizationStudent() bool }, userCtx StudentAccessUserContext) bool {
	return authorize.CanReadStudent(ctx, granted, student, userCtx)
}

// CanModifyStudent decides whether the caller may write to an existing
// child's record: an administrator, or verified staff of the tenant (#2329).
func CanModifyStudent(ctx context.Context, granted []string, student interface{ IsAuthorizationStudent() bool }, userCtx StudentAccessUserContext) bool {
	allowed, _ := authorize.CanModifyStudent(ctx, granted, student, userCtx, "modify")
	return allowed
}
