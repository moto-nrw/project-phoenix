package auth

import (
	"context"
	"errors"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// The school invitation flows are served by Identity & Access (#2722):
// creating the link with its role-grant check, the public preview, the
// acceptance that provisions the account and its identity, resend, revoke
// and the cleanup. InvitationService is the consumer-owned port the
// composition root binds to the public module, so the retained invitation
// routes and the staff import keep their contract.

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
	// OwnerAccessToken is a backend-signed session, never a client-supplied account ID.
	OwnerAccessToken string
	FirstName        string
	LastName         string
	Password         string
	ConfirmPassword  string
}

// InvitationValidationResult represents the public-safe view of an invitation.
type InvitationValidationResult struct {
	TargetPortal         string    `json:"target_portal"`
	RequiresAccountLogin bool      `json:"requires_account_login"`
	Email                string    `json:"email"`
	RoleName             string    `json:"role_name"`
	FirstName            *string   `json:"first_name,omitempty"`
	LastName             *string   `json:"last_name,omitempty"`
	Position             *string   `json:"position,omitempty"`
	CaregiverEnabled     bool      `json:"caregiver_enabled"`
	ExpiresAt            time.Time `json:"expires_at"`
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

var (
	// Invitation errors
	ErrInvitationOwnerRequired       = errors.New("sign in to the invited account before accepting")
	ErrInvitationOwnerMismatch       = errors.New("the signed-in account does not own this invitation")
	ErrInvitationNotFound            = errors.New("invitation not found")
	ErrInvitationExpired             = errors.New("invitation has expired")
	ErrInvitationUsed                = errors.New("invitation has already been used")
	ErrInvitationTenantDeleted       = errors.New("the school for this invitation has been deleted")
	ErrInvitationNameRequired        = errors.New("first name and last name are required")
	ErrAccountAlreadyHasTenantAccess = errors.New("account already has access to tenant")
)

// InvitationRecord is one invitation as the owner reports it; the retained
// service maps it onto the model the invitation routes render.
type InvitationRecord struct {
	ID               int64
	TenantID         int64
	Email            string
	Token            string
	RoleID           int64
	RoleName         string
	ExpiresAt        time.Time
	UsedAt           *time.Time
	CreatedBy        *int64
	CreatorEmail     string
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
	PersonID         *int64
	EmailSentAt      *time.Time
	EmailError       *string
	EmailRetryCount  int
	CreatedAt        time.Time
}

// InvitationAccount is the account an acceptance hands back.
type InvitationAccount struct {
	ID    int64
	Email string
}

// ErrInvitationsUnavailable reports a service composed without the Identity
// & Access invitation port.
var ErrInvitationsUnavailable = errors.New("school invitations are not composed")

// SchoolInvitations is the consumer-owned port over the Identity & Access
// invitation capability. Errors arrive in the AuthError envelope with this
// package's sentinels.
type SchoolInvitations interface {
	CreateInvitation(ctx context.Context, req InvitationRequest) (InvitationRecord, error)
	ValidateInvitation(ctx context.Context, token string) (*InvitationValidationResult, error)
	AcceptInvitation(ctx context.Context, token string, userData UserRegistrationData) (InvitationAccount, error)
	ResendInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	ListPendingInvitations(ctx context.Context) ([]InvitationRecord, error)
	RevokeInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	InvalidatePendingInvitationsByTenantID(ctx context.Context, tenantID int64) (int, error)
	CleanupExpiredInvitations(ctx context.Context) (int, error)
	GetTenantSubdomainForToken(ctx context.Context, token string) string
}

// NewInvitationService serves the retained invitation contract over the
// owner's port: the flows live in Identity & Access, the models the routes
// render are built here.
func NewInvitationService(invitations SchoolInvitations) InvitationService {
	return invitationService{invitations: invitations}
}

type invitationService struct{ invitations SchoolInvitations }

func (s invitationService) port(op string) (SchoolInvitations, error) {
	if s.invitations == nil {
		return nil, &AuthError{Op: op, Err: ErrInvitationsUnavailable}
	}
	return s.invitations, nil
}

func invitationTokenModel(record InvitationRecord) *authModels.InvitationToken {
	token := &authModels.InvitationToken{
		Email: record.Email, Token: record.Token, RoleID: record.RoleID, ExpiresAt: record.ExpiresAt,
		UsedAt: record.UsedAt, CreatedBy: record.CreatedBy, FirstName: record.FirstName, LastName: record.LastName,
		Position: record.Position, CaregiverEnabled: record.CaregiverEnabled, PersonID: record.PersonID,
		EmailSentAt: record.EmailSentAt, EmailError: record.EmailError, EmailRetryCount: record.EmailRetryCount,
	}
	token.ID, token.CreatedAt = record.ID, record.CreatedAt
	token.SetTenantID(record.TenantID)
	if record.RoleName != "" {
		role := &authModels.Role{Name: record.RoleName}
		role.ID = record.RoleID
		token.Role = role
	}
	if record.CreatorEmail != "" {
		creator := &authModels.Account{Email: record.CreatorEmail}
		if record.CreatedBy != nil {
			creator.ID = *record.CreatedBy
		}
		token.Creator = creator
	}
	return token
}

func (s invitationService) CreateInvitation(ctx context.Context, req InvitationRequest) (*authModels.InvitationToken, error) {
	invitations, err := s.port(opCreateInvitation)
	if err != nil {
		return nil, err
	}
	record, err := invitations.CreateInvitation(ctx, req)
	if err != nil {
		return nil, err
	}
	return invitationTokenModel(record), nil
}

func (s invitationService) ValidateInvitation(ctx context.Context, token string) (*InvitationValidationResult, error) {
	invitations, err := s.port(opFetchInvitation)
	if err != nil {
		return nil, err
	}
	return invitations.ValidateInvitation(ctx, token)
}

func (s invitationService) AcceptInvitation(ctx context.Context, token string, userData UserRegistrationData) (*authModels.Account, error) {
	invitations, err := s.port(opAcceptInvitation)
	if err != nil {
		return nil, err
	}
	account, err := invitations.AcceptInvitation(ctx, token, userData)
	if err != nil {
		return nil, err
	}
	model := &authModels.Account{Email: account.Email, Active: true}
	model.ID = account.ID
	return model, nil
}

func (s invitationService) ResendInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	invitations, err := s.port(opResendInvitation)
	if err != nil {
		return err
	}
	return invitations.ResendInvitation(ctx, invitationID, actorAccountID)
}

func (s invitationService) ListPendingInvitations(ctx context.Context) ([]*authModels.InvitationToken, error) {
	invitations, err := s.port("list invitations")
	if err != nil {
		return nil, err
	}
	records, err := invitations.ListPendingInvitations(ctx)
	if err != nil {
		return nil, err
	}
	tokens := make([]*authModels.InvitationToken, 0, len(records))
	for _, record := range records {
		tokens = append(tokens, invitationTokenModel(record))
	}
	return tokens, nil
}

func (s invitationService) RevokeInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	invitations, err := s.port(opRevokeInvitation)
	if err != nil {
		return err
	}
	return invitations.RevokeInvitation(ctx, invitationID, actorAccountID)
}

func (s invitationService) InvalidatePendingInvitationsByTenantID(ctx context.Context, tenantID int64) (int, error) {
	invitations, err := s.port("invalidate invitations by tenant")
	if err != nil {
		return 0, err
	}
	return invitations.InvalidatePendingInvitationsByTenantID(ctx, tenantID)
}

func (s invitationService) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	invitations, err := s.port("cleanup invitations")
	if err != nil {
		return 0, err
	}
	return invitations.CleanupExpiredInvitations(ctx)
}

func (s invitationService) GetTenantSubdomainForToken(ctx context.Context, token string) string {
	invitations, err := s.port(opFetchInvitation)
	if err != nil {
		return ""
	}
	return invitations.GetTenantSubdomainForToken(ctx, token)
}

// Operation names the invitation flows report.
const (
	opCreateInvitation = "create invitation"
	opAcceptInvitation = "accept invitation"
	opResendInvitation = "resend invitation"
	opRevokeInvitation = "revoke invitation"
	opFetchInvitation  = "fetch invitation"
)
