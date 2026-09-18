package operator

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// The operator dashboard keeps the routes Identity & Access does not own:
// the invitation management, the profile read and e-mail change, the second
// factor, the passkey ceremonies and the school-account MFA administration.
// It may not name the owner's contract, so it consumes the seam Identity &
// Access exposes for it (#3364).

// OperatorAccess is what those routes need from Identity & Access: the
// operator directory, the token mint their second factors end in, and the
// invitation and e-mail change flows.
type OperatorAccess interface {
	identityoperator.Operators
	identityoperator.OperatorSessions
	identityoperator.OperatorProvisioning
}

// clientAddress renders the request address for the operator ledger; an
// absent address is recorded as none, as it always was.
func clientAddress(r *http.Request) string {
	ip := common.ParseClientIP(r)
	if ip == nil {
		return ""
	}
	return ip.String()
}
