package api

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Identity & Access owns the guardian invitation lifecycle (#2722). This
// file binds the two seams the flows leave to the root — the mail, which
// knows the parents portal host and the tenant's token lifetime, and the
// enrollment requests an acceptance claims — and the runtimes the consumers
// that may not name the owner's contract reach it through (#3332).

// GuardianInvitationCapability is what the guardian invitation routes, the
// enrollment decisions, the guardian directory and the parents portal
// consume: the invitation lifecycle and the relative access flows of one
// owner. Each consumer takes the narrower half it needs.
type GuardianInvitationCapability interface {
	identityaccess.GuardianInvitations
	identityaccess.GuardianRelativeAccess
}

// --- delivery ---------------------------------------------------------------

// --- the invitation reads other owners' flows need ---------------------------

// --- school branding --------------------------------------------------------

// --- the parents portal's guardian access runtime ---------------------------
