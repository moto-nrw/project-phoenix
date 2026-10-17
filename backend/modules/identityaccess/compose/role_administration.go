package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// newRoleAdministration composes the role administration. The school identity
// is read at call time because the lifecycle flows it belongs to are composed
// after the administration they remove roles through.
func newRoleAdministration(auth *application.AccountAuthentication, runtime ports.Runtime, store ports.RoleStore, accounts ports.ManageableAccounts, deps *LifecycleDependencies, identity func() *application.AccountLifecycle) (*application.RoleAdministration, error) {
	return application.NewRoleAdministration(application.RoleAdministrationDependencies{
		Store:         store,
		Accounts:      accounts,
		Profiles:      staffDirectory{deps.Staff},
		Policy:        roleAssignmentPolicy{},
		IdentityRoles: rolePolicy{},
		Identity:      lateSchoolIdentity{current: identity},
		Sessions:      auth,
		Runtime:       runtime,
		Logger:        deps.Logger,
	})
}

type lateSchoolIdentity struct {
	current func() *application.AccountLifecycle
}

func (i lateSchoolIdentity) EnsureSchoolIdentity(ctx context.Context, input domain.SchoolIdentityInput) (*domain.SchoolIdentity, error) {
	lifecycle := i.current()
	if lifecycle == nil {
		return nil, errors.New("identity access compose: the school identity chain is not composed")
	}
	return lifecycle.EnsureSchoolIdentity(ctx, input)
}

// accountAdministration serves the offboarding port: role removals run
// through the role administration so their session revocation holds, the
// deactivation through the account administration, which schedules the
// account-wide session wipe with it (#3332).
type accountAdministration struct {
	roles    *application.RoleAdministration
	accounts *application.AccountAdministration
}

func (a accountAdministration) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error {
	return a.roles.RemoveRoleFromAccount(ctx, accountID, roleID)
}

func (a accountAdministration) DeactivateAccount(ctx context.Context, accountID int64) error {
	return a.accounts.DeactivateAccount(ctx, accountID)
}

func (d staffDirectory) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return d.source.HasLiveCaregiverProfile(ctx, accountID)
}

// roleAssignmentPolicy binds the public assignment rules to the application
// port.
type roleAssignmentPolicy struct{}

func (roleAssignmentPolicy) IsLehrkraftSystemRole(role *domain.RoleFacts) bool {
	return identityaccess.IsLehrkraftSystemRole(publicRoleFacts(role))
}

func (roleAssignmentPolicy) IsGuardianTierRole(role *domain.RoleFacts) bool {
	return identityaccess.IsGuardianTierRole(publicRoleFacts(role))
}

func (roleAssignmentPolicy) IsPlatformCaregiverRole(role *domain.RoleFacts) bool {
	return identityaccess.IsPlatformCaregiverRole(publicRoleFacts(role))
}

func (roleAssignmentPolicy) ValidateAssignableSchoolRole(role *domain.RoleFacts, tenantID int64) error {
	return identityaccess.ValidateAssignableSchoolRole(publicRoleFacts(role), tenantID)
}

// --- engine methods -------------------------------------------------------
