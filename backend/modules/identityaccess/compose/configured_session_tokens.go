package compose

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// NewConfiguredSessionTokens builds the session token codec from
// configuration values: the signing secret and the lifetimes of the access
// and refresh tokens. The signing primitive stays in Identity's session token
// adapter, so a root hands over values instead of building the signer
// (ADR 0053 point 6).
func NewConfiguredSessionTokens(secret string, accessExpiry, refreshExpiry time.Duration) (SignedIdentityTokens, error) {
	signer, err := sessiontoken.NewTokenAuthWithDurations(secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid auth JWT configuration: %w", err)
	}
	return NewSessionTokenCodec(signer.JwtAuth, signer.JwtExpiry, signer.JwtRefreshExpiry)
}
