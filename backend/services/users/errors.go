package users

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
)

// The model-free error vocabulary of the people services lives in People
// Directory's userscontract package (#3750), and this package names those
// instances directly. What stays here is the guardian vocabulary. Do not
// change the messages: they are rendered verbatim by the HTTP layer.
var (
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

// ValidationFailure returns the client-input failure inside err, if any: the
// userscontract.ValidationError the guardian and caregiver services raise. The
// root composition classifies on it but may not import that package.
func ValidationFailure(err error) (error, bool) {
	validation, ok := errors.AsType[*userscontract.ValidationError](err)
	if !ok {
		return nil, false
	}
	return validation, true
}
