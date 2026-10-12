package jwt

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

// Verifier http middleware will verify a jwt string from a http request.
func (a *TokenAuth) Verifier() func(http.Handler) http.Handler {
	return jwtauth.Verifier(a.JwtAuth)
}

// ParseAccessJWT verifies an access token's signature, decodes it into
// AppClaims, and rejects expired tokens. Mirrors ParseMFAChallengeJWT for the
// access-token shape: callers that receive a token in a request BODY (rather
// than through the Verifier middleware) need the same guarantees the
// middleware gives — the staff-view preview end call (#2893) proves with it
// which preview token the admin actually held.
func (a *TokenAuth) ParseAccessJWT(tokenString string) (*AppClaims, error) {
	return a.parseAccessJWT(tokenString, false)
}

func (a *TokenAuth) parseAccessJWT(tokenString string, allowExpired bool) (*AppClaims, error) {
	jwtToken, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]any)
	for _, key := range jwtToken.Keys() {
		var value any
		if jwtToken.Get(key, &value) == nil {
			raw[key] = value
		}
	}
	var claims AppClaims
	if err := claims.ParseClaims(raw); err != nil {
		return nil, err
	}
	claims.ExpiresAt = expiryFromClaims(raw)
	if !allowExpired && claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("access token expired")
	}
	return &claims, nil
}
