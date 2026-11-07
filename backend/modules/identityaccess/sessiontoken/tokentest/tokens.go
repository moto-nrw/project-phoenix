//go:build testhooks

// Package tokentest mints and reads the legacy session JWTs for tests. It is a
// leaf so these helpers do not change how sessiontoken, and the packages
// importing it, compile under testhooks (#5990). Production issues and reads
// tokens through the Identity & Access codecs.
package tokentest

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// GenTokenPair returns both an access token and a refresh token.
func GenTokenPair(a *sessiontoken.TokenAuth, accessClaims sessiontoken.AppClaims, refreshClaims sessiontoken.RefreshClaims) (string, string, error) {
	access, err := CreateJWT(a, accessClaims)
	if err != nil {
		return "", "", err
	}
	refresh, err := CreateRefreshJWT(a, refreshClaims)
	if err != nil {
		return "", "", err
	}
	return access, refresh, nil
}

// CreateJWT returns an access token for provided account claims.
func CreateJWT(a *sessiontoken.TokenAuth, c sessiontoken.AppClaims) (string, error) {
	c.IssuedAt = time.Now().Unix()
	c.ExpiresAt = time.Now().Add(a.JwtExpiry).Unix()

	claims, err := ParseStructToMap(c)
	if err != nil {
		return "", err
	}

	_, tokenString, err := a.JwtAuth.Encode(claims)
	return tokenString, err
}

// ParseStructToMap returns the JWT claim map for claims.
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
	if appClaims, ok := c.(sessiontoken.AppClaims); ok {
		// Make sure roles is explicitly set
		claims["roles"] = appClaims.Roles

		// Make sure permissions is explicitly set
		claims["permissions"] = appClaims.Permissions

		// Make sure scope is explicitly set (for operator/platform tokens)
		if appClaims.Scope != "" {
			claims["scope"] = appClaims.Scope
		}

		addScopedAppClaims(claims, appClaims)
		// Set common claims manually to ensure they're included
		claims["exp"] = appClaims.ExpiresAt
		claims["iat"] = appClaims.IssuedAt
	}

	return claims, nil
}

// CreateRefreshJWT returns a refresh token for provided token Claims.
func CreateRefreshJWT(a *sessiontoken.TokenAuth, c sessiontoken.RefreshClaims) (string, error) {
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

// ParseExpiredAccessJWT is ParseAccessJWT without the expiry check: the
// signature still has to verify, so the claims are as trustworthy as ever,
// only their freshness is not. Tests use it to read the claims of a token
// that has already expired.
func ParseExpiredAccessJWT(a *sessiontoken.TokenAuth, tokenString string) (*sessiontoken.AppClaims, error) {
	raw, err := decode(a, tokenString)
	if err != nil {
		return nil, err
	}
	var claims sessiontoken.AppClaims
	if err := claims.ParseClaims(raw); err != nil {
		return nil, err
	}
	if exp, ok := raw["exp"].(time.Time); ok {
		claims.ExpiresAt = exp.Unix()
	}
	return &claims, nil
}

// CreateMFAChallengeJWT mints a new MFA challenge JWT with the given TTL.
// Callers are responsible for passing a sane TTL (typically 5 minutes).
func CreateMFAChallengeJWT(a *sessiontoken.TokenAuth, c sessiontoken.MFAChallengeClaims, ttl time.Duration) (string, error) {
	now := time.Now()
	c.IssuedAt = now.Unix()
	c.ExpiresAt = now.Add(ttl).Unix()

	claims := map[string]any{
		"account_id":  c.AccountID,
		"mfa_pending": true,
		"iat":         c.IssuedAt,
		"exp":         c.ExpiresAt,
	}
	if c.Scope != "" {
		claims["scope"] = c.Scope
	}
	if c.TenantID != 0 {
		claims["tenant_id"] = c.TenantID
	}
	if c.ChallengeID != 0 {
		claims["challenge_id"] = c.ChallengeID
	}

	_, tokenString, err := a.JwtAuth.Encode(claims)
	return tokenString, err
}

// ParseMFAChallengeJWT decodes an MFA challenge token, extracts its claims
// into MFAChallengeClaims, and rejects expired tokens.
func ParseMFAChallengeJWT(a *sessiontoken.TokenAuth, tokenString string) (*sessiontoken.MFAChallengeClaims, error) {
	raw, err := decode(a, tokenString)
	if err != nil {
		return nil, err
	}
	var claims sessiontoken.MFAChallengeClaims
	if err := claims.ParseClaims(raw); err != nil {
		return nil, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("challenge token expired")
	}
	return &claims, nil
}

// CreateMFAEnrollmentJWT mints a new MFA enrollment JWT with the given TTL.
func CreateMFAEnrollmentJWT(a *sessiontoken.TokenAuth, c sessiontoken.MFAEnrollmentClaims, ttl time.Duration) (string, error) {
	now := time.Now()
	c.IssuedAt = now.Unix()
	c.ExpiresAt = now.Add(ttl).Unix()

	claims := map[string]any{
		"account_id":             c.AccountID,
		"mfa_enrollment_pending": true,
		"iat":                    c.IssuedAt,
		"exp":                    c.ExpiresAt,
	}
	if c.Scope != "" {
		claims["scope"] = c.Scope
	}
	if c.TenantID != 0 {
		claims["tenant_id"] = c.TenantID
	}

	_, tokenString, err := a.JwtAuth.Encode(claims)
	return tokenString, err
}

// ParseMFAEnrollmentJWT decodes an enrollment token, extracts its claims into
// MFAEnrollmentClaims, and rejects expired tokens.
func ParseMFAEnrollmentJWT(a *sessiontoken.TokenAuth, tokenString string) (*sessiontoken.MFAEnrollmentClaims, error) {
	raw, err := decode(a, tokenString)
	if err != nil {
		return nil, err
	}
	var claims sessiontoken.MFAEnrollmentClaims
	if err := claims.ParseClaims(raw); err != nil {
		return nil, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("enrollment token expired")
	}
	return &claims, nil
}

// decode verifies the signature of tokenString and returns its claim map.
func decode(a *sessiontoken.TokenAuth, tokenString string) (map[string]any, error) {
	token, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]any)
	for _, key := range token.Keys() {
		var value any
		if token.Get(key, &value) == nil {
			raw[key] = value
		}
	}
	return raw, nil
}

func addScopedAppClaims(claims map[string]any, appClaims sessiontoken.AppClaims) {
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
	// Admin staff-view preview claims (#2893), explicit like the fields
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
}
