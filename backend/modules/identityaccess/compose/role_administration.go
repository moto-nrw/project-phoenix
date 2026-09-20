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

var errRoleAdministrationUnavailable = identityaccess.ErrRoleAdministrationUnavailable

func publicRoles(roles []domain.ManagedRole) []identityaccess.Role {
	if roles == nil {
		return nil
	}
	result := make([]identityaccess.Role, 0, len(roles))
	for _, role := range roles {
		result = append(result, identityaccess.Role(role))
	}
	return result
}

func publicPermissions(permissions []domain.ManagedPermission) []identityaccess.Permission {
	if permissions == nil {
		return nil
	}
	result := make([]identityaccess.Permission, 0, len(permissions))
	for _, permission := range permissions {
		result = append(result, identityaccess.Permission(permission))
	}
	return result
}

func (e engine) GetRole(ctx context.Context, id int64) (identityaccess.Role, error) {
	if e.roles == nil {
		return identityaccess.Role{}, errRoleAdministrationUnavailable
	}
	role, err := e.roles.GetRole(e.attach(ctx), id)
	return identityaccess.Role(role), roleError(err)
}

func (e engine) ListRoles(ctx context.Context, filter identityaccess.RoleFilter) ([]identityaccess.Role, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	roles, err := e.roles.ListRoles(e.attach(ctx), domain.RoleFilter(filter))
	return publicRoles(roles), roleError(err)
}

func (e engine) ResolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (*identityaccess.AssignableSchoolRole, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	role, permissions, err := e.roles.ResolveAssignableSchoolRole(e.attach(ctx), roleID, tenantID)
	if err != nil {
		return nil, roleError(err)
	}
	return &identityaccess.AssignableSchoolRole{Role: identityaccess.Role(role), Permissions: permissions}, nil
}

func (e engine) GetAccountRoles(ctx context.Context, accountID int64) ([]identityaccess.Role, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	roles, err := e.roles.GetAccountRoles(e.attach(ctx), accountID)
	return publicRoles(roles), roleError(err)
}

func (e engine) AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error) {
	if e.roles == nil {
		return false, errRoleAdministrationUnavailable
	}
	holds, err := e.roles.AccountHoldsLehrkraftRole(e.attach(ctx), accountID)
	return holds, roleError(err)
}

func (e engine) GetAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	names, err := e.roles.GetAccountRoleNames(e.attach(ctx), accountIDs)
	return names, roleError(err)
}

func (e engine) GetAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	emails, err := e.roles.GetAccountEmails(e.attach(ctx), accountIDs)
	return emails, roleError(err)
}

func (e engine) GetAccountAvatars(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	avatars, err := e.roles.GetAccountAvatars(e.attach(ctx), accountIDs)
	return avatars, roleError(err)
}

func (e engine) CreateRole(ctx context.Context, name, description string, baseRole *string) (identityaccess.Role, error) {
	if e.roles == nil {
		return identityaccess.Role{}, errRoleAdministrationUnavailable
	}
	role, err := e.roles.CreateRole(e.attach(ctx), name, description, baseRole)
	return identityaccess.Role(role), roleError(err)
}

func (e engine) UpdateRole(ctx context.Context, role identityaccess.Role) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.UpdateRole(e.attach(ctx), domain.ManagedRole(role)))
}

func (e engine) DeleteRole(ctx context.Context, id int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.DeleteRole(e.attach(ctx), id))
}

func (e engine) AssignRoleToAccount(ctx context.Context, accountID, roleID int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.AssignRoleToAccount(e.attach(ctx), accountID, roleID))
}

func (e engine) ReplaceAccountRole(ctx context.Context, accountID, roleID int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.ReplaceAccountRole(e.attach(ctx), accountID, roleID))
}

func (e engine) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.RemoveRoleFromAccount(e.attach(ctx), accountID, roleID))
}

func (e engine) GetPermission(ctx context.Context, id int64) (identityaccess.Permission, error) {
	if e.roles == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	permission, err := e.roles.GetPermission(e.attach(ctx), id)
	return identityaccess.Permission(permission), roleError(err)
}

func (e engine) GetPermissionByName(ctx context.Context, name string) (identityaccess.Permission, error) {
	if e.roles == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	permission, err := e.roles.GetPermissionByName(e.attach(ctx), name)
	return identityaccess.Permission(permission), roleError(err)
}

func (e engine) ListPermissions(ctx context.Context, filter identityaccess.PermissionFilter) ([]identityaccess.Permission, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	permissions, err := e.roles.ListPermissions(e.attach(ctx), domain.PermissionFilter(filter))
	return publicPermissions(permissions), roleError(err)
}

func (e engine) GetRolePermissions(ctx context.Context, roleID int64) ([]identityaccess.Permission, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	permissions, err := e.roles.GetRolePermissions(e.attach(ctx), roleID)
	return publicPermissions(permissions), roleError(err)
}

func (e engine) GetAccountPermissions(ctx context.Context, accountID int64) ([]identityaccess.Permission, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	permissions, err := e.roles.GetAccountPermissions(e.attach(ctx), accountID)
	return publicPermissions(permissions), roleError(err)
}

func (e engine) GetAccountDirectPermissions(ctx context.Context, accountID int64) ([]identityaccess.Permission, error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	permissions, err := e.roles.GetAccountDirectPermissions(e.attach(ctx), accountID)
	return publicPermissions(permissions), roleError(err)
}

func (e engine) CreatePermission(ctx context.Context, name, description, resource, action string) (identityaccess.Permission, error) {
	if e.roles == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	permission, err := e.roles.CreatePermission(e.attach(ctx), name, description, resource, action)
	return identityaccess.Permission(permission), roleError(err)
}

func (e engine) UpdatePermission(ctx context.Context, permission identityaccess.Permission) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.UpdatePermission(e.attach(ctx), domain.ManagedPermission(permission)))
}

func (e engine) DeletePermission(ctx context.Context, id int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.DeletePermission(e.attach(ctx), id))
}

func (e engine) GrantPermissionToAccount(ctx context.Context, accountID, permissionID int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.GrantPermissionToAccount(e.attach(ctx), accountID, permissionID))
}

func (e engine) DenyPermissionToAccount(ctx context.Context, accountID, permissionID int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.DenyPermissionToAccount(e.attach(ctx), accountID, permissionID))
}

func (e engine) RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.RemovePermissionFromAccount(e.attach(ctx), accountID, permissionID))
}

func (e engine) AssignPermissionToRole(ctx context.Context, roleID, permissionID int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.AssignPermissionToRole(e.attach(ctx), roleID, permissionID))
}

func (e engine) ReplaceRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.ReplaceRolePermissions(e.attach(ctx), roleID, permissionIDs))
}

func (e engine) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int64) error {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	return roleError(e.roles.RemovePermissionFromRole(e.attach(ctx), roleID, permissionID))
}

func (e engine) GrantStaffDefaultPermission(ctx context.Context, accountID int64, isTeacher bool, permissionName string) {
	if e.roles == nil {
		return
	}
	e.roles.GrantStaffDefaultPermission(e.attach(ctx), accountID, isTeacher, permissionName)
}

var roleSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrSystemRoleImmutable, identityaccess.ErrSystemRoleImmutable},
	{domain.ErrPermissionNotFound, identityaccess.ErrPermissionNotFound},
	{domain.ErrRoleNotFound, identityaccess.ErrRoleNotFound},
	{domain.ErrRoleCaregiverNeedsProfile, identityaccess.ErrRoleCaregiverNeedsProfile},
	{domain.ErrLehrkraftRoleImmutable, identityaccess.ErrLehrkraftRoleImmutable},
	{domain.ErrRoleLehrkraftCaregiverProfile, identityaccess.ErrRoleLehrkraftCaregiverProfile},
}

// roleError translates an administration error to the public contract: the
// operation envelope keeps its text, a role sentinel gains its public twin,
// and everything else falls through to the lifecycle mapping (the school
// identity and session sentinels an assignment shares).
func roleError(err error) error {
	if err == nil {
		return nil
	}
	var operation *application.OperationError
	if errors.As(err, &operation) && operation == err {
		return &identityaccess.AuthenticationError{Op: operation.Op, Err: roleError(operation.Err)}
	}
	for _, sentinel := range roleSentinels {
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
