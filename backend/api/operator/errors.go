package operator

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// ErrResponse is the operator surface's error body. api/common owns it so the
// operator handlers served by the owner modules render the same wire format.
type ErrResponse = common.OperatorErrResponse

// ErrInvalidRequest creates an error response for invalid requests
func ErrInvalidRequest(err error) render.Renderer { return common.OperatorInvalidRequest(err) }

// ErrInvalidCredentials creates an error response for invalid credentials
func ErrInvalidCredentials() render.Renderer { return common.OperatorInvalidCredentials() }

// ErrUnauthorized creates an unauthorized error response for invalid/expired tokens
func ErrUnauthorized() render.Renderer { return common.OperatorUnauthorized() }

// ErrNotFound creates a not found error response
func ErrNotFound(message string) render.Renderer { return common.OperatorNotFound(message) }

// ErrConflict creates a conflict error response.
func ErrConflict(message string) render.Renderer { return common.OperatorConflict(message) }

// ErrForbidden creates a forbidden error response
func ErrForbidden(message string) render.Renderer { return common.OperatorForbidden(message) }

// ErrTooManyRequests creates a rate limit error response
func ErrTooManyRequests(message string) render.Renderer {
	return common.OperatorTooManyRequests(message)
}

// ErrInternal creates an internal server error response
func ErrInternal(message string) render.Renderer { return common.OperatorInternal(message) }

// ErrServiceUnavailable creates a 503 response. Used when a transient
// dependency makes a security decision impossible and the safe behaviour
// is to refuse this caller without globally locking everyone out.
func ErrServiceUnavailable(message string) render.Renderer {
	return common.OperatorServiceUnavailable(message)
}

// UnregisteredTagScanResolveError renders a failed resolution of an
// unregistered RFID scan: a database failure stays internal, every other
// outcome (unknown or already handled scan, missing operator) is reported to
// the operator verbatim.
func UnregisteredTagScanResolveError(err error) render.Renderer {
	if _, ok := errors.AsType[*modelBase.DatabaseError](err); ok {
		return ErrInternal("Failed to resolve unregistered RFID scan")
	}
	return ErrInvalidRequest(err)
}

// AuthErrorRenderer maps auth service errors to HTTP responses
func AuthErrorRenderer(err error) render.Renderer {
	var invalidCreds *platformSvc.InvalidCredentialsError
	var operatorInactive *platformSvc.OperatorInactiveError
	var operatorNotFound *platformSvc.OperatorNotFoundError

	switch {
	case errors.As(err, &invalidCreds):
		return ErrInvalidCredentials()
	case errors.As(err, &operatorInactive):
		return ErrForbidden("Operator account is inactive")
	case errors.As(err, &operatorNotFound):
		return ErrInvalidCredentials()
	case errors.Is(err, authService.ErrMFARateLimited):
		return ErrTooManyRequests("Too many code requests, please wait")
	case errors.Is(err, authService.ErrMFALocked):
		return ErrTooManyRequests("MFA account temporarily locked")
	case errors.Is(err, authService.ErrMFAChallengeTokenInvalid),
		errors.Is(err, authService.ErrMFACodeInvalid):
		return ErrInvalidCredentials()
	case errors.Is(err, authService.ErrMFAStatusUnavailable):
		return ErrServiceUnavailable("MFA status temporarily unavailable, please retry")
	default:
		return ErrInternal("Authentication failed")
	}
}
