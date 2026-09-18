package auth

import "errors"

// The school and guardian invitation flows live in Identity & Access (#2722)
// and their routes call the public capability directly (#3332). The
// sentinels below are what remains here: the operator provisioning routes of
// Organisation & Tenancy classify a refused invitation on them and may not
// name the owner's contract, so the composition root translates the public
// outcomes into these. Their texts are the wire contract.
var (
	// ErrInvitationNotFound reports a link that does not exist.
	ErrInvitationNotFound = errors.New("invitation not found")
	// ErrInvitationExpired reports a link past its expiry.
	ErrInvitationExpired = errors.New("invitation has expired")
	// ErrInvitationUsed reports a link that was already spent.
	ErrInvitationUsed = errors.New("invitation has already been used")
	// ErrInvitationTenantDeleted reports a link whose school is gone.
	ErrInvitationTenantDeleted = errors.New("the school for this invitation has been deleted")
	// ErrInvitationNameRequired reports an acceptance without both names.
	ErrInvitationNameRequired = errors.New("first name and last name are required")
	// ErrAccountAlreadyHasTenantAccess reports an invitation to a school the
	// account can already sign in to.
	ErrAccountAlreadyHasTenantAccess = errors.New("account already has access to tenant")
)
