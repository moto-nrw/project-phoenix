package users

import (
	"errors"
	"fmt"
)

// Common error variables for the users service
var (
	// ErrPersonNotFound indicates a person could not be found
	ErrPersonNotFound = errors.New("person not found")

	// ErrInvalidPersonData indicates invalid person data
	ErrInvalidPersonData = errors.New("invalid person data")

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

	// ErrGuardianNotFound indicates a guardian could not be found
	ErrGuardianNotFound = errors.New("guardian not found")

	// ErrStaffNotFound indicates a staff member could not be found
	ErrStaffNotFound = errors.New("staff member not found")

	// ErrTeacherNotFound indicates a teacher could not be found
	ErrTeacherNotFound = errors.New("teacher not found")

	// ErrStaffAlreadyExists indicates a staff member already exists for the person
	ErrStaffAlreadyExists = errors.New("staff member already exists for this person")

	// ErrTeacherAlreadyExists indicates a teacher already exists for the staff member
	ErrTeacherAlreadyExists = errors.New("teacher already exists for this staff member")

	// ErrInvalidPIN indicates an invalid staff PIN
	ErrInvalidPIN = errors.New("invalid staff PIN")

	// ErrStaffInUse indicates staff has attendance records or active supervisions
	ErrStaffInUse = errors.New("Personal kann nicht gelöscht werden: Mitarbeiter/in hat aktive Aufsichten oder Anwesenheitseinträge") //nolint:staticcheck // ST1005: user-facing German message

	// ErrPersonHasDependents indicates person has linked staff/student/account records
	ErrPersonHasDependents = errors.New("Person kann nicht gelöscht werden: Person hat verknüpfte Personal-, Kinder- oder Kontodaten") //nolint:staticcheck // ST1005: user-facing German message

	// ErrGuardianHasStudents indicates guardian is still linked to students
	ErrGuardianHasStudents = errors.New("Erziehungsberechtigte/r kann nicht gelöscht werden: Noch mit Kinder verknüpft") //nolint:staticcheck // ST1005: user-facing German message
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
