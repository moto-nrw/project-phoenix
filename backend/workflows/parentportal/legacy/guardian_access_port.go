package legacy

import (
	"context"
	"errors"
)

// Inviting a further guardian to a child and removing one again are Identity
// & Access flows (#2722). The parents portal reaches them through the
// consumer-owned port below, which the composition root binds to the owner's
// relative-access capability (#3332). The port names no owner contract, so
// the outcome strings the portal answers with and the refusals it renders
// stay this package's business.

var (
	// ErrGuardianAccessUnavailable reports a service composed without the
	// relative-access port.
	ErrGuardianAccessUnavailable = errors.New("guardian relative access is not composed")
	// ErrCannotRemovePrimaryGuardian refuses removing the child's primary
	// guardian from the parents portal.
	ErrCannotRemovePrimaryGuardian = errors.New("the primary guardian cannot be removed by a parent")
	// ErrCannotRemoveStaffManagedGuardian refuses removing a contact the
	// school manages.
	ErrCannotRemoveStaffManagedGuardian = errors.New("staff-managed guardian contacts cannot be removed by a parent")
	// ErrCannotRemoveOwnAccess refuses a parent removing themselves.
	ErrCannotRemoveOwnAccess = errors.New("a parent cannot remove their own access to a child")
	// ErrCannotRemovePayerGuardian refuses removing the child's payer: that
	// is a financial decision the parents portal never holds (#2608). German
	// because the portal shows it verbatim.
	ErrCannotRemovePayerGuardian = errors.New("Diese Person ist als Zahler für das Kind eingetragen und kann nicht entfernt werden. Bitte wenden Sie sich an die Schule.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrInviteSocialWorkerManaged refuses inviting a social-worker contact,
	// which the school manages, into the parents portal.
	ErrInviteSocialWorkerManaged = errors.New("a social-worker contact is managed by the school and cannot be invited to the parents portal")
)

// GuardianInviteRequest is one parent-initiated "invite this address to my
// child" request.
type GuardianInviteRequest struct {
	StudentID int64
	Email     string
	FirstName string
	LastName  string
	// CreatedBy and RequestedByAccountID are the same parent account; the
	// owner records the second one to mark the request parent-initiated.
	CreatedBy            int64
	RequestedByAccountID int64
	// RequireApproval queues the invite for staff instead of acting now; the
	// caller resolves it from guardians.parent_invite_mode.
	RequireApproval bool
	// ConfirmRoleUpgrade confirms upgrading an existing restrictive contact
	// link to full portal access (#2172).
	ConfirmRoleUpgrade bool
}

// GuardianInviteOutcome is what the owner did, as the portal reports it.
type GuardianInviteOutcome struct {
	Outcome           string
	GuardianProfileID int64
	ExistingRole      string
}

// GuardianAccessRevocation removes one account's access to one child.
type GuardianAccessRevocation struct {
	StudentID         int64
	GuardianProfileID int64
	ActorAccountID    int64
}

// GuardianAccess is the consumer-owned port over the relative-access flows.
// The composition root binds it and translates the owner's refusals into the
// sentinels above.
type GuardianAccess interface {
	InviteToStudent(ctx context.Context, request GuardianInviteRequest) (GuardianInviteOutcome, error)
	// RevokeAccess always runs as a parent removal: the portal never holds
	// the staff authority the owner's staff paths carry.
	RevokeAccess(ctx context.Context, revocation GuardianAccessRevocation) error
}
