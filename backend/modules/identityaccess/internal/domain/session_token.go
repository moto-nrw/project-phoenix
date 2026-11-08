package domain

import "time"

// SessionProvesInvitationOwnership excludes delegated and unfinished sessions.
// Parent sessions are global; staff and school sessions must name their school.
func SessionProvesInvitationOwnership(claims SessionClaims, now time.Time) bool {
	if claims.AccountID <= 0 || claims.ExpiresAt <= now.Unix() || claims.ReadOnly || claims.ActingAdminID != 0 || claims.PreviewID != "" {
		return false
	}
	switch claims.Scope {
	case PortalTenant.Claim, PortalTenant.LegacyClaim, PortalOrg.Claim, PortalSchool.Claim:
		return claims.TenantID > 0
	case PortalParent.Claim:
		return true
	default:
		return false
	}
}
