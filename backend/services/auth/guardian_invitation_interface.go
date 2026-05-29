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
	CleanupExpired(ctx context.Context) (int, error)
}

// GuardianSettingsResolver is the narrow contract the guardian invitation
// service needs from the settings service. Defined locally so the auth
// package does not import the full config service. Pass nil to fall back to
// env-var → default expiry.
type GuardianSettingsResolver interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}
