package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The school invitation flows (#2722) need three seams the root binds: the
// role-grant decision of Security Runtime, the access token that proves an
// invitee owns the invited account, and the mail.

// InvitationGrantPolicy answers whether the inviting account may hand out
// the role, given its own permissions and the role's.
type InvitationGrantPolicy interface {
	CanGrantRole(role identityaccess.RoleFacts, actorPermissions, rolePermissions []string) bool
}

// InvitationOwnerTokens verifies that the caller holds a live session of the
// invited account.
type InvitationOwnerTokens interface {
	// AccountOfAccessToken returns the account a usable access token proves,
	// or zero when the token proves nothing.
	AccountOfAccessToken(token string) (int64, error)
}

// SchoolInvitationDelivery mails an invitation. The module decides the
// portal; the root resolves its host and the reply-to identity and records
// the outcome back through the module.
type SchoolInvitationDelivery interface {
	DispatchSchoolInvitation(ctx context.Context, invitation identityaccess.SchoolInvitation, schoolName string, portal identityaccess.InvitationPortal, expiry time.Duration)
}

// SchoolInvitationDependencies compose the invitation flows. They require
// the session and lifecycle dependencies: the flows reuse the school
// directory, the role storage and the identity chain.
type SchoolInvitationDependencies struct {
	Grants    InvitationGrantPolicy
	Owners    InvitationOwnerTokens
	Delivery  SchoolInvitationDelivery
	Passwords PasswordPolicy
	Expiry    time.Duration
	Logger    *slog.Logger
}

func newSchoolInvitation(
	store *postgres.Store,
	roles *application.RoleAdministration,
	lifecycle *application.AccountLifecycle,
	sessions *SessionDependencies,
	lifecycleDeps *LifecycleDependencies,
	deps *SchoolInvitationDependencies,
) (*application.SchoolInvitation, error) {
	if deps == nil {
		return nil, nil
	}
	if sessions == nil || lifecycleDeps == nil || roles == nil || lifecycle == nil {
		return nil, errors.New("identity access compose: the invitation flows require the session and lifecycle dependencies")
	}
	if deps.Grants == nil || deps.Owners == nil || deps.Delivery == nil || deps.Passwords == nil {
		return nil, errors.New("identity access compose: every invitation dependency is required")
	}
	attach := sessions.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewSchoolInvitation(application.SchoolInvitationDependencies{
		Store: store, Logins: store, Roles: roleStore{lifecycleDeps.Roles},
		Policy:    roleAssignmentPolicy{},
		Grants:    invitationGrantPolicy{deps.Grants},
		Identity:  lifecycle,
		Schools:   schoolDirectory{sessions.Schools},
		Owners:    deps.Owners,
		Passwords: deps.Passwords,
		Delivery:  schoolInvitationDelivery{deps.Delivery},
		Runtime:   tenantRuntime{attach: attach, runner: newTransactionRunner()},
		Expiry:    deps.Expiry,
		Logger:    deps.Logger,
	})
}

type invitationGrantPolicy struct{ source InvitationGrantPolicy }

func (p invitationGrantPolicy) CanGrantRole(role domain.RoleFacts, actorPermissions, rolePermissions []string) bool {
	return p.source.CanGrantRole(identityaccess.RoleFacts(role), actorPermissions, rolePermissions)
}

type schoolInvitationDelivery struct{ source SchoolInvitationDelivery }

func (d schoolInvitationDelivery) DispatchSchoolInvitation(ctx context.Context, invitation domain.SchoolInvitation, schoolName string, portal domain.InvitationPortal, expiry time.Duration) {
	d.source.DispatchSchoolInvitation(ctx, publicSchoolInvitation(invitation), schoolName, identityaccess.InvitationPortal(portal), expiry)
}

func publicSchoolInvitation(invitation domain.SchoolInvitation) identityaccess.SchoolInvitation {
	return identityaccess.SchoolInvitation{
		ID: invitation.ID, TenantID: invitation.TenantID, Email: invitation.Email, Token: invitation.Token,
		RoleID: invitation.RoleID, RoleName: invitation.RoleName, ExpiresAt: invitation.ExpiresAt, UsedAt: invitation.UsedAt,
		CreatedBy: invitation.CreatedBy, FirstName: invitation.FirstName, LastName: invitation.LastName,
		Position: invitation.Position, CaregiverEnabled: invitation.CaregiverEnabled, PersonID: invitation.PersonID,
		Delivery: identityaccess.TokenDelivery(invitation.Delivery), CreatedAt: invitation.CreatedAt,
	}
}

// --- engine methods --------------------------------------------------------

var errSchoolInvitationUnavailable = identityaccess.ErrSchoolInvitationUnavailable

func (e engine) CreateSchoolInvitation(ctx context.Context, request identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error) {
	if e.invitations == nil {
		return identityaccess.SchoolInvitation{}, errSchoolInvitationUnavailable
	}
	invitation, err := e.invitations.CreateInvitation(e.attach(ctx), domain.SchoolInvitationRequest{
		Email: request.Email, RoleID: request.RoleID, TenantID: request.TenantID, FirstName: request.FirstName,
		LastName: request.LastName, Position: request.Position, CaregiverEnabled: request.CaregiverEnabled,
		PersonID: request.PersonID, CreatedBy: request.CreatedBy, SchoolName: request.SchoolName,
		ActorPermissions: request.ActorPermissions, OperatorGrant: request.OperatorGrant,
	})
	return publicSchoolInvitation(invitation), invitationError(err)
}

func (e engine) ValidateSchoolInvitation(ctx context.Context, token string) (identityaccess.InvitationPreview, error) {
	if e.invitations == nil {
		return identityaccess.InvitationPreview{}, errSchoolInvitationUnavailable
	}
	preview, err := e.invitations.ValidateInvitation(e.attach(ctx), token)
	return identityaccess.InvitationPreview{
		Portal: identityaccess.InvitationPortal(preview.Portal), RequiresAccountLogin: preview.RequiresAccountLogin,
		Email: preview.Email, RoleName: preview.RoleName, FirstName: preview.FirstName, LastName: preview.LastName,
		Position: preview.Position, CaregiverEnabled: preview.CaregiverEnabled, ExpiresAt: preview.ExpiresAt,
	}, invitationError(err)
}

func (e engine) AcceptSchoolInvitation(ctx context.Context, token string, registration identityaccess.InvitationRegistration) (identityaccess.Account, error) {
	if e.invitations == nil {
		return identityaccess.Account{}, errSchoolInvitationUnavailable
	}
	account, err := e.invitations.AcceptInvitation(e.attach(ctx), token, domain.InvitationRegistration{
		OwnerAccessToken: registration.OwnerAccessToken, FirstName: registration.FirstName, LastName: registration.LastName,
		Password: registration.Password, ConfirmPassword: registration.ConfirmPassword,
	})
	return identityaccess.Account{ID: account.ID, Email: account.Email}, invitationError(err)
}

func (e engine) ResendSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	if e.invitations == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(e.invitations.ResendInvitation(e.attach(ctx), invitationID, actorAccountID))
}

func (e engine) RevokeSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	if e.invitations == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(e.invitations.RevokeInvitation(e.attach(ctx), invitationID, actorAccountID))
}

func (e engine) ListPendingSchoolInvitations(ctx context.Context) ([]identityaccess.SchoolInvitation, error) {
	if e.invitations == nil {
		return nil, errSchoolInvitationUnavailable
	}
	invitations, err := e.invitations.ListPendingInvitations(e.attach(ctx))
	if err != nil {
		return nil, invitationError(err)
	}
	result := make([]identityaccess.SchoolInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, publicSchoolInvitation(invitation))
	}
	return result, nil
}

func (e engine) RevokeTenantSchoolInvitations(ctx context.Context, tenantID int64) (int, error) {
	revoked, err := e.invitationMaintenance.RevokeTenantInvitations(e.attach(ctx), tenantID)
	return revoked, invitationError(err)
}

func (e engine) DeleteExpiredSchoolInvitations(ctx context.Context) (int, error) {
	deleted, err := e.invitationMaintenance.CleanupExpiredInvitations(e.attach(ctx))
	return deleted, invitationError(err)
}

func (e engine) RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery identityaccess.TokenDelivery) error {
	if e.invitations == nil {
		return errSchoolInvitationUnavailable
	}
	return invitationError(e.invitations.RecordInvitationDelivery(e.attach(ctx), id, domain.TokenDelivery(delivery)))
}

func (e engine) SchoolInvitationSubdomain(ctx context.Context, token string) string {
	if e.invitations == nil {
		return ""
	}
	return e.invitations.InvitationSubdomain(e.attach(ctx), token)
}

var invitationSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrInvitationNotFound, identityaccess.ErrInvitationNotFound},
	{domain.ErrInvitationExpired, identityaccess.ErrInvitationExpired},
	{domain.ErrInvitationUsed, identityaccess.ErrInvitationUsed},
	{domain.ErrInvitationTenantDeleted, identityaccess.ErrInvitationTenantDeleted},
	{domain.ErrInvitationNameRequired, identityaccess.ErrInvitationNameRequired},
	{domain.ErrInvitationOwnerRequired, identityaccess.ErrInvitationOwnerRequired},
	{domain.ErrInvitationOwnerMismatch, identityaccess.ErrInvitationOwnerMismatch},
	{domain.ErrAccountAlreadyHasTenantAccess, identityaccess.ErrAccountAlreadyHasTenantAccess},
	{domain.ErrPasswordMismatch, identityaccess.ErrInvitationPasswordMismatch},
	{domain.ErrRoleGrantNotPermitted, identityaccess.ErrRoleGrantNotPermitted},
	{domain.ErrLehrkraftNoCaregiver, identityaccess.ErrLehrkraftNoCaregiver},
	{domain.ErrEmailAlreadyExists, identityaccess.ErrEmailAlreadyExists},
	{domain.ErrRoleNotFound, identityaccess.ErrRoleNotAssignable},
	{domain.ErrTransactionUnusable, identityaccess.ErrTransactionUnusable},
}

// invitationError translates a flow error to the public contract: the
// operation envelope keeps its text, an invitation sentinel gains its public
// twin, and everything else falls through to the lifecycle mapping, which
// carries the role-policy and account sentinels the flows share.
func invitationError(err error) error {
	if err == nil {
		return nil
	}
	var operation *application.OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: invitationError(operation.Err)}
	}
	for _, sentinel := range invitationSentinels {
		if !errors.Is(err, sentinel.internal) {
			continue
		}
		if err == sentinel.internal {
			return sentinel.public
		}
		return &translatedError{text: err.Error(), public: sentinel.public, cause: err}
	}
	return lifecycleError(err)
}

// invitationLogger is the logger the always-composed invitation maintenance
// uses; compositions without the invitation seams fall back to the default.
func invitationLogger(deps *SchoolInvitationDependencies) *slog.Logger {
	if deps == nil {
		return slog.Default()
	}
	return deps.Logger
}
