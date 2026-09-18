package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwt"
)

type CommonClaims struct {
	ExpiresAt int64 `json:"exp,omitempty"`
	IssuedAt  int64 `json:"iat,omitempty"`
}

// AppClaims represent the claims parsed from JWT access token.
type AppClaims struct {
	ID          int      `json:"id,omitempty"`
	Sub         string   `json:"sub,omitempty"`
	Username    string   `json:"username,omitempty"`
	FirstName   string   `json:"first_name,omitempty"`
	LastName    string   `json:"last_name,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"` // Added permissions field
	// Static role flags for quick access
	IsAdmin bool `json:"is_admin,omitempty"`
	// Scope distinguishes tenant tokens from platform tokens
	// "platform" = operator tokens (moto DevOps team)
	// "" or "tenant" = regular user tokens
	Scope string `json:"scope,omitempty"`
	// Multi-tenancy fields (Phase 1)
	TenantID int64 `json:"tenant_id,omitempty"` // School ID (tenant boundary)
	OrgID    int64 `json:"org_id,omitempty"`    // Organization ID
	// FamilyID is the refresh-token family that minted this access token.
	// Push subscriptions stamp it so family-scoped logout can revoke the
	// same device's server-side subscription.
	FamilyID string `json:"family_id,omitempty"`
	// ReadOnly marks an admin staff-view preview token (#2893): the token
	// carries the TARGET account's identity, roles, and permissions so every
	// read evaluates exactly as that person, while
	// common.ReadOnlyPreviewMiddleware blocks every write. Preview tokens are
	// access-only — no refresh token is ever minted in the target's name.
	ReadOnly bool `json:"read_only,omitempty"`
	// ActingAdminID is the admin account that started the read-only preview.
	// Zero on every regular token. Kept in the claims so the preview can
	// never be mistaken for a real session of the target account.
	ActingAdminID int64 `json:"acting_admin_id,omitempty"`
	// PreviewID identifies ONE preview instance (#2893). It is created when
	// the admin opens the preview and carried unchanged through every
	// re-mint, so the audit trail can pair exactly one start with exactly one
	// end no matter how often the token was renewed. Empty on regular tokens.
	PreviewID string `json:"preview_id,omitempty"`
	CommonClaims
}

// IsReadOnlyPreview reports whether this token is an admin staff-view
// preview token (#2893). Such tokens are read-only by contract.
func (c *AppClaims) IsReadOnlyPreview() bool {
	return c.ReadOnly
}

// IsPlatformScope returns true if this is a platform/operator token
func (c *AppClaims) IsPlatformScope() bool {
	return c.Scope == "platform"
}

// IsSchoolScope returns true if this is a school-portal token ("moto schule",
// #2207). School tokens are tenant-bound like regular tenant tokens, so the
// scope is the ONLY thing that distinguishes them once the tenant transaction
// is open — which is why authorization rules that must not apply to a
// Lehrkraft ask this question instead of inspecting roles (#2527).
func (c *AppClaims) IsSchoolScope() bool {
	return c.Scope == "school"
}

// Error format for missing claims
const errMissingClaim = "missing required claim: %s"

// Helper functions for safe claim extraction

func getRequiredInt(claims map[string]any, key string) (int, error) {
	val, ok := claims[key]
	if !ok {
		return 0, fmt.Errorf(errMissingClaim, key)
	}
	f, ok := val.(float64)
	if !ok {
		return 0, fmt.Errorf("claim %s is not a number", key)
	}
	return int(f), nil
}

func getRequiredString(claims map[string]any, key string) (string, error) {
	val, ok := claims[key]
	if !ok {
		return "", fmt.Errorf(errMissingClaim, key)
	}
	s, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("claim %s is not a string", key)
	}
	return s, nil
}

func getOptionalString(claims map[string]any, key string) string {
	val, ok := claims[key]
	if !ok || val == nil {
		return ""
	}
	s, _ := val.(string)
	return s
}

func getOptionalBool(claims map[string]any, key string) bool {
	val, ok := claims[key]
	if !ok || val == nil {
		return false
	}
	b, _ := val.(bool)
	return b
}

func getOptionalInt64(claims map[string]any, key string) int64 {
	val, ok := claims[key]
	if !ok || val == nil {
		return 0
	}
	// JSON numbers are decoded as float64
	f, ok := val.(float64)
	if !ok {
		return 0
	}
	return int64(f)
}

// expiryFromClaims reads the "exp" claim as a unix timestamp. The decoder
// hands it over as a number for a raw claim map and as a time.Time for a
// token decoded by jwx, so every caller has to handle both shapes; 0 means
// the token carries no expiry.
func expiryFromClaims(claims map[string]any) int64 {
	exp, ok := claims["exp"]
	if !ok {
		return 0
	}
	switch v := exp.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case time.Time:
		return v.Unix()
	}
	return 0
}

func getRequiredStringSlice(claims map[string]any, key string) ([]string, error) {
	val, ok := claims[key]
	if !ok {
		return nil, fmt.Errorf(errMissingClaim, key)
	}
	// Use strict validation for required claims - reject malformed arrays
	result, err := toStringSliceStrict(val)
	if err != nil {
		return nil, fmt.Errorf("claim %s: %w", key, err)
	}
	return result, nil
}

func getOptionalStringSlice(claims map[string]any, key string) []string {
	val, ok := claims[key]
	if !ok || val == nil {
		return []string{}
	}
	// Use lenient parsing for optional claims - skip invalid elements gracefully
	return toStringSliceLenient(val)
}

// toStringSliceStrict converts an interface slice to string slice.
// Returns an error if any element is not a string (for required claims).
func toStringSliceStrict(val any) ([]string, error) {
	slice, ok := val.([]any)
	if !ok {
		return nil, errors.New("value is not an array")
	}
	result := make([]string, 0, len(slice))
	for i, v := range slice {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("element %d is not a string", i)
		}
		result = append(result, s)
	}
	return result, nil
}

// toStringSliceLenient converts an interface slice to string slice.
// Silently skips non-string elements (for optional claims).
func toStringSliceLenient(val any) []string {
	slice, ok := val.([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ParseClaims parses JWT claims into AppClaims.
// Uses safe type assertions to prevent panics from malformed tokens.
//
// Defense-in-depth: tokens that carry an MFA-pending or
// MFA-enrollment-pending flag are rejected here even when the rest of the
// AppClaims fields happen to be set. Without this gate, a token issued for
// the narrow MFA-challenge or enrollment surface could be replayed against
// any fully authenticated route if a future change ever fills in id/sub/
// roles on those tokens.
func (c *AppClaims) ParseClaims(claims map[string]any) error {
	if getOptionalBool(claims, "mfa_pending") {
		return errors.New("token is a pending-MFA challenge, not a session token")
	}
	if getOptionalBool(claims, "mfa_enrollment_pending") {
		return errors.New("token is a pending-MFA-enrollment token, not a session token")
	}

	var err error

	c.ID, err = getRequiredInt(claims, "id")
	if err != nil {
		return err
	}

	c.Sub, err = getRequiredString(claims, jwt.SubjectKey)
	if err != nil {
		return err
	}

	c.Username = getOptionalString(claims, "username")
	c.FirstName = getOptionalString(claims, "first_name")
	c.LastName = getOptionalString(claims, "last_name")

	c.Roles, err = getRequiredStringSlice(claims, "roles")
	if err != nil {
		return err
	}

	c.Permissions = getOptionalStringSlice(claims, "permissions")
	c.IsAdmin = getOptionalBool(claims, "is_admin")
	c.Scope = getOptionalString(claims, "scope")
	c.TenantID = getOptionalInt64(claims, "tenant_id")
	c.OrgID = getOptionalInt64(claims, "org_id")
	c.FamilyID = getOptionalString(claims, "family_id")
	c.ReadOnly = getOptionalBool(claims, "read_only")
	c.ActingAdminID = getOptionalInt64(claims, "acting_admin_id")
	c.PreviewID = getOptionalString(claims, "preview_id")

	return nil
}

// RefreshClaims represents the claims parsed from JWT refresh token.
//
// Scope mirrors AppClaims.Scope so the refresh path can re-mint the
// access token in the right shape — e.g. a parent-scope token must
// stay parent-scope across refresh, not silently get re-issued as a
// tenant-scope token (which would break the parents portal).
type RefreshClaims struct {
	ID       int    `json:"id,omitempty"`
	Token    string `json:"token,omitempty"`
	TenantID int64  `json:"tenant_id,omitempty"` // Multi-tenancy: School ID
	Scope    string `json:"scope,omitempty"`     // "", "org", "platform", "parent", "school"
	CommonClaims
}

// ParseClaims parses the JWT claims into RefreshClaims.
//
// Defense-in-depth (mirrors AppClaims.ParseClaims): refresh-token parsing
// MUST reject tokens that carry an MFA-pending or MFA-enrollment-pending
// flag, even if the rest of the RefreshClaims fields happen to be set.
// Today's challenge / enrollment JWTs don't include `id`/`token`, so the
// parse would fail at the next check anyway — but that's incidental, not
// deliberate. If a future change ever extends the challenge/enrollment
// shape, this explicit gate keeps those tokens from being accepted by the
// /auth/refresh route. (#1430 review item #8)
func (c *RefreshClaims) ParseClaims(claims map[string]any) error {
	if getOptionalBool(claims, "mfa_pending") {
		return errors.New("token is a pending-MFA challenge, not a refresh token")
	}
	if getOptionalBool(claims, "mfa_enrollment_pending") {
		return errors.New("token is a pending-MFA-enrollment token, not a refresh token")
	}
	// Preview tokens are access-only by contract (#2893). Today they carry no
	// `token` claim so the parse below would fail anyway — this gate keeps
	// that deliberate, not incidental, mirroring the MFA gates above.
	if getOptionalBool(claims, "read_only") {
		return errors.New("token is a read-only preview token, not a refresh token")
	}

	// Parse ID field
	id, ok := claims["id"]
	if !ok {
		return errors.New("could not parse claim id")
	}
	// Handle type assertion for numeric ID
	switch v := id.(type) {
	case float64:
		c.ID = int(v)
	case int:
		c.ID = v
	case int64:
		c.ID = int(v)
	default:
		return errors.New("invalid type for claim id")
	}

	// Parse token field
	token, ok := claims["token"]
	if !ok {
		return errors.New("could not parse claim token")
	}
	c.Token = token.(string)

	// Optional multi-tenancy field
	c.TenantID = getOptionalInt64(claims, "tenant_id")

	// Optional scope field. Absent on tenant-scope refresh tokens
	// (omitempty); present on parent / platform / org tokens so the
	// refresh path can preserve scope.
	if s, ok := claims["scope"].(string); ok {
		c.Scope = s
	}

	return nil
}
