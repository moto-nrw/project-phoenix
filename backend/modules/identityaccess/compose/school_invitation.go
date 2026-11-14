package compose

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-ogs/project-phoenix/modules/identityaccess/ports"
)

// Invitation flows consume root-bound authorization, account proof and mail.
func newSchoolInvitation(
	store *postgres.Store,
	roles *application.RoleAdministration,
	lifecycle *application.AccountLifecycle,
	sessions *identityPorts.SessionDependencies,
	lifecycleDeps *LifecycleDependencies,
	deps *identityPorts.SchoolInvitationDependencies,
	runtime tenantRuntime,
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
		delivery = deps.DeliveryFor(application.NewSchoolInvitationDeliveryRecords(store, attach, runtime))
	}
	if deps.Grants == nil || deps.Owners == nil || delivery == nil || deps.Passwords == nil {
		return nil, errors.New("identity access compose: every invitation dependency is required")
	}
	return application.NewSchoolInvitation(application.SchoolInvitationDependencies{
		PermissionGranted: deps.PermissionGranted,
		EmploymentReset:   deps.EmploymentReset, Store: store, Logins: store, Roles: postgres.NewRoleStore(store),
		Policy:    application.RoleAssignmentPolicy{},
		Grants:    application.InvitationGrantPolicy{Source: deps.Grants},
		Identity:  lifecycle,
		Schools:   application.SchoolDirectory{SchoolDirectory: sessions.Schools},
		Owners:    deps.Owners,
		Passwords: deps.Passwords,
		Delivery:  application.SchoolInvitationDelivery{Source: delivery}, MailStates: delivery.InvitationMailStates,
		Runtime: runtime,
		Attach:  attach,
		Expiry:  deps.Expiry,
		Logger:  deps.Logger,
	})
}
