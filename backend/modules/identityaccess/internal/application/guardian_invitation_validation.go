package application

import "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"

func invalidGuardianInvitation(op string, err error) error {
	return failed(op, &domain.GuardianInvitationValidationError{Err: err})
}
