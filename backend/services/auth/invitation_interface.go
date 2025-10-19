package auth

import (
	"context"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// InvitationRequest describes the data required to create a new invitation.
type InvitationRequest struct {
	Email     string
	RoleID    int64
	FirstName *string
	LastName  *string
	CreatedBy int64
}

// UserRegistrationData captures the information supplied when accepting an invitation.
type UserRegistrationData struct {
	FirstName       string
	LastName        string
	Password        string
	ConfirmPassword string
}

// InvitationValidationResult represents the public-safe view of an invitation.
type InvitationValidationResult struct {
	Email     string    `json:"email"`
	RoleName  string    `json:"role_name"`
	FirstName *string   `json:"first_name,omitempty"`
	LastName  *string   `json:"last_name,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// InvitationService defines the operations for managing invitation workflows.
type InvitationService interface {
	base.TransactionalService

	CreateInvitation(ctx context.Context, req InvitationRequest) (*authModels.InvitationToken, error)
	ValidateInvitation(ctx context.Context, token string) (*InvitationValidationResult, error)
	AcceptInvitation(ctx context.Context, token string, userData UserRegistrationData) (*authModels.Account, error)
	ResendInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error
	ListPendingInvitations(ctx context.Context) ([]*authModels.InvitationToken, error)
	RevokeInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error
	CleanupExpiredInvitations(ctx context.Context) (int, error)
}
