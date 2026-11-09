package domain

import (
	"errors"
	"strings"
	"time"
)

// ErrTransactionUnusable reports a seam that left the caller's transaction
// in a state nothing may commit on top of. A best-effort step that fails
// this way is not best-effort any more: the flow gives up its writes
// instead of committing half of them.
var ErrTransactionUnusable = errors.New("transaction state is unusable")

// ErrGuardianInvitationAddressChanged refuses a link whose address is no
// longer the guardian's: the profile address changed after the link was
// issued (#6294).
var ErrGuardianInvitationAddressChanged = errors.New("the guardian's address changed after the invitation was issued")

// The guardian invitation lifecycle (#2722): the token a guardian redeems
// for their parents-portal account. The relative access flows in
// account_lifecycle.go write the same rows for a specific child.

// GuardianInvitationRequest issues an invitation for a guardian contact that
// has an address on file and no account yet.
type GuardianInvitationRequest struct {
	GuardianProfileID int64
	CreatedBy         int64
}

// Redeemable reports why the invitation can no longer be spent, and nil
// while it can. An invitation awaiting or refused by a staff decision is
// not a link anybody may redeem: its token stays frozen (#2172).
func (i GuardianInvitation) Redeemable(now time.Time) error {
	if i.AcceptedAt != nil {
		return ErrInvitationUsed
	}
	if now.After(i.ExpiresAt) {
		return ErrInvitationExpired
	}
	switch i.ApprovalStatus {
	case "", GuardianInvitationApprovalNotRequired, GuardianInvitationApprovalApproved:
		return nil
	default:
		return ErrInvitationNotFound
	}
}

// GuardianInvitationAddress is the form an invitation stores its address in:
// the comparison ignores case and surrounding blanks, as every guardian
// address lookup does.
func GuardianInvitationAddress(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
