package domain

import (
	"errors"
	"time"
)

// ErrTransactionUnusable reports a seam that left the caller's transaction
// in a state nothing may commit on top of. A best-effort step that fails
// this way is not best-effort any more: the flow gives up its writes
// instead of committing half of them.
var ErrTransactionUnusable = errors.New("transaction state is unusable")

// The guardian invitation lifecycle (#2722): the token a guardian redeems
// for their parents-portal account. The relative access flows in
// account_lifecycle.go write the same rows for a specific child.

// GuardianInvitationRequest issues an invitation for a guardian contact that
// has an address on file and no account yet.
type GuardianInvitationRequest struct {
	GuardianProfileID int64
	CreatedBy         int64
}

// GuardianInvitationPreview is the public-safe view of a redeemable
// invitation: enough to pre-fill the accept page and brand it, and no
// internal identifier.
type GuardianInvitationPreview struct {
	Email         string
	FirstName     string
	LastName      string
	ExpiresAt     time.Time
	SchoolName    string
	SchoolSlug    string
	SchoolLogoURL string
}

// GuardianRegistration is the form payload of the accept page.
type GuardianRegistration struct {
	Password        string
	ConfirmPassword string
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
