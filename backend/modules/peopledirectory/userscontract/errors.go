// Package userscontract is the model-free error vocabulary of the retained
// people services (#3750, services/users slice 1). It moved out of
// services/users so later slices can leave that package without importing it
// back; services/users keeps aliases of every name until the person and student
// services move (#3753), so errors.Is and errors.As match the same instances
// for every existing caller. The messages are part of the wire contract and
// must not change.
//
// The interface half (PersonService and the model-typed inputs it names) does
// not move: it exposes models/users, models/base and internal/timezone, which no
// people-directory package may import without a new ratchet key. Its consumers
// depend on ports they own (#3771), and the interface leaves together with its
// implementation (#3753).
//
// ErrPersonNotFound, ErrStudentNotFound, ErrGuardianDeletePreviewChanged,
// ErrGuardianForceDeleteRequiresAdmin, ErrPayerRemovalRequiresFinancial and
// GuardianStillLinkedError already exist in the public People Directory package
// with their own instances and stay defined in services/users: importing the
// public package from a people-directory application package is a forbidden
// edge. ErrCompanionWouldLoseDeparture and ErrCompanionLockBusy stay
// re-exports of models/users there: the repository's departure-plan write path
// raises that instance and it carries its own message.
package userscontract

import (
	"errors"
	"fmt"
)

var (
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

	// ErrStaffAdoptionNotPermitted indicates a staff-creation request landed on
	// a person who already carries a live staff record, from a caller that may
	// only create. Adopting that record writes the notes and the caregiver
	// fields of someone who is already in the directory, which is an edit — and
	// POST /api/staff is gated on users:create alone.
	//
	// Since #2906 the required authority is staff:manage — the same one
	// PUT /api/staff/{id} needs. Admins hold it through the admin:* wildcard,
	// so this refuses the direct-API case, not the staff form.
	ErrStaffAdoptionNotPermitted = errors.New("Für das Ändern eines vorhandenen Mitarbeiter-Datensatzes fehlt die Berechtigung") //nolint:staticcheck // ST1005: user-facing German message

	// ErrStaffLehrkraftCaregiverProfile indicates a caregiver profile was
	// requested for an account holding the Lehrkraft system role (#1772). That
	// role is class_day:read only and is provisioned without a profile on
	// purpose; the role-assignment paths refuse the same combination from the
	// other direction (ErrRoleLehrkraftCaregiverProfile).
	ErrStaffLehrkraftCaregiverProfile = errors.New("Ein Lehrkraft-Konto kann kein Betreuungsprofil erhalten") //nolint:staticcheck // ST1005: user-facing German message

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

	// ErrStaffInUse indicates staff has attendance records or active supervisions
	ErrStaffInUse = errors.New("Personal kann nicht gelöscht werden: Mitarbeiter/in hat aktive Aufsichten oder Anwesenheitseinträge") //nolint:staticcheck // ST1005: user-facing German message
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
