package jwt

import (
	"errors"
	"fmt"
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
func (c *MFAChallengeClaims) ParseClaims(claims map[string]any) error {
	c.AccountID = getOptionalInt64(claims, "account_id")
	if c.AccountID == 0 {
		return errors.New("missing required claim: account_id")
	}

	c.Scope = getOptionalString(claims, "scope")
	if c.Scope != MFAChallengeScopeTenant && c.Scope != MFAChallengeScopePlatform {
		return fmt.Errorf("invalid scope claim: %q", c.Scope)
	}

	c.TenantID = getOptionalInt64(claims, "tenant_id")
	c.MFAPending = getOptionalBool(claims, "mfa_pending")
	if !c.MFAPending {
		return errors.New("token is not a pending-MFA challenge")
	}

	if exp, ok := claims["exp"]; ok {
		switch v := exp.(type) {
		case float64:
			c.ExpiresAt = int64(v)
		case int64:
			c.ExpiresAt = v
		case time.Time:
			c.ExpiresAt = v.Unix()
		}
	}
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
