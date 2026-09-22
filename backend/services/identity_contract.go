package services

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The Identity & Access request shapes and the parents-portal guardian port,
// exposed here the way AuditCommand and the time-tracking cleanup contract
// are: the owners whose own tests drive these flows keep composing and
// asserting through this root instead of naming a contract their package may
// not import (#3332).

type (
	// SchoolInvitationRequest describes a school invitation to create.
	SchoolInvitationRequest = identityaccess.SchoolInvitationRequest
	// InvitationRegistration is what an invitee supplies when accepting.
	InvitationRegistration = identityaccess.InvitationRegistration
	// GuardianInviteRequest is one parent-initiated invite to a child.
	GuardianInviteRequest = parentportal.GuardianInviteRequest
	// GuardianInviteOutcome is what the owner did with such an invite.
	GuardianInviteOutcome = parentportal.GuardianInviteOutcome
	// GuardianAccessRevocation removes one account's access to one child.
	GuardianAccessRevocation = parentportal.GuardianAccessRevocation
)

var (
	// ErrAccountAlreadyHasTenantAccess reports an invitation to a school the
	// account can already sign in to.
	ErrAccountAlreadyHasTenantAccess = identityaccess.ErrAccountAlreadyHasTenantAccess
	// ErrInvitationOwnerRequired reports an acceptance for an address that
	// already has an account, without a session of that account.
	ErrInvitationOwnerRequired = identityaccess.ErrInvitationOwnerRequired
)
