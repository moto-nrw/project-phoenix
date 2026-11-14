package domain

import "slices"

// The domain may import no other package of its owner, so the portal names,
// claims and token shapes repeat the tokenprimitives portals; a test pins
// the two tables to each other.

// Shape says how a portal's access token carries a school or a Träger.
type Shape int

const (
	// ShapeIgnored: the portal neither checks nor binds the ID.
	ShapeIgnored Shape = iota
	// ShapeOptional: the ID is bound when the token carries one.
	ShapeOptional
	// ShapeRequired: the token must carry the ID.
	ShapeRequired
	// ShapeForbidden: the token must not carry the ID.
	ShapeForbidden
)

func (s Shape) holds(id int64) bool {
	switch s {
	case ShapeRequired:
		return id > 0
	case ShapeForbidden:
		return id == 0
	default:
		return true
	}
}

// MFARule is the second-factor policy a portal's sessions follow at mint and
// refresh.
type MFARule int

const (
	// MFASchoolPolicy applies the MFA mode of the session's school.
	MFASchoolPolicy MFARule = iota + 1
	// MFAAlways needs the proof for every role: the Träger-Portal is bound to
	// no school's mode (ADR 0084, #4651).
	MFAAlways
	// MFAAccountChoice follows the account's own enrollment: the caterer's
	// factor is voluntary and no school's mode reaches it (ADR 0077 point 7).
	MFAAccountChoice
	// MFANone: parent sessions carry no staff authority.
	MFANone
	// MFAOperator: operators run their own pipeline.
	MFAOperator
)

// Push portals a session's push subscriptions belong to.
const (
	PushPortalStaff  = "staff"
	PushPortalParent = "parent"
	PushPortalSchool = "school"
)

// PortalScopeUnknown marks refresh sessions persisted before the portal scope
// was recorded. They match every scope, keep their own cap bucket, may belong
// to every push portal and follow the school MFA policy.
const PortalScopeUnknown = "unknown"

// Portal is one portal and every fact the session flows derive from it. Name
// is the persisted scope, the MFA challenge and enrollment scope, and the
// portal the BFF names; Claim is the JWT scope.
type Portal struct {
	Name        string
	Claim       string
	LegacyClaim string
	Tenant      Shape
	Org         Shape
	// Persisted: the portal's account refresh sessions are stored under Name,
	// each portal in its own cap bucket (ADR 0084).
	Persisted bool
	// Challenge and Enrollment: the portal has its own MFA challenge or
	// enrollment token, redeemable only there.
	Challenge  bool
	Enrollment bool
	// TrustedDevice: a redeemed challenge may remember the browser. A
	// portal without a school remembers it without one: the caterer portal
	// at the account (#4497), the Träger-Portal at its Träger (#6955).
	TrustedDevice bool
	MFA           MFARule
	// Push is empty for a portal without push messages.
	Push string
}

// The portals, closed.
var (
	PortalTenant = Portal{Name: "tenant", Claim: "", LegacyClaim: "tenant", Tenant: ShapeRequired, Org: ShapeOptional,
		Persisted: true, Challenge: true, Enrollment: true, TrustedDevice: true, MFA: MFASchoolPolicy, Push: PushPortalStaff}
	PortalSchool = Portal{Name: "school", Claim: "school", Tenant: ShapeRequired, Org: ShapeOptional,
		Persisted: true, Challenge: true, Enrollment: true, TrustedDevice: true, MFA: MFASchoolPolicy, Push: PushPortalSchool}
	PortalParent = Portal{Name: "parent", Claim: "parent",
		Persisted: true, MFA: MFANone, Push: PushPortalParent}
	PortalCaterer = Portal{Name: "caterer", Claim: "caterer",
		Persisted: true, Challenge: true, TrustedDevice: true, MFA: MFAAccountChoice}
	PortalOrg = Portal{Name: "org", Claim: "org", Tenant: ShapeForbidden, Org: ShapeRequired,
		Persisted: true, Challenge: true, Enrollment: true, TrustedDevice: true, MFA: MFAAlways, Push: PushPortalStaff}
	PortalPlatform = Portal{Name: "platform", Claim: "platform", Tenant: ShapeOptional, Org: ShapeOptional,
		Challenge: true, Enrollment: true, TrustedDevice: true, MFA: MFAOperator}

	portals = []Portal{PortalTenant, PortalSchool, PortalParent, PortalCaterer, PortalOrg, PortalPlatform}
)

// Holds reports whether a token's school and Träger fit the portal. A
// Träger-Portal token names its Träger and no school (ADR 0084).
func (p Portal) Holds(tenantID, orgID int64) bool {
	return p.Tenant.holds(tenantID) && p.Org.holds(orgID)
}

// Schoolless reports a portal whose sign-in names no school: its challenge,
// its remembered device and its authentication-ledger entries carry none
// (Träger-Portal, ADR 0084; caterer, ADR 0077 addendum).
func (p Portal) Schoolless() bool { return p.Tenant != ShapeRequired }

// KeptAtSchool reports whether the portal's refresh sessions are stored at a
// school. Only a portal whose token must not name one keeps them without:
// the Träger-Portal. Parent and caterer sessions are pinned to a school their
// token does not carry.
func (p Portal) KeptAtSchool() bool { return p.Tenant != ShapeForbidden }

// SessionKind names the rules a session follows: its portal's, or those of
// a sub-portal that names a Träger: the Träger school, the tenant portal's
// sub-portal for an OGS session the Träger-Büro opened through the Absprung
// (ADR 0084 section 5), and the Träger parent, the parents portal's
// sub-portal for an account set up through a holiday registration (ADR 0216).
type SessionKind string

const (
	// SessionKindTraegerSchool is the Träger school sub-portal.
	SessionKindTraegerSchool SessionKind = "traeger_school"
	// SessionKindTraegerParent is a parent session that names a Träger and
	// no school: the account holds no guardian role at a school that books
	// Eltern-App, but parents-portal access at the Träger.
	SessionKindTraegerParent SessionKind = "traeger_parent"
)

// SessionKindOf is the one recognition rule of the sub-portals: a session of
// the tenant or the parents portal that names a Träger. Each token
// representation says so in its own way: the refresh claims and the stored
// row by their Träger; the access token of the tenant portal by via_traeger,
// because every staff access token names its school's Träger, and the access
// token of the parents portal by its Träger, which no other parent token
// names.
func SessionKindOf(portal Portal, namesTraeger bool) SessionKind {
	switch {
	case portal.Name == PortalTenant.Name && namesTraeger:
		return SessionKindTraegerSchool
	case portal.Name == PortalParent.Name && namesTraeger:
		return SessionKindTraegerParent
	}
	return SessionKind(portal.Name)
}

// PortalOfClaim resolves a JWT scope, the legacy tenant claim included.
func PortalOfClaim(scope string) (Portal, bool) {
	for _, portal := range portals {
		if scope == portal.Claim || (portal.LegacyClaim != "" && scope == portal.LegacyClaim) {
			return portal, true
		}
	}
	return Portal{}, false
}

// PortalNamed resolves a portal name.
func PortalNamed(name string) (Portal, bool) {
	for _, portal := range portals {
		if name == portal.Name {
			return portal, true
		}
	}
	return Portal{}, false
}

// PortalOfPersisted resolves a persisted portal scope; unknown and empty
// legacy rows resolve to no portal.
func PortalOfPersisted(scope string) (Portal, bool) {
	portal, ok := PortalNamed(scope)
	return portal, ok && portal.Persisted
}

// PersistedPortalScope maps a JWT scope to the portal scope a refresh
// session is stored under.
func PersistedPortalScope(jwtScope string) string {
	if portal, ok := PortalOfClaim(jwtScope); ok && portal.Persisted {
		return portal.Name
	}
	return PortalScopeUnknown
}

// SessionScopeMatches reports whether an access token's scope belongs to the
// portal a session validation names.
func SessionScopeMatches(scope, portalName string) bool {
	portal, ok := PortalNamed(portalName)
	claimed, claimOK := PortalOfClaim(scope)
	return ok && claimOK && portal.Name == claimed.Name
}

// CapPortalScopes returns the portal scopes that share one session cap.
// Unknown legacy rows stay in their own bucket so a staff login cannot evict
// a legacy parent session.
func CapPortalScopes(portalScope string) []string {
	if portalScope == "" {
		return []string{PortalScopeUnknown}
	}
	return []string{portalScope}
}

// MFARuleOf is the MFA rule of a persisted portal scope.
func MFARuleOf(portalScope string) MFARule {
	if portal, ok := PortalOfPersisted(portalScope); ok {
		return portal.MFA
	}
	return MFASchoolPolicy
}

// PushPortalsOf names the push portals a persisted session may hold
// subscriptions in.
func PushPortalsOf(portalScope string) []string {
	portal, ok := PortalOfPersisted(portalScope)
	if !ok {
		if portalScope == "" || portalScope == PortalScopeUnknown {
			return AllPushPortals()
		}
		return []string{PushPortalStaff}
	}
	if portal.Push == "" {
		return nil
	}
	return []string{portal.Push}
}

// AllPushPortals names every push portal of the table once, in table order.
func AllPushPortals() []string {
	var pushes []string
	for _, portal := range portals {
		if portal.Push != "" && !slices.Contains(pushes, portal.Push) {
			pushes = append(pushes, portal.Push)
		}
	}
	return pushes
}
