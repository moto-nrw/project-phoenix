package domain

import (
	"errors"
	"time"
)

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
	// org) and of a parent session at a Träger; neither has a school. A
	// Träger school session names the Träger beside its school. Zero for
	// every other session.
	OrganizationID int64
	AccountID      int64
	Token          string
	TokenHash      string
	Expiry         time.Time
	Mobile         bool
	Identifier     *string
	PortalScope    string
	MFAVerified    bool
	FamilyID       string
	// SwitchedFromFamilyID names the origin of the session's switch lineage:
	// the login family a school switch descends from, or the Träger family
	// of an Absprung. It outlives the origin's rows (#5422).
	SwitchedFromFamilyID string
	FamilyExpiryCap      *time.Time
	// LineageDeadline is the transition deadline of a lineage descending
	// from a family issued before #5422; switches and rotations keep it.
	LineageDeadline      *time.Time
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
	if _, ok := PortalOfPersisted(s.PortalScope); !ok && s.PortalScope != PortalScopeUnknown {
		return errors.New("invalid portal scope")
	}
	if !s.traegerShapeHolds() {
		return errors.New("a Träger session and a parent session at a Träger name the Träger and no school; an OGS session of the Träger names both; no other session names a Träger")
	}
	switch {
	case (s.RotatedAt == nil) != (s.ReplacementTokenHash == nil):
		return errors.New("rotation handoff must include both timestamp and replacement token")
	case s.RotatedAt == nil && len(s.RecoveryProofHash) != 0:
		return errors.New("recovery proof hash requires a rotation handoff")
	}
	return nil
}

// traegerShapeHolds: only a Träger session, an OGS session the Träger
// opened through the Absprung (ADR 0084 section 5) and a parent session at a
// Träger (ADR 0216) name a Träger.
func (s *AccountSession) traegerShapeHolds() bool {
	switch {
	case s.PortalScope == PortalOrg.Name:
		return PortalOrg.Holds(s.TenantID, s.OrganizationID)
	case s.OrganizationID <= 0:
		return true
	case s.SessionKind() == SessionKindTraegerSchool:
		return s.TenantID > 0
	case s.SessionKind() == SessionKindTraegerParent:
		return s.TenantID == 0
	default:
		return false
	}
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

// SessionKind is the rules the stored session follows; a legacy row without
// a portal keeps its scope.
func (s AccountSession) SessionKind() SessionKind {
	portal, ok := PortalNamed(s.PortalScope)
	if !ok {
		return SessionKind(s.PortalScope)
	}
	return SessionKindOf(portal, s.OrganizationID > 0)
}

// KeptAtSchool reports whether the session belongs to a school; a legacy
// row without a portal does, a parent session at a Träger does not.
func (s AccountSession) KeptAtSchool() bool {
	portal, ok := PortalOfPersisted(s.PortalScope)
	return !ok || (portal.KeptAtSchool() && s.SessionKind() != SessionKindTraegerParent)
}
