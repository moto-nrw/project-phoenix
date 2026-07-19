package users

import (
	"errors"
	"fmt"
)

// Common error variables for the users service
var (
	// ErrPersonNotFound indicates a person could not be found
	ErrPersonNotFound = errors.New("person not found")

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

	// ErrStudentNotFound indicates a student could not be found in this tenant
	ErrStudentNotFound = errors.New("student not found")

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

	// ErrCompanionWouldLoseDeparture indicates that removing a link would leave
	// the OTHER child with an accompanied ("Anderes Kind") departure plan and no
	// remaining detail — neither another link nor a free-text note. Refused
	// rather than silently narrowing that child's plan: one child's edit must
	// never change another child's departure permissions, and leaving the
	// contradiction in place would block every later edit of that child.
	ErrCompanionWouldLoseDeparture = errors.New("Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionExtensionNotAuthorized indicates the write path was asked to
	// widen a companion's departure plan that the caller was never authorized
	// for. Not user-facing: it can only fire if the authorization pass and the
	// write pass disagree, which the shared row locks are supposed to prevent —
	// so it must surface as a 500 (rolling the transaction back), never as a
	// 4xx the middleware would commit.
	ErrCompanionExtensionNotAuthorized = errors.New("companion departure-plan extension was not authorized")

	// ErrStaffInUse indicates staff has attendance records or active supervisions
	ErrStaffInUse = errors.New("Personal kann nicht gelöscht werden: Mitarbeiter/in hat aktive Aufsichten oder Anwesenheitseinträge") //nolint:staticcheck // ST1005: user-facing German message

	// ErrGuardianDeletePreviewChanged indicates the affected guardian links no
	// longer match the set the admin confirmed.
	ErrGuardianDeletePreviewChanged = errors.New("Die Verknüpfungen haben sich seit der Vorschau geändert. Bitte erneut prüfen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrGuardianForceDeleteRequiresAdmin indicates a full delete of a guardian
	// still linked to students was requested by a non-admin. The full delete
	// reaches across siblings the caller may not supervise, so it is restricted
	// to admins.
	ErrGuardianForceDeleteRequiresAdmin = errors.New("only administrators can fully delete a guardian linked to students")
)

// GuardianStillLinkedError signals that a guardian delete was refused because the
// guardian is still linked to one or more students and no force delete was
// requested. StudentNames carries the affected children for the caller to render
// (only admins may see them).
type GuardianStillLinkedError struct {
	StudentNames []string
}

// Error returns a stable, non-user-facing marker; the HTTP layer builds the
// audience-specific German message from StudentNames.
func (e *GuardianStillLinkedError) Error() string {
	return "guardian is still linked to students"
}

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
