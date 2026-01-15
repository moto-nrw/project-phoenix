package jwt

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/jwtauth/v5"
	jwx "github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/spf13/viper"
)

// TokenAuth implements JWT authentication flow.
type TokenAuth struct {
	JwtAuth          *jwtauth.JWTAuth
	JwtExpiry        time.Duration
	JwtRefreshExpiry time.Duration
}

// NewTokenAuth configures and returns a JWT authentication instance.
func NewTokenAuth() (*TokenAuth, error) {
	secret := strings.TrimSpace(viper.GetString("auth_jwt_secret"))
	if secret == "" {
		return nil, errors.New("AUTH_JWT_SECRET is required")
	}
	if strings.EqualFold(secret, "random") {
		return nil, errors.New("AUTH_JWT_SECRET cannot be 'random'; set a secure secret")
	}

	jwtExpiry := viper.GetDuration("auth_jwt_expiry")
	if jwtExpiry <= 0 {
		return nil, errors.New("AUTH_JWT_EXPIRY is required and must be greater than zero")
	}
	jwtRefreshExpiry := viper.GetDuration("auth_jwt_refresh_expiry")
	if jwtRefreshExpiry <= 0 {
		return nil, errors.New("AUTH_JWT_REFRESH_EXPIRY is required and must be greater than zero")
	}

	// Validate secret length/strength
	if len(secret) < 32 {
		logger.Logger.WithField("secret_length", len(secret)).Warn("JWT secret is too short. Recommend at least 32 chars.")
	}

	return NewTokenAuthWithSecret(secret, jwtExpiry, jwtRefreshExpiry)
}

// MustTokenAuth creates TokenAuth or terminates the process on error.
func MustTokenAuth() *TokenAuth {
	tokenAuth, err := NewTokenAuth()
	if err != nil {
		logger.Logger.WithError(err).Fatal("JWT configuration is invalid")
	}
	return tokenAuth
}

// NewTokenAuthWithSecret creates a TokenAuth with a specific secret
func NewTokenAuthWithSecret(secret string, jwtExpiry, jwtRefreshExpiry time.Duration) (*TokenAuth, error) {
	a := &TokenAuth{
		JwtAuth:          jwtauth.New("HS256", []byte(secret), nil),
		JwtExpiry:        jwtExpiry,
		JwtRefreshExpiry: jwtRefreshExpiry,
	}

	return a, nil
}

// Verifier http middleware will verify a jwt string from a http request.
func (a *TokenAuth) Verifier() func(http.Handler) http.Handler {
	return jwtauth.Verifier(a.JwtAuth)
}

// GenTokenPair returns both an access token and a refresh token.
func (a *TokenAuth) GenTokenPair(accessClaims AppClaims, refreshClaims RefreshClaims) (string, string, error) {
	access, err := a.CreateJWT(accessClaims)
	if err != nil {
		return "", "", err
	}
	refresh, err := a.CreateRefreshJWT(refreshClaims)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// GenerateTokenPair satisfies port.TokenProvider using core claims.
func (a *TokenAuth) GenerateTokenPair(accessClaims port.AppClaims, refreshClaims port.RefreshClaims) (string, string, error) {
	return a.GenTokenPair(accessClaims, refreshClaims)
}

// Decode satisfies port.TokenProvider for decoding token strings.
func (a *TokenAuth) Decode(tokenString string) (jwx.Token, error) {
	return a.JwtAuth.Decode(tokenString)
}

// AccessExpiry satisfies port.TokenProvider.
func (a *TokenAuth) AccessExpiry() time.Duration {
	return a.JwtExpiry
}

// RefreshExpiry satisfies port.TokenProvider.
func (a *TokenAuth) RefreshExpiry() time.Duration {
	return a.JwtRefreshExpiry
}

// CreateJWT returns an access token for provided account claims.
func (a *TokenAuth) CreateJWT(c AppClaims) (string, error) {
	c.IssuedAt = time.Now().Unix()
	c.ExpiresAt = time.Now().Add(a.JwtExpiry).Unix()

	claims, err := ParseStructToMap(c)
	if err != nil {
		return "", err
	}

	_, tokenString, err := a.JwtAuth.Encode(claims)
	return tokenString, err
}

func ParseStructToMap(c any) (map[string]any, error) {
	var claims map[string]any
	inrec, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(inrec, &claims)
	if err != nil {
		return nil, err
	}

	// Special handling for embedded structs like CommonClaims
	// This ensures all fields from embedded structs are properly included
	if appClaims, ok := c.(AppClaims); ok {
		// Make sure roles is explicitly set
		claims["roles"] = appClaims.Roles

		// Make sure permissions is explicitly set
		claims["permissions"] = appClaims.Permissions

		// Set common claims manually to ensure they're included
		claims["exp"] = appClaims.ExpiresAt
		claims["iat"] = appClaims.IssuedAt
	}

	return claims, nil
}

// CreateRefreshJWT returns a refresh token for provided token Claims.
func (a *TokenAuth) CreateRefreshJWT(c RefreshClaims) (string, error) {
	c.IssuedAt = time.Now().Unix()
	c.ExpiresAt = time.Now().Add(a.JwtRefreshExpiry).Unix()

	claims, err := ParseStructToMap(c)
	if err != nil {
		return "", err
	}

	_, tokenString, err := a.JwtAuth.Encode(claims)
	return tokenString, err
}

// GetRefreshExpiry returns the refresh token expiration duration
func (a *TokenAuth) GetRefreshExpiry() time.Duration {
	return a.JwtRefreshExpiry
}

var _ port.TokenProvider = (*TokenAuth)(nil)
