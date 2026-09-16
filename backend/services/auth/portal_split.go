package auth

import (
	"errors"
	"net"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/clientip"
)

// The login, refresh and switch portal decisions live in
// modules/identityaccess (#3251, #3225); the helpers below are shared with
// the operator login flow.

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

var (
	// Parent / staff portal split. Returned when an account tries to log
	// in at the wrong portal:
	//   - guardian-only account hitting the tenant login → ErrParentMustUseParentPortal
	//   - account with no guardian role hitting the parents login → ErrAccountNoGuardianRole
	// Frontend turns these into a clear redirect message ("please log in
	// at https://parents.{TENANT_DOMAIN}/" or vice versa).
	ErrParentMustUseParentPortal = errors.New("guardian accounts must log in at the parents portal")
	ErrAccountNoGuardianRole     = errors.New("account is not a guardian at any school")

	// School portal split (#2207). Returned when an account without a
	// school-portal role (today: the lehrkraft system role) tries to log
	// in at the school portal. Deliberately named after the portal, not
	// the role — a future Schulleitung role widens isSchoolPortalRole
	// without renaming this sentinel or its wire code.
	ErrAccountNoSchoolPortalRole = errors.New("account has no school portal role at any school")

	// ErrMustUseSchoolPortal is the symmetric half of the school portal
	// split: an account whose only role at this school is a school-portal
	// role (today: lehrkraft) has no reachable surface in the OGS tenant
	// portal, so the tenant login refuses it and points at moto schule.
	// Dual-role accounts (also caregiver, admin, or guest) pass through
	// unchanged — same rule as the guardian split above.
	ErrMustUseSchoolPortal = errors.New("school portal accounts must log in at the school portal")
)
