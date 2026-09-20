package test

import "time"

// GuardianInvitation is a fixture row for auth.guardian_invitations. Runtime
// consumers use the Identity & Access capability, never this BUN fixture.
type GuardianInvitation struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID  int64     `bun:"tenant_id,notnull" json:"tenant_id"`

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
