package jwt

import (
	"errors"
	"time"
)

// MFA challenge scope values — distinguish tenant accounts from platform operators.
const (
	MFAChallengeScopeTenant   = "tenant"
	MFAChallengeScopePlatform = "platform"
)

// MFAChallengeClaims represents a short-lived JWT issued after a successful
// password check on an account that requires a second factor. It is exchanged
// at /auth/mfa/verify (or the operator equivalent) for a regular access /
// refresh token pair. The token is intentionally narrow: it carries the
// account / operator identity needed to look up the challenge row and an
// `mfa_pending` flag so middleware never confuses it with an authenticated
// session token.
type MFAChallengeClaims struct {
	// AccountID is the auth.accounts.id (tenant scope) or
	// platform.operators.id (platform scope) of the user being challenged.
	AccountID int64 `json:"account_id"`

	// Scope distinguishes tenant from platform tokens — same conventions as
	// AppClaims.Scope, but constrained to the two MFAChallengeScope* values.
	Scope string `json:"scope,omitempty"`

	// TenantID is set on tenant-scope tokens so the verify handler can wrap
	// repository calls in the correct tenant tx. Zero on platform tokens.
	TenantID int64 `json:"tenant_id,omitempty"`

	// MFAPending must be true on every challenge token — middleware uses this
	// to reject challenge tokens at endpoints that expect a fully
	// authenticated session.
	MFAPending bool `json:"mfa_pending"`

	CommonClaims
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
		pendingFlagKey: "mfa_pending",
		notPendingErr:  "token is not a pending-MFA challenge",
	}, &c.CommonClaims)
	if err != nil {
		return err
	}
	c.AccountID = accountID
	c.TenantID = tenantID
	c.Scope = scope
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

	_, tokenString, err := a.JwtAuth.Encode(claims)
	return tokenString, err
}

// ParseMFAChallengeJWT decodes an MFA challenge token, extracts its
// claims into MFAChallengeClaims, and rejects expired tokens. Used by
// both the tenant- and operator-side MFA verification flows — the
// service-layer wrappers used to inline this logic, but the loop was
// identical and flagged by SonarCloud as duplication.
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
