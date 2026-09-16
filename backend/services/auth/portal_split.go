package auth

import (
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
