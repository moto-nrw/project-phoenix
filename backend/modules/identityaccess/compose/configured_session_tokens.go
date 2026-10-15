package compose

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// NewConfiguredSessionAuth builds the HTTP session runtime the server root
// mounts (verifier, authenticator, rate-limit key) from configuration values:
// the signing secret and the lifetimes of the access and refresh tokens. The
// root hands over values instead of building the signer (ADR 0053 point 6,
// ADR 0056).
func NewConfiguredSessionAuth(secret string, accessExpiry, refreshExpiry time.Duration) (*sessiontoken.TokenAuth, error) {
	signer, err := sessiontoken.NewTokenAuthWithDurations(secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid auth JWT configuration: %w", err)
	}
	return signer, nil
}

// NewConfiguredSessionTokens builds the session token codec from the same
// configuration values. The signing primitive stays in Identity's session
// token adapter, so a root hands over values instead of building the signer
// (ADR 0053 point 6).
func NewConfiguredSessionTokens(secret string, accessExpiry, refreshExpiry time.Duration) (SignedIdentityTokens, error) {
	signer, err := NewConfiguredSessionAuth(secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, err
	}
	return NewSessionTokenCodec(signer.JwtAuth, signer.JwtExpiry, signer.JwtRefreshExpiry)
}
