package application

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// MapError reports a store or service sentinel as its public twin. A mapped
// sentinel loses its wrapped text, as the capability always reported it.
func MapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrAccountNotFound):
		return identityaccess.ErrAccountNotFound
	case errors.Is(err, domain.ErrRoleNotFound):
		return identityaccess.ErrRoleNotFound
	case errors.Is(err, domain.ErrTenantRequired):
		return identityaccess.ErrTenantRequired
	case errors.Is(err, domain.ErrCatererAccountJoinsNoSchool):
		// A caterer account answers like an address that is taken (ADR 0077).
		return fmt.Errorf("%w: %w", identityaccess.ErrEmailAlreadyExists, err)
	case errors.Is(err, domain.ErrGuardianRoleMissing):
		return identityaccess.ErrGuardianRoleMissing
	case errors.Is(err, domain.ErrGuardianStudentAccessTenantMismatch):
		return fmt.Errorf("%w: %w", identityaccess.ErrGuardianStudentAccessTenantMismatch, err)
	case errors.Is(err, identityaccess.ErrOperatorNotFound):
		return identityaccess.ErrOperatorNotFound
	case errors.Is(err, identityaccess.ErrOperatorSessionNotFound):
		return identityaccess.ErrOperatorSessionNotFound
	case errors.Is(err, identityaccess.ErrOperatorSessionRotated):
		return identityaccess.ErrOperatorSessionRotated
	case errors.Is(err, identityaccess.ErrAccountSessionNotFound):
		return identityaccess.ErrAccountSessionNotFound
	case errors.Is(err, identityaccess.ErrAccountSessionRotated):
		return identityaccess.ErrAccountSessionRotated
	case errors.Is(err, identityaccess.ErrOperatorMFACredentialNotFound):
		return identityaccess.ErrOperatorMFACredentialNotFound
	case errors.Is(err, identityaccess.ErrOperatorMFAChallengeNotFound):
		return identityaccess.ErrOperatorMFAChallengeNotFound
	case errors.Is(err, identityaccess.ErrOperatorMFAChallengeStateChanged):
		return identityaccess.ErrOperatorMFAChallengeStateChanged
	case errors.Is(err, identityaccess.ErrOperatorTrustedDeviceNotFound):
		return identityaccess.ErrOperatorTrustedDeviceNotFound
	case errors.Is(err, identityaccess.ErrOperatorInvitationNotFound):
		return identityaccess.ErrOperatorInvitationNotFound
	case errors.Is(err, identityaccess.ErrOperatorEmailChangeNotFound):
		return identityaccess.ErrOperatorEmailChangeNotFound
	case errors.Is(err, identityaccess.ErrOperatorPasskeyNotFound):
		return identityaccess.ErrOperatorPasskeyNotFound
	case errors.Is(err, identityaccess.ErrOperatorPasskeySessionNotFound):
		return identityaccess.ErrOperatorPasskeySessionNotFound
	case errors.Is(err, identityaccess.ErrAccountPasskeyNotFound):
		return identityaccess.ErrAccountPasskeyNotFound
	case errors.Is(err, identityaccess.ErrAccountPasskeySessionNotFound):
		return identityaccess.ErrAccountPasskeySessionNotFound
	default:
		return err
	}
}
