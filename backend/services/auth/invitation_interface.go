package auth

import (
	"context"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// InvitationRequest describes the data required to create a new invitation.
type InvitationRequest struct {
	Email            string
	RoleID           int64
	TenantID         int64
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
	// PersonID is the already existing person (without an account) the
	// invitee should become on acceptance. The staff import sets it; the
	// invitation form leaves it nil.
	PersonID   *int64
	CreatedBy  int64
	SchoolName string // Display name of the tenant (shown in invitation email)

	// ActorPermissions are the permissions of the account creating the
	// invitation, used to decide whether it may hand out the requested role
	// (authorize.CanGrantRole). An empty set grants nothing beyond the user
	// tier — the zero value is the safe one, so a caller that forgets this
	// field fails closed instead of silently allowing an admin grant.
	ActorPermissions []string

	// OperatorGrant marks an invitation issued by the platform operator, whose
	// authority comes from the operator portal rather than from tenant
	// permissions (operators carry no tenant permission set at all). Set it
	// only on operator-authenticated paths; it skips the role-grant check.
	OperatorGrant bool
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
	Email            string    `json:"email"`
	RoleName         string    `json:"role_name"`
	FirstName        *string   `json:"first_name,omitempty"`
	LastName         *string   `json:"last_name,omitempty"`
	Position         *string   `json:"position,omitempty"`
	CaregiverEnabled bool      `json:"caregiver_enabled"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// InvitationService defines the operations for managing invitation workflows.
type InvitationService interface {
	CreateInvitation(ctx context.Context, req InvitationRequest) (*authModels.InvitationToken, error)
	ValidateInvitation(ctx context.Context, token string) (*InvitationValidationResult, error)
	AcceptInvitation(ctx context.Context, token string, userData UserRegistrationData) (*authModels.Account, error)
	ResendInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error
	ListPendingInvitations(ctx context.Context) ([]*authModels.InvitationToken, error)
	RevokeInvitation(ctx context.Context, invitationID int64, actorAccountID int64) error
	InvalidatePendingInvitationsByTenantID(ctx context.Context, tenantID int64) (int, error)
	CleanupExpiredInvitations(ctx context.Context) (int, error)
	// GetTenantSubdomainForToken resolves the tenant subdomain for an
	// invitation token — the value used to build {subdomain}.{TENANT_DOMAIN}
	// URLs (#1977). Best-effort: returns "" on any error or for deleted
	// schools, so accept responses can still succeed without it.
	GetTenantSubdomainForToken(ctx context.Context, token string) string
}
