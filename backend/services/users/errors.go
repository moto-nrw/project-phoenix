package users

import (
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
)

// The model-free error vocabulary of this package lives in People Directory's
// userscontract package (#3750). The names below stay until the person and
// student services move (#3753), so errors.Is and errors.As keep matching the
// same instances for every caller. Do not change the messages: they are
// rendered verbatim by the HTTP layer and by PyrePortal.
var (
	ErrPersonIdentifierRequired        = userscontract.ErrPersonIdentifierRequired
	ErrAccountNotFound                 = userscontract.ErrAccountNotFound
	ErrRFIDCardNotFound                = userscontract.ErrRFIDCardNotFound
	ErrAccountAlreadyLinked            = userscontract.ErrAccountAlreadyLinked
	ErrRFIDCardAlreadyLinked           = userscontract.ErrRFIDCardAlreadyLinked
	ErrTeacherNotFound                 = userscontract.ErrTeacherNotFound
	ErrStaffNotFound                   = userscontract.ErrStaffNotFound
	ErrStaffAdoptionNotPermitted       = userscontract.ErrStaffAdoptionNotPermitted
	ErrStaffLehrkraftCaregiverProfile  = userscontract.ErrStaffLehrkraftCaregiverProfile
	ErrStudentGraduated                = userscontract.ErrStudentGraduated
	ErrCompanionNotFound               = userscontract.ErrCompanionNotFound
	ErrDuplicateCompanion              = userscontract.ErrDuplicateCompanion
	ErrCompanionWeekdayRequired        = userscontract.ErrCompanionWeekdayRequired
	ErrCompanionDayNotAllowed          = userscontract.ErrCompanionDayNotAllowed
	ErrTooManyCompanions               = userscontract.ErrTooManyCompanions
	ErrCompanionAtLimit                = userscontract.ErrCompanionAtLimit
	ErrCompanionsChanged               = userscontract.ErrCompanionsChanged
	ErrCompanionExtensionNotAuthorized = userscontract.ErrCompanionExtensionNotAuthorized
	ErrStaffInUse                      = userscontract.ErrStaffInUse

	// ErrPersonNotFound indicates a person could not be found
	ErrPersonNotFound = errors.New("person not found")

	// ErrStudentNotFound indicates a student could not be found in this tenant
	ErrStudentNotFound = errors.New("student not found")

	// ErrGuardianDeletePreviewChanged indicates the affected guardian links no
	// longer match the set the admin confirmed.
	ErrGuardianDeletePreviewChanged = errors.New("Die Verknüpfungen haben sich seit der Vorschau geändert. Bitte erneut prüfen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrGuardianForceDeleteRequiresAdmin indicates a full delete of a guardian
	// still linked to students was requested by a non-admin. The full delete
	// reaches across siblings the caller may not supervise, so it is restricted
	// to admins.
	ErrGuardianForceDeleteRequiresAdmin = errors.New("only administrators can fully delete a guardian linked to students")

	// ErrPayerRemovalRequiresFinancial refuses to unlink a guardian who is the
	// child's payer when the caller lacks guardians:financial (#2608). Clearing
	// the payer mark changes who is charged for the child; that decision is
	// owned by the financial permission, so users:update alone must not reach
	// it through a plain unlink.
	ErrPayerRemovalRequiresFinancial = errors.New("Diese Person ist als Zahler für das Kind eingetragen. Zum Entfernen ist die Berechtigung für Bankverbindungen nötig. Bitte wenden Sie sich an die Schulleitung.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionWouldLoseDeparture and ErrCompanionLockBusy are re-exported
	// from models/users: the repository's shared departure-plan write path
	// returns the same instances, so errors.Is matches regardless of which
	// layer refused.
	ErrCompanionWouldLoseDeparture = userModels.ErrCompanionWouldLoseDeparture
	ErrCompanionLockBusy           = userModels.ErrCompanionLockBusy
)

type (
	// UsersError represents an error in the users service.
	UsersError = userscontract.UsersError
	// ValidationError marks a users-service failure as bad client input.
	ValidationError = userscontract.ValidationError
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
