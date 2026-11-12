package tokenprimitives

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwt"
)

// The wire schema of the identity tokens. Every writer and reader of access,
// refresh, MFA challenge and MFA enrollment tokens goes through this file:
// the Identity & Access codec signs and parses with it, the request
// middleware reads with it, and the test mints use it. A writer signs the
// JSON of a *Wire value; a reader decodes a verified Token.

// Token is a verified JWT. The HTTP role names it through this alias, since
// it may not import the JWT library itself.
type Token = jwt.Token

// AccessClaims is the content of an access token. The field list matches the
// session claims of Identity & Access, so the owner converts between them.
type AccessClaims struct {
	AccountID     int64
	Email         string
	Username      string
	FirstName     string
	LastName      string
	Roles         []string
	RoleIsSystem  []bool
	Permissions   []string
	IsAdmin       bool
	Scope         string
	TenantID      int64
	OrgID         int64
	FamilyID      string
	ReadOnly      bool
	ActingAdminID int64
	PreviewID     string
	ViaTraeger    bool
	ExpiresAt     int64
}

// RefreshClaims is the content of a refresh token.
type RefreshClaims struct {
	AccountID int64
	Token     string
	TenantID  int64
	OrgID     int64
	Scope     string
	FamilyID  string
	LineageID string
	ExpiresAt int64
}

// ChallengeClaims is the content of an MFA challenge token.
// CredentialGeneration is the account's credential generation its code was
// issued under; a resend is bound to it (#6289).
type ChallengeClaims struct {
	AccountID            int64
	Scope                string
	TenantID             int64
	ChallengeID          int64
	CredentialGeneration int64
}

// EnrollmentClaims is the content of an MFA enrollment token.
// CredentialGeneration is the account's credential generation the password
// sign-in checked; the enrollment start is bound to it (#6289).
type EnrollmentClaims struct {
	AccountID            int64
	Scope                string
	TenantID             int64
	ExpiresAt            int64
	CredentialGeneration int64
}

// AccessWire is the signed form of an access token. Roles, system flags and
// permissions are always present; other claims only when set.
type AccessWire struct {
	ID            int64    `json:"id,omitempty"`
	Sub           string   `json:"sub,omitempty"`
	Username      string   `json:"username,omitempty"`
	FirstName     string   `json:"first_name,omitempty"`
	LastName      string   `json:"last_name,omitempty"`
	Roles         []string `json:"roles"`
	RoleIsSystem  []bool   `json:"role_is_system"`
	Permissions   []string `json:"permissions"`
	IsAdmin       bool     `json:"is_admin,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	TenantID      int64    `json:"tenant_id,omitempty"`
	OrgID         int64    `json:"org_id,omitempty"`
	FamilyID      string   `json:"family_id,omitempty"`
	ReadOnly      bool     `json:"read_only,omitempty"`
	ActingAdminID int64    `json:"acting_admin_id,omitempty"`
	PreviewID     string   `json:"preview_id,omitempty"`
	ViaTraeger    bool     `json:"via_traeger,omitempty"`
	IssuedAt      int64    `json:"iat"`
	ExpiresAt     int64    `json:"exp"`
}

// RefreshWire is the signed form of a refresh token.
type RefreshWire struct {
	ID        int64  `json:"id,omitempty"`
	Token     string `json:"token,omitempty"`
	TenantID  int64  `json:"tenant_id,omitempty"`
	OrgID     int64  `json:"org_id,omitempty"`
	Scope     string `json:"scope,omitempty"`
	FamilyID  string `json:"family_id,omitempty"`
	LineageID string `json:"lineage_id,omitempty"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// ChallengeWire is the signed form of an MFA challenge token.
type ChallengeWire struct {
	AccountID   int64  `json:"account_id"`
	Scope       string `json:"scope,omitempty"`
	TenantID    int64  `json:"tenant_id,omitempty"`
	ChallengeID int64  `json:"challenge_id,omitempty"`
	// Generation is written even when zero: a missing claim reads as zero,
	// which binds a token from before the claim existed to generation 0.
	Generation int64 `json:"credential_generation"`
	MFAPending bool  `json:"mfa_pending"`
	IssuedAt   int64 `json:"iat"`
	ExpiresAt  int64 `json:"exp"`
}

// EnrollmentWire is the signed form of an MFA enrollment token.
type EnrollmentWire struct {
	AccountID            int64  `json:"account_id"`
	Scope                string `json:"scope"`
	TenantID             int64  `json:"tenant_id,omitempty"`
	Generation           int64  `json:"credential_generation"`
	MFAEnrollmentPending bool   `json:"mfa_enrollment_pending"`
	IssuedAt             int64  `json:"iat"`
	ExpiresAt            int64  `json:"exp"`
}

const (
	pendingChallengeFlag  = "mfa_pending"
	pendingEnrollmentFlag = "mfa_enrollment_pending"
)

// EncodeAccess is the wire form of an access token.
func EncodeAccess(c AccessClaims, issuedAt, expiresAt time.Time) AccessWire {
	return AccessWire{
		ID: c.AccountID, Sub: c.Email, Username: c.Username, FirstName: c.FirstName, LastName: c.LastName,
		Roles: c.Roles, RoleIsSystem: c.RoleIsSystem, Permissions: c.Permissions, IsAdmin: c.IsAdmin, Scope: c.Scope,
		TenantID: c.TenantID, OrgID: c.OrgID, FamilyID: c.FamilyID, ReadOnly: c.ReadOnly, ActingAdminID: c.ActingAdminID,
		PreviewID: c.PreviewID, ViaTraeger: c.ViaTraeger, IssuedAt: issuedAt.Unix(), ExpiresAt: expiresAt.Unix(),
	}
}

// EncodeRefresh is the wire form of a refresh token. A token without its own
// expiry gets defaultExpiry.
func EncodeRefresh(c RefreshClaims, issuedAt, defaultExpiry time.Time) RefreshWire {
	expiry := c.ExpiresAt
	if expiry <= 0 {
		expiry = defaultExpiry.Unix()
	}
	return RefreshWire{
		ID: c.AccountID, Token: c.Token, TenantID: c.TenantID, OrgID: c.OrgID, Scope: c.Scope,
		FamilyID: c.FamilyID, LineageID: c.LineageID, IssuedAt: issuedAt.Unix(), ExpiresAt: expiry,
	}
}

// EncodeChallenge is the wire form of an MFA challenge token.
func EncodeChallenge(c ChallengeClaims, issuedAt time.Time, ttl time.Duration) ChallengeWire {
	return ChallengeWire{
		AccountID: c.AccountID, Scope: c.Scope, TenantID: c.TenantID, ChallengeID: c.ChallengeID,
		Generation: c.CredentialGeneration, MFAPending: true, IssuedAt: issuedAt.Unix(), ExpiresAt: issuedAt.Add(ttl).Unix(),
	}
}

// EncodeEnrollment is the wire form of an MFA enrollment token. Only a portal
// with an enrollment of its own has one.
func EncodeEnrollment(c EnrollmentClaims, issuedAt time.Time, ttl time.Duration) (EnrollmentWire, error) {
	if portal, ok := PortalNamed(c.Scope); !ok || !portal.Enrollment {
		return EnrollmentWire{}, fmt.Errorf("portal %q has no mfa enrollment", c.Scope)
	}
	return EnrollmentWire{
		AccountID: c.AccountID, Scope: c.Scope, TenantID: c.TenantID, Generation: c.CredentialGeneration,
		MFAEnrollmentPending: true, IssuedAt: issuedAt.Unix(), ExpiresAt: issuedAt.Add(ttl).Unix(),
	}, nil
}

// Verified is the one parse of a token the request verifier accepted: the
// token read as each of the shapes it might have.
type Verified struct {
	// Expired: the token no longer validates. The verifier refuses such a
	// token already, so this holds only for a token placed without it.
	Expired       bool
	Access        AccessClaims
	AccessErr     error
	Refresh       RefreshClaims
	RefreshErr    error
	Enrollment    EnrollmentClaims
	EnrollmentErr error
}

// DecodeVerified reads a verified token as access, refresh and enrollment
// token at once.
func DecodeVerified(token Token, now time.Time) Verified {
	verified := Verified{Expired: jwt.Validate(token) != nil}
	wire, err := wireClaims(token)
	if err != nil {
		verified.AccessErr, verified.RefreshErr, verified.EnrollmentErr = err, err, err
		return verified
	}
	verified.Access, verified.AccessErr = decodeAccess(wire)
	verified.Refresh, verified.RefreshErr = decodeRefresh(wire)
	verified.Enrollment, verified.EnrollmentErr = decodeEnrollment(wire, now)
	return verified
}

// DecodeAccess reads an access token.
func DecodeAccess(token Token) (AccessClaims, error) {
	wire, err := wireClaims(token)
	if err != nil {
		return AccessClaims{}, err
	}
	return decodeAccess(wire)
}

// DecodeRefresh reads a refresh token.
func DecodeRefresh(token Token) (RefreshClaims, error) {
	wire, err := wireClaims(token)
	if err != nil {
		return RefreshClaims{}, err
	}
	return decodeRefresh(wire)
}

// DecodeChallenge reads an MFA challenge token of a portal with a challenge
// of its own.
func DecodeChallenge(token Token, now time.Time) (ChallengeClaims, error) {
	wire, err := wireClaims(token)
	if err != nil {
		return ChallengeClaims{}, err
	}
	return decodeChallenge(wire, now)
}

// wireClaims returns the claims of a token as JSON values: numbers are
// float64 and exp is a unix time.
func wireClaims(token Token) (map[string]any, error) {
	encoded, err := json.Marshal(token)
	if err != nil {
		return nil, err
	}
	var claims map[string]any
	if err := json.Unmarshal(encoded, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// decodeAccess refuses challenge and enrollment tokens even if they carry
// every access claim.
func decodeAccess(wire map[string]any) (AccessClaims, error) {
	if err := rejectPending(wire, "session"); err != nil {
		return AccessClaims{}, err
	}
	id, err := requiredNumber(wire, "id")
	if err != nil {
		return AccessClaims{}, err
	}
	email, err := requiredString(wire, "sub")
	if err != nil {
		return AccessClaims{}, err
	}
	roles, err := requiredRoles(wire)
	if err != nil {
		return AccessClaims{}, err
	}
	return AccessClaims{
		AccountID: id, Email: email, Roles: roles, RoleIsSystem: optionalBools(wire["role_is_system"]), Permissions: optionalStrings(wire["permissions"]),
		Username: text(wire, "username"), FirstName: text(wire, "first_name"), LastName: text(wire, "last_name"),
		IsAdmin: flag(wire, "is_admin"), Scope: text(wire, "scope"), TenantID: number(wire, "tenant_id"), OrgID: number(wire, "org_id"),
		FamilyID: text(wire, "family_id"), ReadOnly: flag(wire, "read_only"), ActingAdminID: number(wire, "acting_admin_id"), PreviewID: text(wire, "preview_id"),
		ViaTraeger: flag(wire, "via_traeger"), ExpiresAt: number(wire, "exp"),
	}, nil
}

// decodeRefresh refuses challenge, enrollment and read-only preview tokens;
// a preview is access-only (#2893).
func decodeRefresh(wire map[string]any) (RefreshClaims, error) {
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
	return RefreshClaims{
		AccountID: id, Token: value, TenantID: number(wire, "tenant_id"), OrgID: number(wire, "org_id"), Scope: text(wire, "scope"),
		FamilyID: text(wire, "family_id"), LineageID: text(wire, "lineage_id"), ExpiresAt: number(wire, "exp"),
	}, nil
}

func decodeChallenge(wire map[string]any, now time.Time) (ChallengeClaims, error) {
	accountID, scope, err := decodePendingMFA(wire, pendingSpec{
		foreign: pendingEnrollmentFlag, foreignErr: "token is a pending-MFA-enrollment token, not a challenge",
		own: pendingChallengeFlag, notOwnErr: "token is not a pending-MFA challenge", expiredErr: "challenge token expired",
		accepts: func(p PortalClaims) bool { return p.Challenge },
	}, now)
	if err != nil {
		return ChallengeClaims{}, err
	}
	return ChallengeClaims{
		AccountID: accountID, Scope: scope, TenantID: number(wire, "tenant_id"), ChallengeID: number(wire, "challenge_id"),
		CredentialGeneration: number(wire, "credential_generation"),
	}, nil
}

func decodeEnrollment(wire map[string]any, now time.Time) (EnrollmentClaims, error) {
	accountID, scope, err := decodePendingMFA(wire, pendingSpec{
		foreign: pendingChallengeFlag, foreignErr: "token is a pending-MFA challenge, not an enrollment token",
		own: pendingEnrollmentFlag, notOwnErr: "token is not a pending-MFA-enrollment token", expiredErr: "enrollment token expired",
		accepts: func(p PortalClaims) bool { return p.Enrollment },
	}, now)
	if err != nil {
		return EnrollmentClaims{}, err
	}
	return EnrollmentClaims{
		AccountID: accountID, Scope: scope, TenantID: number(wire, "tenant_id"), ExpiresAt: number(wire, "exp"),
		CredentialGeneration: number(wire, "credential_generation"),
	}, nil
}

// pendingSpec holds what differs between challenge and enrollment tokens.
// The error texts reach 401 bodies and stay verbatim.
type pendingSpec struct {
	foreign, foreignErr string
	own, notOwnErr      string
	expiredErr          string
	accepts             func(PortalClaims) bool
}

// decodePendingMFA refuses a token that carries the other pending flag, so
// no token satisfies both the verify and the enroll routes (#1430).
func decodePendingMFA(wire map[string]any, spec pendingSpec, now time.Time) (int64, string, error) {
	if flag(wire, spec.foreign) {
		return 0, "", errors.New(spec.foreignErr)
	}
	if flag(wire, holidayParentSetupFlag) {
		return 0, "", errors.New("token is a holiday parent setup token, not a pending-MFA token")
	}
	accountID := number(wire, "account_id")
	if accountID == 0 {
		return 0, "", errors.New("missing required claim: account_id")
	}
	scope := text(wire, "scope")
	if portal, ok := PortalNamed(scope); !ok || !spec.accepts(portal) {
		return 0, "", fmt.Errorf("invalid scope claim: %q", scope)
	}
	if !flag(wire, spec.own) {
		return 0, "", errors.New(spec.notOwnErr)
	}
	if expiry := number(wire, "exp"); expiry > 0 && expiry < now.Unix() {
		return 0, "", errors.New(spec.expiredErr)
	}
	return accountID, scope, nil
}

func rejectPending(wire map[string]any, kind string) error {
	if flag(wire, holidayParentSetupFlag) {
		return fmt.Errorf("token is a holiday parent setup token, not a %s token", kind)
	}
	if flag(wire, pendingChallengeFlag) {
		return fmt.Errorf("token is a pending-MFA challenge, not a %s token", kind)
	}
	if flag(wire, pendingEnrollmentFlag) {
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
	n, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("claim %s is not a number", key)
	}
	return int64(n), nil
}

func requiredString(wire map[string]any, key string) (string, error) {
	value, exists := wire[key]
	if !exists {
		return "", fmt.Errorf("missing required claim: %s", key)
	}
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("claim %s is not a string", key)
	}
	return s, nil
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
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func optionalBools(value any) []bool {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]bool, 0, len(items))
	for _, item := range items {
		if b, ok := item.(bool); ok {
			result = append(result, b)
		}
	}
	return result
}
