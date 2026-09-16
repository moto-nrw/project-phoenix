package operator

import (
	"net"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// Operator login and refresh are Identity & Access routes (#3252); this
// router mounts them from modules/identityaccess/inbound/operator. The wire
// types stay reachable here because the passkey, invitation and profile
// routes answer with the same shapes.

// trustedDeviceCookieName is the MFA trusted-device cookie the operator
// login reads and the MFA verification sets.
const trustedDeviceCookieName = identityoperator.TrustedDeviceCookieName

type (
	// LoginRequest represents the login request body.
	LoginRequest = identityoperator.LoginRequest
	// LoginResponse represents the discriminated login response.
	LoginResponse = identityoperator.LoginResponse
	// OperatorResponse represents an operator in a response.
	OperatorResponse = identityoperator.OperatorResponse
	// RefreshTokenResponse represents the refresh token response.
	RefreshTokenResponse = identityoperator.RefreshTokenResponse
)

// IdentityResponses hands the operator surface's error bodies to the
// Identity & Access operator routes, so both halves answer in one format.
func IdentityResponses() identityoperator.Responses {
	return identityoperator.Responses{
		InvalidRequest:     ErrInvalidRequest,
		InvalidCredentials: ErrInvalidCredentials,
		Unauthorized:       ErrUnauthorized,
		NotFound:           ErrNotFound,
		Forbidden:          ErrForbidden,
		Conflict:           ErrConflict,
		AuthFallback:       AuthErrorRenderer,
		ProfileFallback:    ProfileErrorRenderer,
		AccessFallback:     ProvisioningErrorRenderer,
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) net.IP {
	return common.ParseClientIP(r)
}
