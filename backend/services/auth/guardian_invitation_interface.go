package auth

import (
	"context"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// GuardianInvitationCreateRequest captures the data needed to issue a guardian
// invitation. Email is taken from the GuardianProfile; the caller passes the
// profile ID and the inviting account ID. Tenant comes from context.
type GuardianInvitationCreateRequest struct {
	GuardianProfileID int64
	CreatedBy         int64
}

// GuardianInvitationAcceptData captures the form payload submitted on the
// accept-guardian-invite page.
type GuardianInvitationAcceptData struct {
	Password        string
	ConfirmPassword string
}

// GuardianInvitationValidation is the public-safe view of an invitation.
// Returned by ValidateInvitation so the accept page can pre-fill the email
// and the guardian's first/last name without exposing internal IDs.
type GuardianInvitationValidation struct {
	Email         string    `json:"email"`
	FirstName     string    `json:"first_name,omitempty"`
	LastName      string    `json:"last_name,omitempty"`
	ExpiresAt     time.Time `json:"expires_at"`
	SchoolName    string    `json:"school_name,omitempty"`
	TenantSlug    string    `json:"tenant_slug,omitempty"`
	SchoolLogoURL string    `json:"school_logo_url,omitempty"`
}

// GuardianInvitationService manages the lifecycle of guardian-account
// invitations. Mirrors the staff InvitationService pattern but writes to
// auth.accounts (not the orphaned auth.accounts_parents) and assigns the
// guardian base role. Created on first child approval (PR 8); accepted by
// the guardian via the public accept-guardian-invite page (this PR).
type GuardianInvitationService interface {
	Create(ctx context.Context, req GuardianInvitationCreateRequest) (*authModels.GuardianInvitation, error)
	Validate(ctx context.Context, token string) (*GuardianInvitationValidation, error)
	Accept(ctx context.Context, token string, data GuardianInvitationAcceptData) (*authModels.Account, error)
	Resend(ctx context.Context, invitationID int64, actorAccountID int64) error

	// Related-accounts management (invite further guardians to a child,
	// approve/reject parent-initiated requests, revoke an account's access).
	// Shared by the staff "Erziehungsberechtigte" tab and the parents portal.
	InviteToStudent(ctx context.Context, req InviteToStudentRequest) (*InviteToStudentResult, error)
	ApproveInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error
	RejectInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error
	PendingInvitationStudentID(ctx context.Context, invitationID int64) (int64, error)
	ListPendingApprovalsDetailed(ctx context.Context) ([]*PendingApprovalView, error)
	RevokeAccess(ctx context.Context, req RevokeAccessRequest) error

	// DeliveryStatus reports what actually happened to the invitation emails
	// for an invitation: one entry per dispatch attempt, newest first,
	// separating "handed to the mail server" from "arrived at the recipient".
	DeliveryStatus(ctx context.Context, invitationID int64) (*GuardianInvitationDelivery, error)

	// GetTenantSlugForToken resolves the tenant slug for a guardian
	// invitation token. Best-effort: returns "" on any error or for deleted
	// schools (issue #584: moved verbatim out of api/auth).
	GetTenantSlugForToken(ctx context.Context, token string) string
}

// GuardianSettingsResolver is the narrow contract the guardian invitation
// service needs from the settings service. Defined locally so the auth
// package does not import the full config service. Pass nil to fall back to
// env-var → default expiry.
type GuardianSettingsResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}
