package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"

// An owner error keeps its exact message and original cause while also
// satisfying the workflow's native classification. No cause is discarded.
type ownerErrorClass struct{ cause, class error }

func (e ownerErrorClass) Error() string        { return e.cause.Error() }
func (e ownerErrorClass) Unwrap() error        { return e.cause }
func (e ownerErrorClass) Is(target error) bool { return target == e.class }
func WithGuardianProfileMissingCause(cause error) error {
	if cause == nil {
		return nil
	}
	return ownerErrorClass{cause, care.ErrGuardianProfileNotFound}
}
func WithGuardianRelationshipMissingCause(cause error) error {
	if cause == nil {
		return nil
	}
	return ownerErrorClass{cause, care.ErrStudentGuardianNotFound}
}
