package jwt

import (
	"encoding/json"
	"errors"
	"time"
)

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

// GetRefreshExpiry returns the refresh token expiration duration
func (a *TokenAuth) GetRefreshExpiry() time.Duration {
	return a.JwtRefreshExpiry
}

// ParseClaims fills MFAChallengeClaims from a decoded JWT claim map.
//
// Defense-in-depth (symmetric to MFAEnrollmentClaims): a challenge token
// MUST NOT also carry mfa_enrollment_pending=true. Rejecting the foreign
// flag up front means a malformed JWT can't satisfy both /auth/mfa/verify
// and /auth/mfa/enroll/* middlewares. (#1430 review item #8)
func (c *MFAChallengeClaims) ParseClaims(claims map[string]any) error {
	accountID, tenantID, scope, err := parseMFAPendingClaims(claims, mfaPendingClaimsSpec{
		foreignFlagKey: "mfa_enrollment_pending",
		foreignFlagErr: "token is a pending-MFA-enrollment token, not a challenge",
		scopeTenant:    MFAChallengeScopeTenant,
		scopePlatform:  MFAChallengeScopePlatform,
		scopeSchool:    MFAChallengeScopeSchool,
		pendingFlagKey: "mfa_pending",
		notPendingErr:  "token is not a pending-MFA challenge",
	}, &c.CommonClaims)
	if err != nil {
		return err
	}
	c.AccountID = accountID
	c.TenantID = tenantID
	c.Scope = scope
	c.ChallengeID = getOptionalInt64(claims, "challenge_id")
	c.MFAPending = true
	return nil
}

// CreateMFAChallengeJWT mints a new MFA challenge JWT with the given TTL.
// Callers are responsible for passing a sane TTL (typically 5 minutes).
func (a *TokenAuth) CreateMFAChallengeJWT(c MFAChallengeClaims, ttl time.Duration) (string, error) {
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

// ParseMFAChallengeJWT decodes an MFA challenge token, extracts its
// claims into MFAChallengeClaims, and rejects expired tokens. Used by
// both the tenant- and operator-side MFA verification flows — the
// service-layer wrappers used to inline this logic, but the loop was
// identical in both, so it lives here once.
func (a *TokenAuth) ParseMFAChallengeJWT(tokenString string) (*MFAChallengeClaims, error) {
	jwtToken, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]any)
	for _, k := range jwtToken.Keys() {
		var v any
		if jwtToken.Get(k, &v) == nil {
			raw[k] = v
		}
	}
	var claims MFAChallengeClaims
	if err := claims.ParseClaims(raw); err != nil {
		return nil, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("challenge token expired")
	}
	return &claims, nil
}

// CreateMFAEnrollmentJWT mints a new MFA enrollment JWT with the given TTL.
// Callers pick the TTL based on how long the enrollment flow may take.
// Recommended: same window as a regular access token (15 minutes) — long
// enough for a real user to retrieve the emailed code, short enough that
// a forgotten enrollment session expires quickly.
func (a *TokenAuth) CreateMFAEnrollmentJWT(c MFAEnrollmentClaims, ttl time.Duration) (string, error) {
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

// ParseMFAEnrollmentJWT decodes an enrollment token, extracts its claims
// into MFAEnrollmentClaims, and rejects expired tokens. Mirrors
// ParseMFAChallengeJWT — the verify path on the enrollment authenticator
// uses this for the same reason: the loop would otherwise duplicate in
// every consumer.
func (a *TokenAuth) ParseMFAEnrollmentJWT(tokenString string) (*MFAEnrollmentClaims, error) {
	jwtToken, err := a.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, err
	}
	raw := make(map[string]any)
	for _, k := range jwtToken.Keys() {
		var v any
		if jwtToken.Get(k, &v) == nil {
			raw[k] = v
		}
	}
	var claims MFAEnrollmentClaims
	if err := claims.ParseClaims(raw); err != nil {
		return nil, err
	}
	if claims.ExpiresAt > 0 && claims.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("enrollment token expired")
	}
	return &claims, nil
}
