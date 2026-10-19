package compose

import (
	"errors"
	"time"

	application "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// SignedIdentityTokens serves the session and MFA challenge ports with one signer.
type SignedIdentityTokens interface {
	identityPorts.TokenCodec
	identityPorts.MFAChallengeCodec
	identityPorts.InvitationOwnerTokens
}

func NewSessionTokenCodec(signer application.JWTSigner, accessExpiry, refreshExpiry time.Duration) (SignedIdentityTokens, error) {
	if signer == nil {
		return nil, errors.New("session token signer is required")
	}
	return application.NewSessionCodec(signer, accessExpiry, refreshExpiry), nil
}
