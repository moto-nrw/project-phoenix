package domain

import (
	"errors"
	"time"
)

// Portal scopes an account refresh session is persisted under. Org is the
// Träger-Portal (ADR 0084); unknown marks legacy rows minted before the scope
// was recorded.
const (
	PortalScopeTenant  = "tenant"
	PortalScopeOrg     = "org"
	PortalScopeParent  = "parent"
	PortalScopeSchool  = "school"
	PortalScopeUnknown = "unknown"
)

// CapPortalScopes returns the portal scopes that share one session cap. Every
// portal has its own bucket, the Träger-Portal included (ADR 0084). Unknown
// legacy rows stay in their own bucket so a staff login cannot evict a legacy
// parent session.
func CapPortalScopes(portalScope string) []string {
	switch portalScope {
	case PortalScopeTenant:
		return []string{PortalScopeTenant}
	case PortalScopeOrg:
		return []string{PortalScopeOrg}
	case PortalScopeParent:
		return []string{PortalScopeParent}
	case PortalScopeSchool:
		return []string{PortalScopeSchool}
	case "", PortalScopeUnknown:
		return []string{PortalScopeUnknown}
	default:
		return []string{portalScope}
	}
}

// AccountSession is one persisted, revocable refresh session of a platform
// account at one school (auth.tokens). A family groups the generations one
// login produced through rotation; the hand-off columns let a lost rotation
// response be recovered within the rotation grace and a replay be detected.
//
// Token is the raw value the refresh JWT carries. Only the minting or
// presenting request knows it; the store keeps TokenHash, its hex SHA-256,
// and ReplacementTokenHash likewise names the successor by hash (#4902).
// Listings and revocations return Token empty.
type AccountSession struct {
	ID       int64
	TenantID int64
	// OrganizationID is the Träger of a Träger-Portal session (portal scope
	// org); such a session has no school. Zero for every other portal.
	OrganizationID       int64
	AccountID            int64
	Token                string
	TokenHash            string
	Expiry               time.Time
	Mobile               bool
	Identifier           *string
	PortalScope          string
	MFAVerified          bool
	FamilyID             string
	FamilyExpiryCap      *time.Time
	Generation           int
	RotatedAt            *time.Time
	ReplacementTokenHash *string
	RecoveryProofHash    []byte
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Validate normalizes an empty portal scope to unknown and rejects a session
// that cannot be stored. Expiry is wall-clock policy the read paths enforce
// through their SQL filters, so it is not validated here.
func (s *AccountSession) Validate() error {
	if s.PortalScope == "" {
		s.PortalScope = PortalScopeUnknown
	}
	switch {
	case s.AccountID <= 0:
		return errors.New("account ID is required")
	case s.Token == "":
		return errors.New("token value is required")
	}
	switch s.PortalScope {
	case PortalScopeTenant, PortalScopeOrg, PortalScopeParent, PortalScopeSchool, PortalScopeUnknown:
	default:
		return errors.New("invalid portal scope")
	}
	// An OGS session may name the Träger of its Absprung (ADR 0084 section 5).
	traegerSchool := s.PortalScope == PortalScopeTenant && s.OrganizationID > 0
	if (s.PortalScope == PortalScopeOrg) != (s.OrganizationID > 0 && !traegerSchool) || (s.PortalScope == PortalScopeOrg && s.TenantID != 0) || (traegerSchool && s.TenantID <= 0) {
		return errors.New("a Träger session names its Träger and no school; an OGS session of the Träger names both; no other session names a Träger")
	}
	switch {
	case (s.RotatedAt == nil) != (s.ReplacementTokenHash == nil):
		return errors.New("rotation handoff must include both timestamp and replacement token")
	case s.RotatedAt == nil && len(s.RecoveryProofHash) != 0:
		return errors.New("recovery proof hash requires a rotation handoff")
	}
	return nil
}

// AccountSessionLiveness narrows a listing by the expiry of the sessions.
type AccountSessionLiveness int

const (
	// AccountSessionsAny lists sessions regardless of expiry.
	AccountSessionsAny AccountSessionLiveness = iota
	// AccountSessionsLive lists sessions whose expiry is in the future.
	AccountSessionsLive
	// AccountSessionsExpired lists sessions whose expiry has passed.
	AccountSessionsExpired
)

// AccountSessionFilter narrows a listing. Zero values do not filter.
type AccountSessionFilter struct {
	AccountID int64
	FamilyID  string
	Mobile    *bool
	Liveness  AccountSessionLiveness
}

// TraegerSchoolHandoffTTL is how long the hand-off refresh token of an
// Absprung stays redeemable (ADR 0084 section 5, #4654). The school's host
// redeems it right away; its first rotation gives the session its normal
// lifetime.
const TraegerSchoolHandoffTTL = 2 * time.Minute

// IsTraegerSchoolSession reports an OGS session the Träger-Büro opened through
// the Absprung: portal scope tenant with the Träger in OrganizationID.
func IsTraegerSchoolSession(session AccountSession) bool {
	return session.PortalScope == PortalScopeTenant && session.OrganizationID > 0
}
