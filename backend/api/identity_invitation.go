package api

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Identity & Access owns the school invitation flows (#2722). This file
// binds the seams they need to the retained material the root composes (the
// role-grant policy, the JWT signer behind the owner check, the mail with
// its tenant reply-to identity). The invitation routes and the staff import
// call the public capability directly (#3332); the operator provisioning
// routes, which may not name it, read the retained envelope this file
// translates into.

// InvitationCapability is what the invitation routes consume: the school
// invitation flows the staff screens drive and the guardian invitation flows
// the public accept page drives. One owner serves both (#3332).
type InvitationCapability interface {
	identityaccess.SchoolInvitations
	identityaccess.GuardianInvitations
}
