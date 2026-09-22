package operator

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// Sessions are the operator session middleware chains the operator router
// mounts in front of its route groups (#2736). The router names only these
// chains; the token verifier, the authenticators and the operator gates are
// Identity & Access's.
type Sessions struct {
	tokenAuth *jwt.TokenAuth
	operators OperatorLookup
}

// NewSessions binds the operator session chains to the session signer and
// the operator lookup of the active-operator gate.
func NewSessions(tokenAuth *jwt.TokenAuth, operators OperatorLookup) Sessions {
	return Sessions{tokenAuth: tokenAuth, operators: operators}
}

// Configured reports whether the session signer is present. Without it the
// chains cannot verify a token.
func (s Sessions) Configured() bool {
	return s.tokenAuth != nil
}

// TokenAuth is the session signer the operator MFA exchange mints tokens
// with.
func (s Sessions) TokenAuth() *jwt.TokenAuth {
	return s.tokenAuth
}

// Refresh authenticates a refresh token, without a scope check.
func (s Sessions) Refresh() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		s.tokenAuth.Verifier(),
		jwt.AuthenticateRefreshJWT,
	}
}

// MFAEnrollment accepts only the narrow enrollment token operator login
// mints when no MFA credential is on file (#1308), never a full operator
// access token.
func (s Sessions) MFAEnrollment() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		s.tokenAuth.Verifier(),
		jwt.MFAEnrollmentAuthenticator,
	}
}

// Operator authenticates an operator access token: platform scope, the
// read-only preview guard, the security principal and an operator that is
// still active.
func (s Sessions) Operator() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		s.tokenAuth.Verifier(),
		jwt.Authenticator,
		common.ReadOnlyPreviewMiddleware,
		RequiresOperatorScope,
		common.SecurityPrincipalMiddleware,
		RequiresActiveOperator(s.operators),
	}
}
