package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

// The school invitation flows (#2722) need three seams the root binds: the
// role-grant decision of Security Runtime, the access token that proves an
// invitee owns the invited account, and the mail.

func newSchoolInvitation(
	store *postgres.Store,
	roles *application.RoleAdministration,
	lifecycle *application.AccountLifecycle,
	sessions *identityPorts.SessionDependencies,
	lifecycleDeps *LifecycleDependencies,
	deps *identityPorts.SchoolInvitationDependencies,
) (*application.SchoolInvitation, error) {
	if deps == nil {
		return nil, nil
	}
	if sessions == nil || lifecycleDeps == nil || roles == nil || lifecycle == nil {
		return nil, errors.New("identity access compose: the invitation flows require the session and lifecycle dependencies")
	}
	attach := sessions.TenantRuntime
	var delivery identityPorts.SchoolInvitationDelivery
	if deps.DeliveryFor != nil {
		delivery = deps.DeliveryFor(application.NewSchoolInvitationDeliveryRecords(store, attach, tenantRuntime{attach: attach, runner: application.NewTransactionRunner()}))
	}
	if deps.Grants == nil || deps.Owners == nil || delivery == nil || deps.Passwords == nil {
		return nil, errors.New("identity access compose: every invitation dependency is required")
	}
	return application.NewSchoolInvitation(application.SchoolInvitationDependencies{
		EmploymentReset: deps.EmploymentReset, Store: store, Logins: store, Roles: postgres.NewRoleStore(store),
		Policy:    application.RoleAssignmentPolicy{},
		Grants:    application.InvitationGrantPolicy{Source: deps.Grants},
		Identity:  lifecycle,
		Schools:   application.SchoolDirectory{Source: sessions.Schools},
		Owners:    deps.Owners,
		Passwords: deps.Passwords,
		Delivery:  application.SchoolInvitationDelivery{Source: delivery},
		Runtime:   tenantRuntime{attach: attach, runner: application.NewTransactionRunner()},
		Expiry:    deps.Expiry,
		Logger:    deps.Logger,
	})
}

// --- engine methods --------------------------------------------------------
