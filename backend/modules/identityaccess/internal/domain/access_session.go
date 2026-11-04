package domain

import "time"

// AccessSession names the refresh-token family an access token rests on
// (#5194). ActingAdminID is set on a staff-view preview token: the preview
// carries the target's identity but rests on the acting admin's family.
type AccessSession struct {
	AccountID     int64
	ActingAdminID int64
	Scope         string
	FamilyID      string
}

// AccessSessionLive reports whether head, the latest generation of the
// family the access token names, still backs the token. Logout, the cap and
// every revocation delete the family, a tenant switch shortens it to the
// rotation grace, and its expiry ends it; an earlier generation's access
// token stays valid while the family lives. Rows minted before the portal
// scope was recorded match every scope.
func AccessSessionLive(head AccountSession, token AccessSession, now time.Time) bool {
	owner := token.AccountID
	if token.ActingAdminID > 0 {
		owner = token.ActingAdminID
	}
	scopeMatches := head.PortalScope == PortalScopeUnknown || head.PortalScope == PersistedPortalScope(token.Scope)
	return token.FamilyID != "" && head.FamilyID == token.FamilyID && owner > 0 && head.AccountID == owner &&
		scopeMatches && head.Expiry.After(now)
}
