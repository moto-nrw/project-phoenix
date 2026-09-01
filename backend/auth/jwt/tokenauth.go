package jwt

import (
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/moto-nrw/project-phoenix/internal/randstr"
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
	secret := viper.GetString("auth_jwt_secret")

	// Handle "random" secret setting with persistence
	if secret == "random" {
		var err error
		secret, err = resolveRandomSecret()
		if err != nil {
			return nil, err
		}
	}

	// Validate secret length/strength
	if len(secret) < 32 {
		log.Printf("Warning: JWT secret is too short (%d chars). Recommend at least 32 chars.", len(secret))
	}

	return NewTokenAuthWithSecret(secret)
}

// MustNewTokenAuth is like NewTokenAuth but fatals on error.
// Use this in Router() functions where JWT auth is required at startup.
func MustNewTokenAuth() *TokenAuth {
	ta, err := NewTokenAuth()
	if err != nil {
		slog.Error("failed to initialize JWT auth", slog.String("error", err.Error()))
		os.Exit(1)
	}
	return ta
}

// resolveRandomSecret generates or loads a persistent development secret.
func resolveRandomSecret() (string, error) {
	// Check environment - don't allow random in production
	env := viper.GetString("app_env")
	if env == "production" {
		return "", errors.New("JWT secret cannot be 'random' in production")
	}

	// For development, use a persistent secret file
	baseDir := viper.GetString("app_base_dir")
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			baseDir = "."
		}
	}

	// Store secret in a file within the project
	secretFile := filepath.Join(baseDir, ".jwt-dev-secret.key")
	secretBytes, err := os.ReadFile(secretFile)

	if err == nil && len(secretBytes) >= 32 {
		log.Printf("Using persistent JWT secret from %s", secretFile)
		return string(secretBytes), nil
	}

	// Generate new secret
	secret, err := randstr.String(32, randstr.Alphanumeric)
	if err != nil {
		panic(err)
	}
	log.Printf("Generated new JWT secret and saving to %s", secretFile)

	// Save for future use
	if err := os.WriteFile(secretFile, []byte(secret), 0600); err != nil {
		log.Printf("Warning: Could not persist JWT secret: %v", err)
	}

	return secret, nil
}

// NewTokenAuthWithSecret creates a TokenAuth with a specific secret
func NewTokenAuthWithSecret(secret string) (*TokenAuth, error) {
	a := &TokenAuth{
		JwtAuth:          jwtauth.New("HS256", []byte(secret), nil),
		JwtExpiry:        viper.GetDuration("auth_jwt_expiry"),
		JwtRefreshExpiry: viper.GetDuration("auth_jwt_refresh_expiry"),
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

		// Make sure scope is explicitly set (for operator/platform tokens)
		if appClaims.Scope != "" {
			claims["scope"] = appClaims.Scope
		}

		// Multi-tenancy fields (only include when non-zero)
		if appClaims.TenantID != 0 {
			claims["tenant_id"] = appClaims.TenantID
		}
		if appClaims.OrgID != 0 {
			claims["org_id"] = appClaims.OrgID
		}
		if appClaims.FamilyID != "" {
			claims["family_id"] = appClaims.FamilyID
		}
		// Admin staff-view preview claims (#2893) — explicit like the fields
		// above so the allowlist stays the single source of truth.
		if appClaims.ReadOnly {
			claims["read_only"] = true
		}
		if appClaims.ActingAdminID != 0 {
			claims["acting_admin_id"] = appClaims.ActingAdminID
		}
		if appClaims.PreviewID != "" {
			claims["preview_id"] = appClaims.PreviewID
		}

		// Set common claims manually to ensure they're included
		claims["exp"] = appClaims.ExpiresAt
		claims["iat"] = appClaims.IssuedAt
	}

	return claims, nil
}

// CreateRefreshJWT returns a refresh token for provided token Claims.
func (a *TokenAuth) CreateRefreshJWT(c RefreshClaims) (string, error) {
	c.IssuedAt = time.Now().Unix()
	if c.ExpiresAt <= 0 {
		c.ExpiresAt = time.Now().Add(a.JwtRefreshExpiry).Unix()
	}

	claims, err := ParseStructToMap(c)
	if err != nil {
		return "", err
	}

	_, tokenString, err := a.JwtAuth.Encode(claims)
	return tokenString, err
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

// ParseExpiredAccessJWT is ParseAccessJWT without the expiry check: the
// signature still has to verify, so the claims are as trustworthy as ever —
// only their freshness is not. Use it ONLY where an expired token is
// evidence, never where it grants access. The staff-view preview (#2893)
// ends this way: an admin who lets the preview run past the 15-minute access
// expiry and then clicks "Vorschau beenden" must still produce a
// staff_preview_ended row, otherwise the audit trail loses the pair.
func (a *TokenAuth) ParseExpiredAccessJWT(tokenString string) (*AppClaims, error) {
	return a.parseAccessJWT(tokenString, true)
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

// GetRefreshExpiry returns the refresh token expiration duration
func (a *TokenAuth) GetRefreshExpiry() time.Duration {
	return a.JwtRefreshExpiry
}
