package domain

import (
	"errors"
	"fmt"
	"time"
)

// SessionProvesInvitationOwnership excludes delegated and unfinished sessions.
// Parent sessions are global; staff and school sessions must name their school.
func SessionProvesInvitationOwnership(claims SessionClaims, now time.Time) bool {
	if claims.AccountID <= 0 || claims.ExpiresAt <= now.Unix() || claims.ReadOnly || claims.ActingAdminID != 0 || claims.PreviewID != "" {
		return false
	}
	switch claims.Scope {
	case "", "tenant", "org", "school":
		return claims.TenantID > 0
	case "parent":
		return true
	default:
		return false
	}
}

// DecodeMFAChallengeToken enforces the challenge/enrollment separation and scope.
func DecodeMFAChallengeToken(wire map[string]any, now time.Time) (MFAChallengeClaims, error) {
	if flag(wire, "mfa_enrollment_pending") {
		return MFAChallengeClaims{}, errors.New("token is a pending-MFA-enrollment token, not a challenge")
	}
	accountID := number(wire, "account_id")
	if accountID == 0 {
		return MFAChallengeClaims{}, errors.New("missing required claim: account_id")
	}
	scope := text(wire, "scope")
	if scope != "tenant" && scope != "platform" && scope != "school" {
		return MFAChallengeClaims{}, fmt.Errorf("invalid scope claim: %q", scope)
	}
	if !flag(wire, "mfa_pending") {
		return MFAChallengeClaims{}, errors.New("token is not a pending-MFA challenge")
	}
	if expiry := number(wire, "exp"); expiry > 0 && expiry < now.Unix() {
		return MFAChallengeClaims{}, errors.New("challenge token expired")
	}
	return MFAChallengeClaims{AccountID: accountID, Scope: scope, TenantID: number(wire, "tenant_id"), ChallengeID: number(wire, "challenge_id")}, nil
}

// DecodeSessionToken validates the access-token wire shape after signature verification.
func DecodeSessionToken(wire map[string]any) (SessionClaims, error) {
	if err := rejectPending(wire, "session"); err != nil {
		return SessionClaims{}, err
	}
	id, err := requiredNumber(wire, "id")
	if err != nil {
		return SessionClaims{}, err
	}
	email, err := requiredString(wire, "sub")
	if err != nil {
		return SessionClaims{}, err
	}
	roles, err := requiredRoles(wire)
	if err != nil {
		return SessionClaims{}, err
	}
	return SessionClaims{
		AccountID: id, Email: email, Roles: roles, Permissions: optionalStrings(wire["permissions"]),
		Username: text(wire, "username"), FirstName: text(wire, "first_name"), LastName: text(wire, "last_name"),
		IsAdmin: flag(wire, "is_admin"), Scope: text(wire, "scope"), TenantID: number(wire, "tenant_id"), OrgID: number(wire, "org_id"),
		FamilyID: text(wire, "family_id"), ReadOnly: flag(wire, "read_only"), ActingAdminID: number(wire, "acting_admin_id"), PreviewID: text(wire, "preview_id"),
		ExpiresAt: number(wire, "exp"),
	}, nil
}

// DecodeRefreshToken validates refresh identity and refuses MFA and preview tokens.
func DecodeRefreshToken(wire map[string]any) (RefreshClaims, error) {
	if err := rejectPending(wire, "refresh"); err != nil {
		return RefreshClaims{}, err
	}
	if flag(wire, "read_only") {
		return RefreshClaims{}, errors.New("token is a read-only preview token, not a refresh token")
	}
	id, err := requiredNumber(wire, "id")
	if err != nil {
		return RefreshClaims{}, err
	}
	value, err := requiredString(wire, "token")
	if err != nil {
		return RefreshClaims{}, err
	}
	return RefreshClaims{AccountID: id, Token: value, TenantID: number(wire, "tenant_id"), Scope: text(wire, "scope"), ExpiresAt: number(wire, "exp")}, nil
}

func rejectPending(wire map[string]any, kind string) error {
	if flag(wire, "mfa_pending") {
		return fmt.Errorf("token is a pending-MFA challenge, not a %s token", kind)
	}
	if flag(wire, "mfa_enrollment_pending") {
		return fmt.Errorf("token is a pending-MFA-enrollment token, not a %s token", kind)
	}
	return nil
}

func text(wire map[string]any, key string) string { value, _ := wire[key].(string); return value }
func flag(wire map[string]any, key string) bool   { value, _ := wire[key].(bool); return value }
func number(wire map[string]any, key string) int64 {
	value, _ := wire[key].(float64)
	return int64(value)
}

func requiredNumber(wire map[string]any, key string) (int64, error) {
	value, exists := wire[key]
	if !exists {
		return 0, fmt.Errorf("missing required claim: %s", key)
	}
	number, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("claim %s is not a number", key)
	}
	return int64(number), nil
}

func requiredString(wire map[string]any, key string) (string, error) {
	value, exists := wire[key]
	if !exists {
		return "", fmt.Errorf("missing required claim: %s", key)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("claim %s is not a string", key)
	}
	return text, nil
}

func requiredRoles(wire map[string]any) ([]string, error) {
	value, exists := wire["roles"]
	if !exists {
		return nil, errors.New("missing required claim: roles")
	}
	items, ok := value.([]any)
	if !ok {
		return nil, errors.New("claim roles: value is not an array")
	}
	roles := make([]string, 0, len(items))
	for i, item := range items {
		role, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("claim roles: element %d is not a string", i)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func optionalStrings(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}
