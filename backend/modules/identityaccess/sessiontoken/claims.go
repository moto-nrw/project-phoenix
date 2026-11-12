package sessiontoken

import (
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/tokenprimitives"
)

type CommonClaims struct {
	ExpiresAt int64 `json:"exp,omitempty"`
	IssuedAt  int64 `json:"iat,omitempty"`
}

// AppClaims is the request projection of a verified access token.
type AppClaims struct {
	ID          int      `json:"id,omitempty"`
	Sub         string   `json:"sub,omitempty"`
	Username    string   `json:"username,omitempty"`
	FirstName   string   `json:"first_name,omitempty"`
	LastName    string   `json:"last_name,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	IsAdmin     bool     `json:"is_admin,omitempty"`
	// Scope is the portal claim; tokenprimitives.PortalOfClaim resolves it.
	Scope    string `json:"scope,omitempty"`
	TenantID int64  `json:"tenant_id,omitempty"`
	OrgID    int64  `json:"org_id,omitempty"`
	// FamilyID is the refresh-token family that minted this access token.
	// Push subscriptions stamp it so family-scoped logout can revoke the
	// same device's server-side subscription.
	FamilyID string `json:"family_id,omitempty"`
	// ReadOnly marks an admin staff-view preview token (#2893): the token
	// carries the TARGET account's identity, roles, and permissions so every
	// read evaluates exactly as that person, while
	// api.ReadOnlyPreviewMiddleware blocks every write. Preview tokens are
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
	// ViaTraeger marks a school session the Träger-Büro opened through the
	// Absprung (ADR 0084 section 5, #4654). The person acts as themselves
	// with the school's admin role; the school access rests on the Träger.
	ViaTraeger bool `json:"via_traeger,omitempty"`
	CommonClaims
}

// IsReadOnlyPreview reports whether this token is an admin staff-view
// preview token (#2893). Such tokens are read-only by contract.
func (c *AppClaims) IsReadOnlyPreview() bool {
	return c.ReadOnly
}

// IsPlatformScope reports an operator token.
func (c *AppClaims) IsPlatformScope() bool {
	return c.Scope == tokenprimitives.PlatformClaims.Claim
}

// IsOrgScope returns true if this is a Träger-portal token (#4649).
func (c *AppClaims) IsOrgScope() bool {
	return c.Scope == tokenprimitives.OrgClaims.Claim
}

// IsParentScope returns true if this is a parent-portal token.
func (c *AppClaims) IsParentScope() bool {
	return c.Scope == tokenprimitives.ParentClaims.Claim
}

// IsCatererScope returns true if this is a caterer-portal token (ADR 0077).
// Caterer tokens carry no tenant; the caterer portal opens each school it
// supplies in that school's own tenant transaction.
func (c *AppClaims) IsCatererScope() bool {
	return c.Scope == tokenprimitives.CatererClaims.Claim
}

// IsSchoolScope returns true if this is a school-portal token ("moto schule",
// #2207). School tokens are tenant-bound like regular tenant tokens, so the
// scope is the ONLY thing that distinguishes them once the tenant transaction
// is open — which is why authorization rules that must not apply to a
// Lehrkraft ask this question instead of inspecting roles (#2527).
func (c *AppClaims) IsSchoolScope() bool {
	return c.Scope == tokenprimitives.SchoolClaims.Claim
}

// appClaimsOf projects decoded access claims onto the request claims.
func appClaimsOf(c tokenprimitives.AccessClaims) AppClaims {
	return AppClaims{
		ID: int(c.AccountID), Sub: c.Email, Username: c.Username, FirstName: c.FirstName, LastName: c.LastName,
		Roles: c.Roles, Permissions: c.Permissions, IsAdmin: c.IsAdmin, Scope: c.Scope,
		TenantID: c.TenantID, OrgID: c.OrgID, FamilyID: c.FamilyID,
		ReadOnly: c.ReadOnly, ActingAdminID: c.ActingAdminID, PreviewID: c.PreviewID, ViaTraeger: c.ViaTraeger,
		CommonClaims: CommonClaims{ExpiresAt: c.ExpiresAt},
	}
}

// DisplayName is the caller's name as a page shows it: first and last name,
// else the username.
func (c AppClaims) DisplayName() string {
	if name := strings.TrimSpace(c.FirstName + " " + c.LastName); name != "" {
		return name
	}
	return c.Username
}
