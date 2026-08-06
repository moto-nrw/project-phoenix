package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// GuardianInvitation represents an invitation sent to create a guardian account
type GuardianInvitation struct {
	base.Model `bun:"schema:auth,table:guardian_invitations"`
	base.TenantModel

	Token             string     `bun:"token,notnull,unique" json:"token"`
	GuardianProfileID int64      `bun:"guardian_profile_id,notnull" json:"guardian_profile_id"`
	CreatedBy         int64      `bun:"created_by,notnull" json:"created_by"`
	ExpiresAt         time.Time  `bun:"expires_at,notnull" json:"expires_at"`
	AcceptedAt        *time.Time `bun:"accepted_at" json:"accepted_at,omitempty"`
	EmailSentAt       *time.Time `bun:"email_sent_at" json:"email_sent_at,omitempty"`
	EmailError        *string    `bun:"email_error" json:"email_error,omitempty"`
	EmailRetryCount   int        `bun:"email_retry_count,default:0" json:"email_retry_count"`

	// Parent-initiated invites + staff approval (migration 1.15.120).
	// StudentID names the child this invite grants access to; lets the accept
	// flow link an existing account to an additional child (sibling case).
	StudentID *int64 `bun:"student_id" json:"student_id,omitempty"`
	// RequestedByAccountID is set when a parent (not staff) initiated the
	// invite; NULL for staff-initiated invites.
	RequestedByAccountID *int64 `bun:"requested_by_account_id" json:"requested_by_account_id,omitempty"`
	// ApprovalStatus is one of the GuardianInvitationApproval* constants.
	ApprovalStatus string     `bun:"approval_status,notnull,default:'not_required'" json:"approval_status"`
	ApprovedBy     *int64     `bun:"approved_by" json:"approved_by,omitempty"`
	ApprovedAt     *time.Time `bun:"approved_at" json:"approved_at,omitempty"`
	// ProfileCreatedForInvitation is true only when this invite flow created
	// the guardian profile specifically to back this invitation.
	ProfileCreatedForInvitation bool `bun:"profile_created_for_invitation,notnull,default:false" json:"profile_created_for_invitation"`
	// RoleUpgrade is true when approving this invitation must also upgrade the
	// existing restrictive students_guardians link (emergency_contact/
	// pickup_only/custom) to legal_guardian, because the invited person was
	// already a restricted contact and the inviter confirmed the upgrade
	// (#2172). Direct-mode invites apply the upgrade immediately instead.
	RoleUpgrade bool `bun:"role_upgrade,notnull,default:false" json:"role_upgrade"`

	// Relations (not stored in database)
}

// Approval-status values for GuardianInvitation.ApprovalStatus.
const (
	GuardianInvitationApprovalNotRequired = "not_required" // staff invites + parent direct mode
	GuardianInvitationApprovalPending     = "pending"      // parent invite awaiting staff approval
	GuardianInvitationApprovalApproved    = "approved"     // staff approved; email dispatched
	GuardianInvitationApprovalRejected    = "rejected"     // staff rejected; no access granted
)

// IsPendingApproval reports whether this invite is a parent-initiated request
// still awaiting staff approval (pure field accessor).
func (i *GuardianInvitation) IsPendingApproval() bool {
	return i.ApprovalStatus == GuardianInvitationApprovalPending
}

// Validate ensures core fields are present and sensible
func (i *GuardianInvitation) Validate() error {
	if strings.TrimSpace(i.Token) == "" {
		return errors.New("token is required")
	}
	if i.GuardianProfileID <= 0 {
		return errors.New("guardian profile ID is required")
	}
	if i.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	if i.ExpiresAt.IsZero() {
		return errors.New("expires_at is required")
	}
	return nil
}

// IsAccepted returns true if the invitation was already accepted. This is a
// pure field accessor (AcceptedAt != nil); the wall-clock expiry/validity
// decision lives in the auth service (services/auth.GuardianInvitationExpired /
// GuardianInvitationValid), per issue #586 (Rule 12).
func (i *GuardianInvitation) IsAccepted() bool {
	return i.AcceptedAt != nil
}

// SetExpiry assigns a duration from now as the expiry
func (i *GuardianInvitation) SetExpiry(duration time.Duration) {
	i.ExpiresAt = time.Now().Add(duration)
}
