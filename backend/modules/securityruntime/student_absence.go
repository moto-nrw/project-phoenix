package securityruntime

import (
	"context"

	securitycontract "github.com/moto-nrw/project-phoenix/modules/securityruntime/ports"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime/internal/application/authorize"
)

// ErrAbsenceReadRequired refuses a caller who holds users:absence without
// users:read, on read surfaces and write gates alike.
var ErrAbsenceReadRequired = authorize.ErrAbsenceReadRequired

// CanReviewExcusedAbsenceRequests reports whether the permissions allow
// reviewing parent sick and excused-absence requests at all.
func CanReviewExcusedAbsenceRequests(permissions []string) bool {
	return authorize.CanReviewExcusedAbsenceRequests(permissions)
}

// AbsenceReadPrerequisiteUnmet reports users:absence held without the
// users:read permission it requires.
func AbsenceReadPrerequisiteUnmet(permissions []string) bool {
	return authorize.AbsenceReadPrerequisiteUnmet(permissions)
}

// CanManageStudentAbsence decides whether the caller may write a child's
// absence statuses and decide a guardian's sick or excused request: the
// student write gate, plus the users:read prerequisite of an absence-only
// caller (ErrAbsenceReadRequired).
func CanManageStudentAbsence(ctx context.Context, permissions []string, student interface{ IsAuthorizationStudent() bool }, userCtx securitycontract.StudentAccessUserContext) (bool, error) {
	return authorize.CanManageStudentAbsence(ctx, permissions, student, userCtx)
}
