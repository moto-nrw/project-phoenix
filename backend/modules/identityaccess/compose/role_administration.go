package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	internalports "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// newRoleAdministration composes the role administration. The school identity
// is read at call time because the lifecycle flows it belongs to are composed
// after the administration they remove roles through.
func newRoleAdministration(auth *application.AccountAuthentication, runtime internalports.Runtime, store internalports.RoleStore, accounts internalports.ManageableAccounts, deps *LifecycleDependencies, identity func() *application.AccountLifecycle) (*application.RoleAdministration, error) {
	return application.NewRoleAdministration(application.RoleAdministrationDependencies{
		Store:         store,
		Accounts:      accounts,
		Profiles:      application.StaffDirectory{StaffDirectory: deps.Staff},
		Policy:        application.RoleAssignmentPolicy{},
		IdentityRoles: application.RolePolicy{},
		Identity:      application.LateSchoolIdentity{Current: identity},
		Sessions:      auth,
		Runtime:       runtime,
		Attach:        auth.BindRuntime,
		Logger:        deps.Logger,
	})
}
