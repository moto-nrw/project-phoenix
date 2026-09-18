package operator

import (
	"errors"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
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
// unregistered RFID scan. Unknown or already handled scans, a missing
// operator, and invalid IDs stay 400 with their text. Persistence failures
// stay internal and never echo adapter or Postgres text: Device Fleet wraps
// driver errors and does not return *models/base.DatabaseError.
func UnregisteredTagScanResolveError(err error) render.Renderer {
	if _, ok := errors.AsType[*modelBase.DatabaseError](err); ok {
		return ErrInternal("Failed to resolve unregistered RFID scan")
	}
	if unregisteredTagScanResolveIsClientError(err) {
		return ErrInvalidRequest(err)
	}
	return ErrInternal("Failed to resolve unregistered RFID scan")
}

func unregisteredTagScanResolveIsClientError(err error) bool {
	for err != nil {
		switch err.Error() {
		case "operator ID is required",
			"scan ID is required",
			"unregistered tag scan not found",
			"unregistered tag scan already resolved",
			"invalid unregistered tag scan":
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
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
	case errors.Is(err, authSvc.ErrMFARateLimited):
		return ErrTooManyRequests("Too many code requests, please wait")
	case errors.Is(err, authSvc.ErrMFALocked):
		return ErrTooManyRequests("MFA account temporarily locked")
	case errors.Is(err, authSvc.ErrMFAChallengeTokenInvalid),
		errors.Is(err, authSvc.ErrMFACodeInvalid):
		return ErrInvalidCredentials()
	case errors.Is(err, authSvc.ErrMFAStatusUnavailable):
		return ErrServiceUnavailable("MFA status temporarily unavailable, please retry")
	default:
		return ErrInternal("Authentication failed")
	}
}
