package tokenprimitives

// Portal names. The name of a portal is the scope its refresh sessions are
// persisted under, the scope of its MFA challenge and enrollment tokens, and
// the portal the BFF names when it validates a session.
const (
	PortalTenant   = "tenant"
	PortalSchool   = "school"
	PortalParent   = "parent"
	PortalCaterer  = "caterer"
	PortalOrg      = "org"
	PortalPlatform = "platform"
)

// Shape says how a portal's access token carries a school or a Träger.
type Shape int

const (
	// Ignored: the gate neither checks nor binds the ID.
	Ignored Shape = iota
	// Optional: the gate binds the ID when the token carries one.
	Optional
	// Required: the token must carry the ID.
	Required
	// Forbidden: the token must not carry the ID.
	Forbidden
)

func (s Shape) holds(id int64) bool {
	switch s {
	case Required:
		return id > 0
	case Forbidden:
		return id == 0
	default:
		return true
	}
}

// PortalClaims is a portal as its access token shows it. The tenant portal
// travels with the empty scope claim; tokens minted before that carry
// "tenant".
type PortalClaims struct {
	Name        string
	Claim       string
	LegacyClaim string
	Tenant      Shape
	Org         Shape
	// Challenge and Enrollment: the portal has its own MFA challenge or
	// enrollment token, redeemable only there.
	Challenge  bool
	Enrollment bool
}

// Holds reports whether a token's school and Träger fit the portal. A
// Träger-Portal token names its Träger and no school (ADR 0084).
func (p PortalClaims) Holds(tenantID, orgID int64) bool {
	return p.Tenant.holds(tenantID) && p.Org.holds(orgID)
}

// The portals, closed. Platform tokens bind a school when they name one and
// otherwise their organization; parent and caterer tokens bind no school
// (ADR 0077).
var (
	TenantClaims   = PortalClaims{Name: PortalTenant, Claim: "", LegacyClaim: PortalTenant, Tenant: Required, Org: Optional, Challenge: true, Enrollment: true}
	SchoolClaims   = PortalClaims{Name: PortalSchool, Claim: PortalSchool, Tenant: Required, Org: Optional, Challenge: true, Enrollment: true}
	ParentClaims   = PortalClaims{Name: PortalParent, Claim: PortalParent}
	CatererClaims  = PortalClaims{Name: PortalCaterer, Claim: PortalCaterer, Challenge: true}
	OrgClaims      = PortalClaims{Name: PortalOrg, Claim: PortalOrg, Tenant: Forbidden, Org: Required, Challenge: true, Enrollment: true}
	PlatformClaims = PortalClaims{Name: PortalPlatform, Claim: PortalPlatform, Tenant: Optional, Org: Optional, Challenge: true, Enrollment: true}

	portalClaims = []PortalClaims{TenantClaims, SchoolClaims, ParentClaims, CatererClaims, OrgClaims, PlatformClaims}
)

// PortalOfClaim resolves a scope claim, the legacy tenant claim included.
func PortalOfClaim(scope string) (PortalClaims, bool) {
	for _, portal := range portalClaims {
		if scope == portal.Claim || (portal.LegacyClaim != "" && scope == portal.LegacyClaim) {
			return portal, true
		}
	}
	return PortalClaims{}, false
}

// PortalNamed resolves a portal name.
func PortalNamed(name string) (PortalClaims, bool) {
	for _, portal := range portalClaims {
		if name == portal.Name {
			return portal, true
		}
	}
	return PortalClaims{}, false
}

// EnrollmentFits reports whether an enrollment token's school fits the
// portal. Enrollment tokens carry no Träger, so the check stops at the school.
func EnrollmentFits(portal PortalClaims, tenantID int64) bool {
	return portal.Tenant.holds(tenantID)
}
