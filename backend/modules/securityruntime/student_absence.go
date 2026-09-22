package securityruntime

import "github.com/moto-nrw/project-phoenix/auth/authorize"

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
