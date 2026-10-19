package authorize

import (
	"net"

	"github.com/moto-nrw/project-phoenix/internal/clientip"
)

// ParseClientAddress parses the client address an audited operation carries.
// It is Security Runtime's own guard against persisting a malformed inet
// value: an empty or unparseable address stays nil rather than reaching a
// column as garbage. Consumers reach it through the Security Runtime facade
// (#3364).
func ParseClientAddress(address string) net.IP {
	return clientip.ParseIPString(address)
}
