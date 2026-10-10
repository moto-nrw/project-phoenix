// Package userscontract is the model-free error vocabulary of the retained
// people services (#3750, services/users slice 1). services/users kept
// aliases of every name until the person and student services moved (#3753);
// callers now name these instances directly. The messages are part of the wire
// contract and must not change.
//
// The person and student services moved to modules/peopledirectory/compose
// (#3753) and brought ErrPersonNotFound and ErrStudentNotFound along. The
// public People Directory package has sentinels of the same names and
// messages; they are separate instances, and the retained callers match these.
// The staff write refusals (ErrStaffAdoptionNotPermitted,
// ErrStaffLehrkraftCaregiverProfile, ErrStaffInUse) live in the root
// composition next to the staff directory that raises them, because that
// composition may not import this package.
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

	// Companion ("läuft mit" / Laufgemeinschaft) validation errors. The messages
	// are German because the child detail view surfaces them verbatim.

	// ErrCompanionNotFound indicates a submitted companion is not a child of
	// this school
	ErrCompanionNotFound = errors.New("Ein ausgewähltes Kind wurde nicht gefunden.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrDuplicateCompanion indicates the same child was submitted twice
	ErrDuplicateCompanion = errors.New("Ein Kind kann nur einmal in der Laufgemeinschaft stehen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionWeekdayRequired indicates a companion was submitted without
	// any weekday — a link that applies on no day is not a link
	ErrCompanionWeekdayRequired = errors.New("Bitte mindestens einen Wochentag auswählen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionDayNotAllowed indicates a link on a weekday the child's own
	// departure plan does not permit leaving with another child
	ErrCompanionDayNotAllowed = errors.New("An diesem Tag ist \"Anderes Kind\" als Heimweg nicht erlaubt.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrTooManyCompanions indicates more companions than MaxStudentCompanions
	ErrTooManyCompanions = errors.New("Es können höchstens 10 Kinder verknüpft werden.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionAtLimit indicates the cap is already reached at the FAR end of
	// a submitted link: an edge counts for both children, so a list that is
	// short enough for the child being edited can still overflow a companion who
	// is already linked to MaxStudentCompanions others.
	ErrCompanionAtLimit = errors.New("Ein ausgewähltes Kind ist bereits mit 10 Kindern verknüpft.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionsChanged indicates the submitted list was built on a snapshot
	// that is no longer the stored one — someone else changed this child's
	// Laufgemeinschaft in between.
	//
	// The submitted list REPLACES the stored one, so without this check two
	// staff members editing the same child from the same snapshot would both
	// send a complete list and the second write would silently delete the links
	// the first one committed. The row locks decide the ORDER of the two writes;
	// only the fingerprint comparison under those locks can tell that the second
	// one is stale. Retriable after a reload, hence a 409.
	ErrCompanionsChanged = errors.New("Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert. Bitte neu laden und noch einmal speichern.") //nolint:staticcheck // ST1005: user-facing German message

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
