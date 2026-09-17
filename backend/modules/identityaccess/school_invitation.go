package identityaccess

import (
	"context"
	"errors"
	"time"
)

// School invitations (#2722): the one-time link that gives an invitee access
// to a school with a role. A link is never stored already expired or spent,
// is redeemable exactly once, and accepting it writes the account, its
// school mapping, the role and the identity chain the role needs in one
// transaction.
var (
	// ErrSchoolInvitationUnavailable reports a module composed without the
	// invitation dependencies.
	ErrSchoolInvitationUnavailable = errors.New("school invitations are not composed")
	ErrInvitationNotFound          = errors.New("invitation not found")
	ErrInvitationExpired           = errors.New("invitation has expired")
	ErrInvitationUsed              = errors.New("invitation has already been used")
	ErrInvitationTenantDeleted     = errors.New("the school for this invitation has been deleted")
	ErrInvitationNameRequired      = errors.New("first name and last name are required")
	// ErrInvitationOwnerRequired reports an acceptance for an address that
	// already has an account, without a session of that account.
	ErrInvitationOwnerRequired = errors.New("sign in to the invited account before accepting")
	ErrInvitationOwnerMismatch = errors.New("the signed-in account does not own this invitation")
	// ErrAccountAlreadyHasTenantAccess reports an invitation to a school the
	// account can already sign in to.
	ErrAccountAlreadyHasTenantAccess = errors.New("account already has access to tenant")
	// ErrInvitationPasswordMismatch reports an acceptance whose password
	// fields differ.
	ErrInvitationPasswordMismatch = errors.New("passwords don't match")
	// ErrRoleGrantNotPermitted reports an inviter handing out a role beyond
	// its own authority: an admin-tier grant requires users:manage.
	ErrRoleGrantNotPermitted = errors.New("Du darfst diese Rolle nicht vergeben") //nolint:staticcheck // ST1005: user-facing German message
	// ErrLehrkraftNoCaregiver refuses the combination of the Lehrkraft role
	// with the caregiver upgrade, which would defeat the role's class-scoped
	// read-only design (#1772).
	ErrLehrkraftNoCaregiver = errors.New("Die Rolle 'Lehrkraft' kann nicht mit Betreuungsrechten kombiniert werden") //nolint:staticcheck // ST1005: user-facing German message
)

// InvitationPortal names where an invitation is accepted.
type InvitationPortal string

const (
	InvitationPortalTenant InvitationPortal = "tenant"
	InvitationPortalSchool InvitationPortal = "school"
)

// SchoolInvitation is an invitation as the owner reports it.
type SchoolInvitation struct {
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
	Delivery         TokenDelivery
	CreatedAt        time.Time
}

// SchoolInvitationRequest describes the invitation to create.
type SchoolInvitationRequest struct {
	Email            string
	RoleID           int64
	TenantID         int64
	FirstName        *string
	LastName         *string
	Position         *string
	CaregiverEnabled bool
	PersonID         *int64
	CreatedBy        int64
	SchoolName       string
	// ActorPermissions are the inviting account's permissions; the empty set
	// grants nothing beyond the user tier.
	ActorPermissions []string
	// OperatorGrant marks an invitation issued from the operator portal.
	OperatorGrant bool
}

// InvitationRegistration is what an invitee supplies when accepting.
type InvitationRegistration struct {
	// OwnerAccessToken proves that the caller holds the invited account's
	// session; it is a backend-signed token, never an account id.
	OwnerAccessToken string
	FirstName        string
	LastName         string
	Password         string
	ConfirmPassword  string
}

// InvitationPreview is the public-safe view of a redeemable invitation.
type InvitationPreview struct {
	Portal               InvitationPortal
	RequiresAccountLogin bool
	Email                string
	RoleName             string
	FirstName            *string
	LastName             *string
	Position             *string
	CaregiverEnabled     bool
	ExpiresAt            time.Time
}

// SchoolInvitations is the capability the invitation routes and the staff
// import consume. Flow errors arrive in the AuthenticationError envelope.
type SchoolInvitations interface {
	CreateSchoolInvitation(ctx context.Context, request SchoolInvitationRequest) (SchoolInvitation, error)
	ValidateSchoolInvitation(ctx context.Context, token string) (InvitationPreview, error)
	// AcceptSchoolInvitation returns the account the invitee signs in with.
	AcceptSchoolInvitation(ctx context.Context, token string, registration InvitationRegistration) (Account, error)
	ResendSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	RevokeSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	ListPendingSchoolInvitations(ctx context.Context) ([]SchoolInvitation, error)
	// RevokeTenantSchoolInvitations spends every pending invitation of the
	// school and returns how many it spent.
	RevokeTenantSchoolInvitations(ctx context.Context, tenantID int64) (int, error)
	DeleteExpiredSchoolInvitations(ctx context.Context) (int, error)
	RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
	// SchoolInvitationSubdomain resolves the school host of an accepted
	// invitation; it is best-effort and answers "" when it cannot.
	SchoolInvitationSubdomain(ctx context.Context, token string) string
}

func (m *Module) CreateSchoolInvitation(ctx context.Context, request SchoolInvitationRequest) (SchoolInvitation, error) {
	return m.engine.CreateSchoolInvitation(ctx, request)
}

func (m *Module) ValidateSchoolInvitation(ctx context.Context, token string) (InvitationPreview, error) {
	return m.engine.ValidateSchoolInvitation(ctx, token)
}

func (m *Module) AcceptSchoolInvitation(ctx context.Context, token string, registration InvitationRegistration) (Account, error) {
	return m.engine.AcceptSchoolInvitation(ctx, token, registration)
}

func (m *Module) ResendSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	return m.engine.ResendSchoolInvitation(ctx, invitationID, actorAccountID)
}

func (m *Module) RevokeSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	return m.engine.RevokeSchoolInvitation(ctx, invitationID, actorAccountID)
}

func (m *Module) ListPendingSchoolInvitations(ctx context.Context) ([]SchoolInvitation, error) {
	return m.engine.ListPendingSchoolInvitations(ctx)
}

func (m *Module) RevokeTenantSchoolInvitations(ctx context.Context, tenantID int64) (int, error) {
	return m.engine.RevokeTenantSchoolInvitations(ctx, tenantID)
}

func (m *Module) DeleteExpiredSchoolInvitations(ctx context.Context) (int, error) {
	return m.engine.DeleteExpiredSchoolInvitations(ctx)
}

func (m *Module) RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery TokenDelivery) error {
	return m.engine.RecordSchoolInvitationDelivery(ctx, id, delivery)
}

func (m *Module) SchoolInvitationSubdomain(ctx context.Context, token string) string {
	return m.engine.SchoolInvitationSubdomain(ctx, token)
}
