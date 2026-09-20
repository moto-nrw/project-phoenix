package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// lifecycleError translates a flow error to the public contract: the
// operation envelope keeps its text, a lifecycle sentinel gains its public
// twin, and everything else falls through to the authentication mapping
// (account, tenant and session sentinels the preview and offboarding share).
func lifecycleError(err error) error {
	if err == nil {
		return nil
	}
	var operation *application.OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: lifecycleError(operation.Err)}
	}
	var validation *domain.GuardianInvitationValidationError
	if errors.As(err, &validation) {
		return &identityaccess.GuardianInvitationValidationError{Err: validation.Err}
	}
	for _, sentinel := range lifecycleSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return authenticationError(err)
}
