package jwt

import (
	"errors"
	"time"
)

// MFA enrollment scope values mirror the challenge scopes — same conventions,
// different stage in the flow. An enrollment token is issued when MFA is
// required for the tenant/operator role but the account has not yet
// registered a second factor; it must only authorize the /auth/mfa/enroll/*
// surface, never a fully authenticated route.
const (
	MFAEnrollmentScopeTenant   = "tenant"
	MFAEnrollmentScopePlatform = "platform"
	// MFAEnrollmentScopeSchool tags enrollment tokens minted by the school
	// portal login (#2207). Only the /school/auth/mfa/enroll/* handlers
	// accept it, and confirming there mints SCHOOL tokens — a school login
	// must never be laundered into a tenant session via the enrollment
	// detour.
	MFAEnrollmentScopeSchool = "school"
)

// MFAEnrollmentClaims represents a short-lived JWT issued to an account that
// passed the password check but has no MFA credential on a tenant that
// requires one. It is exchanged at /auth/mfa/enroll/confirm (or the operator
// equivalent) for a regular access/refresh token pair only AFTER the user
// successfully enrolls. The token is intentionally narrow:
//
//   - It carries account identity plus tenant/scope so the enroll handlers
//     can dispatch the challenge email and look up the active code.
//   - `mfa_enrollment_pending` is the discriminator middleware uses to
//     reject this token at any other endpoint — the security boundary that
//     closes the "issue full session before MFA is set up" bypass.
//
// The shape mirrors MFAChallengeClaims so the wire format is consistent and
// the two claim types stay parallel as new fields are added.
type MFAEnrollmentClaims struct {
	// AccountID is auth.accounts.id (tenant scope) or platform.operators.id
	// (platform scope) of the user being enrolled.
	AccountID int64 `json:"account_id"`

	// Scope distinguishes tenant from platform tokens, constrained to the
	// two MFAEnrollmentScope* values.
	Scope string `json:"scope,omitempty"`

	// TenantID is set on tenant-scope tokens so the enroll handlers can
	// wrap the StartChallenge call in the right tenant context. Zero on
	// platform tokens.
	TenantID int64 `json:"tenant_id,omitempty"`

	// MFAEnrollmentPending must be true on every enrollment token —
	// middleware (a) accepts the token only when it is set on the
	// MFA-enrollment authenticator and (b) rejects the token everywhere
	// else as defense-in-depth.
	MFAEnrollmentPending bool `json:"mfa_enrollment_pending"`

	CommonClaims
}

// ParseClaims fills MFAEnrollmentClaims from a decoded JWT claim map.
//
// Defense-in-depth (mirrors AppClaims.ParseClaims and the symmetric
// gate in MFAChallengeClaims): an enrollment token MUST NOT also carry
// mfa_pending=true — that would be a malformed JWT that satisfies both
// challenge and enrollment middleware. Rejecting the foreign flag up
// front means a future bug in CreateMFA*JWT can't accidentally produce
// a JWT both parsers accept. (#1430 review item #8)
func (c *MFAEnrollmentClaims) ParseClaims(claims map[string]any) error {
	accountID, tenantID, scope, err := parseMFAPendingClaims(claims, mfaPendingClaimsSpec{
		foreignFlagKey: "mfa_pending",
		foreignFlagErr: "token is a pending-MFA challenge, not an enrollment token",
		scopeTenant:    MFAEnrollmentScopeTenant,
		scopePlatform:  MFAEnrollmentScopePlatform,
		scopeSchool:    MFAEnrollmentScopeSchool,
		pendingFlagKey: "mfa_enrollment_pending",
		notPendingErr:  "token is not a pending-MFA-enrollment token",
	}, &c.CommonClaims)
	if err != nil {
		return err
	}
	c.AccountID = accountID
	c.TenantID = tenantID
	c.Scope = scope
	c.MFAEnrollmentPending = true
	return nil
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
