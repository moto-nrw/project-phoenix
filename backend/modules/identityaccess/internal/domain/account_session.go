package domain

import (
	"errors"
	"time"
)

var (
	// ErrAccountSessionNotFound reports a lookup that matched no account
	// refresh session in the caller's tenant scope.
	ErrAccountSessionNotFound = errors.New("account session not found")
	// ErrAccountSessionRotated reports a rotation hand-off that found no
	// un-rotated row: the session was already rotated or does not exist.
	ErrAccountSessionRotated = errors.New("account session was already rotated or not found")
)

// Portal scopes an account refresh session is persisted under. Tenant and
// org share the staff portal; unknown marks legacy rows minted before the
// scope was recorded.
const (
	PortalScopeTenant  = "tenant"
	PortalScopeOrg     = "org"
	PortalScopeParent  = "parent"
	PortalScopeSchool  = "school"
	PortalScopeUnknown = "unknown"
)

// CapPortalScopes returns the portal scopes that share one session cap.
// Tenant and org share the staff portal. Unknown legacy rows stay in their
// own bucket so a staff login cannot evict a legacy parent session.
func CapPortalScopes(portalScope string) []string {
	switch portalScope {
	case PortalScopeTenant, PortalScopeOrg:
		return []string{PortalScopeTenant, PortalScopeOrg}
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
type AccountSession struct {
	ID                int64
	TenantID          int64
	AccountID         int64
	Token             string
	Expiry            time.Time
	Mobile            bool
	Identifier        *string
	PortalScope       string
	FamilyID          string
	FamilyExpiryCap   *time.Time
	Generation        int
	RotatedAt         *time.Time
	ReplacementToken  *string
	RecoveryProofHash []byte
	CreatedAt         time.Time
	UpdatedAt         time.Time
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
	switch {
	case (s.RotatedAt == nil) != (s.ReplacementToken == nil):
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
