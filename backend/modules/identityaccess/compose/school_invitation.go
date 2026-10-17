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
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The school invitation flows (#2722) need three seams the root binds: the
// role-grant decision of Security Runtime, the access token that proves an
// invitee owns the invited account, and the mail.

// InvitationOwnerTokens verifies that the caller holds a live session of the
// invited account.
type InvitationOwnerTokens interface {
	// AccountOfAccessToken returns the account a usable access token proves,
	// or zero when the token proves nothing.
	AccountOfAccessToken(token string) (int64, error)
}

// SchoolInvitationDependencies compose the invitation flows. They require
// the session and lifecycle dependencies: the flows reuse the school
// directory, the role storage and the identity chain.
type SchoolInvitationDependencies struct {
	Grants    identityPorts.InvitationGrantPolicy
	Owners    InvitationOwnerTokens
	Delivery  identityPorts.SchoolInvitationDelivery
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
		Store: store, Logins: store, Roles: postgres.NewRoleStore(store),
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

type invitationGrantPolicy struct {
	source identityPorts.InvitationGrantPolicy
}

func (p invitationGrantPolicy) CanGrantRole(role domain.RoleFacts, actorPermissions, rolePermissions []string) bool {
	return p.source.CanGrantRole(identityaccess.RoleFacts(role), actorPermissions, rolePermissions)
}

type schoolInvitationDelivery struct {
	source identityPorts.SchoolInvitationDelivery
}

func (d schoolInvitationDelivery) DispatchSchoolInvitation(ctx context.Context, invitation domain.SchoolInvitation, schoolName string, portal domain.InvitationPortal, expiry time.Duration) {
	d.source.DispatchSchoolInvitation(ctx, application.PublicSchoolInvitation(invitation), schoolName, identityaccess.InvitationPortal(portal), expiry)
}

func (d schoolInvitationDelivery) QueueSchoolWelcome(ctx context.Context, invitation domain.SchoolInvitation, schoolName string, portal domain.InvitationPortal, rolePermissions []string) error {
	return d.source.QueueSchoolWelcome(ctx, application.PublicSchoolInvitation(invitation), schoolName, identityaccess.InvitationPortal(portal), rolePermissions)
}

// --- engine methods --------------------------------------------------------

// invitationLogger is the logger the always-composed invitation maintenance
// uses; compositions without the invitation seams fall back to the default.
func invitationLogger(deps *SchoolInvitationDependencies) *slog.Logger {
	if deps == nil {
		return slog.Default()
	}
	return deps.Logger
}
