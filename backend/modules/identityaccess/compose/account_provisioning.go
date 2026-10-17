package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
)

// newAccountProvisioning composes the registration and link flows (#3332).
// They need no dependency of their own: the role storage, the password
// policy, the caregiver-profile fact and the identity chain are the ones the
// lifecycle flows already bind.
func newAccountProvisioning(
	store *postgres.Store,
	lifecycle *application.AccountLifecycle,
	sessions *SessionDependencies,
	deps *LifecycleDependencies,
) (*application.AccountProvisioning, error) {
	if deps == nil {
		return nil, nil
	}
	if sessions == nil || lifecycle == nil {
		return nil, errors.New("identity access compose: the provisioning flows require the session and lifecycle dependencies")
	}
	attach := sessions.TenantRuntime
	if attach == nil {
		attach = func(ctx context.Context) context.Context { return ctx }
	}
	return application.NewAccountProvisioning(application.AccountProvisioningDependencies{
		Accounts:  store,
		Logins:    store,
		RoleStore: store,
		Roles:     postgres.NewRoleStore(store),
		Policy:    roleAssignmentPolicy{},
		Identity:  lifecycle,
		Profiles:  staffDirectory{deps.Staff},
		Passwords: deps.Passwords,
		Runtime:   tenantRuntime{attach: attach, runner: newTransactionRunner()},
		Logger:    deps.Logger,
	})
}

// --- engine methods --------------------------------------------------------
