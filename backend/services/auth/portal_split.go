package auth

import (
	"net"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/clientip"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// The portal split predicates the retained flows still decide on (staff
// preview, invitations, listings). The login, refresh and switch decisions
// themselves live in modules/identityaccess (#3251).

// guardianRoleName must match auth.roles.name for the guardian base
// role (the one assigned by guardian_invitation_service.linkProfileToAccount
// and by decision_service.ensureGuardianRoleForTenant). Lowercase to
// match the DB seed.
const guardianRoleName = "guardian"

// IsGuardianOnlyForTenant reports whether the account has the guardian
// role and no other (admin/staff/teacher/etc.) for the given tenant.
// Used by the standard tenant LoginWithAudit to refuse guardians who
// try to log in at a tenant subdomain — they should be using the
// parents portal.
//
// "Only guardian" is a strict check: the role list must be exactly
// ["guardian"] case-insensitive after de-duplication. An account that
// is also a teacher at the same school is allowed through.
func IsGuardianOnlyForTenant(roleNames []string) bool {
	if len(roleNames) == 0 {
		return false
	}
	for _, r := range roleNames {
		if !strings.EqualFold(r, guardianRoleName) {
			return false
		}
	}
	return true
}

// isSchoolPortalRole reports whether the role grants access to the school
// portal (#2207). Today that is exactly the lehrkraft system role; a future
// Schulleitung role widens this predicate without touching the login flow,
// the refresh path, or the error codes — they are all named after the
// portal, not the role.
func isSchoolPortalRole(role *authModels.Role) bool {
	return IsLehrkraftSystemRole(role)
}

// IsSchoolPortalOnlyForTenant reports whether EVERY role the account holds at
// this school is a school-portal role. Such an account has no reachable
// surface in the OGS tenant portal since the cutover (#2207 PR 3) removed the
// tenant-side class-day mount, so the tenant login refuses it and points at
// moto schule.
//
// Mirrors IsGuardianOnlyForTenant, with one deliberate difference: it works on
// the loaded role objects rather than on names, because isSchoolPortalRole
// requires the SYSTEM lehrkraft role. A tenant-scoped custom role that merely
// happens to be called "Lehrkraft" carries arbitrary permissions and must keep
// its tenant-portal access — a name match would lock such an account out of
// both portals at once (the school login requires the system role too).
//
// An empty role set is NOT school-portal-only: an account with no roles at all
// is a different problem and stays on the existing path.
func IsSchoolPortalOnlyForTenant(roles []*authModels.Role) bool {
	if len(roles) == 0 {
		return false
	}
	for _, role := range roles {
		if !isSchoolPortalRole(role) {
			return false
		}
	}
	return true
}

// MaskEmailForUX renders an email address as `j***@example.com` so the
// frontend can show the user *which* mailbox just received a code without
// leaking the full address (e.g. in shared-screen scenarios). Shared with
// the operator login flow in services/platform.
func MaskEmailForUX(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 0 {
		return email
	}
	local := email[:at]
	domain := email[at:]
	if len(local) <= 1 {
		return local + "***" + domain
	}
	return string(local[0]) + "***" + domain
}

// ParseClientIP wraps net.ParseIP with the empty-string guard so audit rows
// don't get malformed inet values. Shared with the operator login flow in
// services/platform.
func ParseClientIP(ipAddress string) net.IP {
	return clientip.ParseIPString(ipAddress)
}
