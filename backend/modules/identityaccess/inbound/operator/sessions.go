package operator

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
)

// Sessions are the operator session middleware chains the operator router
// mounts in front of its route groups (#2736). The router names only these
// chains; the token verifier, the authenticators and the operator gates are
// Identity & Access's.
type Sessions struct {
	tokenAuth *sessiontoken.TokenAuth
	operators identityaccess.OperatorSessionLiveness
}

// NewSessions binds the operator session chains to the session signer and
// the operator lookup of the active-operator gate.
func NewSessions(tokenAuth *sessiontoken.TokenAuth, operators identityaccess.OperatorSessionLiveness) Sessions {
	return Sessions{tokenAuth: tokenAuth, operators: operators}
}

// Configured reports whether the session signer is present. Without it the
// chains cannot verify a token.
func (s Sessions) Configured() bool {
	return s.tokenAuth != nil
}

// Refresh authenticates a refresh token, without a scope check.
func (s Sessions) Refresh() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		s.tokenAuth.Verifier(),
		sessiontoken.AuthenticateRefreshJWT,
	}
}

// MFAEnrollment accepts only the operator enrollment token the login mints
// when no MFA credential is on file (#1308): never an access token and never
// another portal's enrollment token.
func (s Sessions) MFAEnrollment() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		s.tokenAuth.Verifier(),
		sessiontoken.EnrollmentAuthenticatorFor(tokenprimitives.PlatformClaims),
	}
}

// Operator authenticates an operator access token: platform scope, the
// read-only preview guard, the security principal and an operator that is
// still active.
func (s Sessions) Operator(preview, principal func(http.Handler) http.Handler) []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		s.tokenAuth.Verifier(),
		sessiontoken.Authenticator,
		preview,
		RequiresOperatorScope,
		principal,
		RequiresActiveOperator(s.operators),
	}
}
