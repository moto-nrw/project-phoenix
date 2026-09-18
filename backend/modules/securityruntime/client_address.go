package securityruntime

import (
	"net"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
)

// ParseClientAddress parses the client address an audited operation carries,
// so the audit rows the compositions write never get a malformed inet value.
// An empty or unparseable address is nil.
func ParseClientAddress(address string) net.IP {
	return authorize.ParseClientAddress(address)
}
