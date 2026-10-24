// Package tokenprimitives provides the existing Identity & Access token primitives.
package tokenprimitives

import (
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/lestrrat-go/jwx/v3/transform"
)

// Token is the native JWT token type, preserving its existing SDK identity.
type Token = jwt.Token

// SubjectKey is the same registered JWT subject claim key.
const SubjectKey = jwt.SubjectKey

// Validate applies the existing SDK claim validation without extra options.
// It does not verify a signature; callers retain their existing Decode step.
func Validate(token Token) error { return jwt.Validate(token) }

// QuotaIdentity is the stable numeric identity used by the request quota.
type QuotaIdentity struct {
	AccountID int64
	Scope     string
	TenantID  int64
}

// RateLimitIdentity retains native claim validation, decoding and numeric acceptance.
// The caller must first verify the token signature, as RateLimitKey does with Decode.
func RateLimitIdentity(token Token) (QuotaIdentity, bool) {
	if err := jwt.Validate(token); err != nil {
		return QuotaIdentity{}, false
	}
	claims := map[string]any{}
	if err := transform.AsMap(token, claims); err != nil {
		return QuotaIdentity{}, false
	}
	accountID, ok := claims["id"].(float64)
	if !ok || accountID <= 0 {
		return QuotaIdentity{}, false
	}
	scope, _ := claims["scope"].(string)
	tenantID, _ := claims["tenant_id"].(float64)
	return QuotaIdentity{AccountID: int64(accountID), Scope: scope, TenantID: int64(tenantID)}, true
}
