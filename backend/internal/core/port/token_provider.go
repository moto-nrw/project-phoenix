package port

import (
	"time"

	jwx "github.com/lestrrat-go/jwx/v2/jwt"
)

// TokenProvider defines the contract for JWT token creation and decoding.
type TokenProvider interface {
	GenerateTokenPair(accessClaims AppClaims, refreshClaims RefreshClaims) (string, string, error)
	Decode(tokenString string) (jwx.Token, error)
	AccessExpiry() time.Duration
	RefreshExpiry() time.Duration
}
