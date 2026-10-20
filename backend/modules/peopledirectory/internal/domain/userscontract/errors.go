// Package userscontract is the model-free error vocabulary of the retained
// people services (#3750, services/users slice 1). services/users kept
// aliases of every name until the person and student services moved (#3753);
// callers now name these instances directly. The messages are diagnostic detail;
// clients show the catalog text selected by the error code.
//
// The person and student services moved to modules/peopledirectory/compose
// (#3753) and brought ErrPersonNotFound and ErrStudentNotFound along. The
// public People Directory package has sentinels of the same names and
// messages; they are separate instances, and the retained callers match these.
// The staff write refusals (ErrStaffAdoptionNotPermitted,
// ErrStaffLehrkraftCaregiverProfile, ErrStaffInUse) live in the People
// Directory composition next to the staff directory that raises them
// (#4007).
package userscontract

import (
	"errors"
	"fmt"
)

var (
	// ErrPersonNotFound indicates a person could not be found
	ErrPersonNotFound = errors.New("person not found")

	// ErrStudentNotFound indicates a student could not be found in this tenant
	ErrStudentNotFound = errors.New("student not found")

	// ErrPersonIdentifierRequired indicates missing required identifier
	ErrPersonIdentifierRequired = errors.New("either tag ID or account ID is required")

	// ErrAccountNotFound indicates an account could not be found
	ErrAccountNotFound = errors.New("account not found")

	// ErrRFIDCardNotFound indicates an RFID card could not be found
	ErrRFIDCardNotFound = errors.New("RFID card not found")

	// ErrAccountAlreadyLinked indicates an account is already linked to another person
	ErrAccountAlreadyLinked = errors.New("account is already linked to another person")

	// ErrRFIDCardAlreadyLinked indicates an RFID card is already linked to another person
	ErrRFIDCardAlreadyLinked = errors.New("RFID card is already linked to another person")

	// ErrTeacherNotFound indicates a teacher could not be found
	ErrTeacherNotFound = errors.New("teacher not found")

	// ErrStaffNotFound indicates the account has no staff record in this
	// tenant. Self-service staff settings render it as a 404 rather than a
	// 500: an account that is not staff here has nothing to configure.
	ErrStaffNotFound = errors.New("staff not found")

	// ErrStudentGraduated indicates a write that only makes sense for an
	// enrolled child targeted a graduated (alumnus) student. Graduation is a
	// soft delete, so callers render it as the same 404 every other
	// staff-facing student route returns for an alumnus (#405).
	ErrStudentGraduated = errors.New("student has graduated")

	// ErrCompanionExtensionNotAuthorized indicates the write path was asked to
	// widen a companion's departure plan that the caller was never authorized
	// for. Not user-facing: it can only fire if the authorization pass and the
	// write pass disagree, which the shared row locks are supposed to prevent —
	// so it must surface as a 500 (rolling the transaction back), never as a
	// 4xx the middleware would commit.
	ErrCompanionExtensionNotAuthorized = errors.New("companion departure-plan extension was not authorized")
)

// UsersError represents an error in the users service
type UsersError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
}

// ValidationError marks a users-service failure as bad client input rather than
// an operational or persistence problem.
type ValidationError struct {
	Err error
}

// Error returns the error message
func (e *UsersError) Error() string {
	return fmt.Sprintf("users.%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *UsersError) Unwrap() error {
	return e.Err
}

// Error returns the validation message.
func (e *ValidationError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// Unwrap returns the underlying validation error.
func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
