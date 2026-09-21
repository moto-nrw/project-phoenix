package jwt

import (
	"encoding/json"
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
