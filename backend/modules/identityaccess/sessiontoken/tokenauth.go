package sessiontoken

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth/v5"
)

// TokenAuth implements JWT authentication flow.
type TokenAuth struct {
	JwtAuth          *jwtauth.JWTAuth
	JwtExpiry        time.Duration
	JwtRefreshExpiry time.Duration
}

// NewTokenAuthWithDurations creates an instance from explicit configuration.
// The signing key is required configuration: no root generates or persists one.
func NewTokenAuthWithDurations(secret string, expiry, refreshExpiry time.Duration) (*TokenAuth, error) {
	if err := rejectGeneratedSecret(secret); err != nil {
		return nil, err
	}
	if len(secret) < 32 {
		log.Printf("Warning: JWT secret is too short (%d chars). Recommend at least 32 chars.", len(secret))
	}
	a := &TokenAuth{
		JwtAuth:          jwtauth.New("HS256", []byte(secret), nil),
		JwtExpiry:        expiry,
		JwtRefreshExpiry: refreshExpiry,
	}

	return a, nil
}

// rejectGeneratedSecret refuses the retired "random" mode.
func rejectGeneratedSecret(secret string) error {
	if secret == "random" {
		return errors.New("AUTH_JWT_SECRET=random is not allowed; set an explicit secret")
	}
	return nil
}

// Verifier verifies the token of a request. ParseVerified after it parses
// that token once for the middleware that follows.
func (a *TokenAuth) Verifier() func(http.Handler) http.Handler {
	return jwtauth.Verifier(a.JwtAuth)
}

// ParseAccessJWT verifies an access token a request carries in its body and
// refuses an expired one; the staff-view preview end (#2893) proves with it
// which preview token the admin held.
func (a *TokenAuth) ParseAccessJWT(tokenString string) (*AppClaims, error) {
	token, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	decoded := decode(token)
	if decoded.AccessErr != nil {
		return nil, decoded.AccessErr
	}
	if decoded.access.ExpiresAt > 0 && decoded.access.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("access token expired")
	}
	return &decoded.access, nil
}
