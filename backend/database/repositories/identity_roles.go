package repositories

import (
	"context"
	"errors"
	"strings"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
)

// Identity & Access owns role and permission management (#3314). Its role
// administration reads and writes the role, permission, assignment and
// grant rows through the retained repositories below until #3226 moves them
// into the module; this adapter serves the module's role storage seam over
// them without changing a statement.

// IdentityRoleRepositories are the retained repositories the role storage
// seam binds.
type IdentityRoleRepositories struct {
	Roles              authModels.RoleRepository
	Permissions        authModels.PermissionRepository
	RolePermissions    authModels.RolePermissionRepository
	AccountRoles       authModels.AccountRoleRepository
	AccountPermissions authModels.AccountPermissionRepository
	Accounts           authModels.AccountRepository
	AccountTenants     authModels.AccountTenantRepository
}

// NewIdentityRoleDirectory binds the Identity & Access role storage seam to
// the retained repositories.
func NewIdentityRoleDirectory(repos IdentityRoleRepositories) (identityCompose.RoleDirectory, error) {
	if repos.Roles == nil || repos.Permissions == nil || repos.RolePermissions == nil || repos.AccountRoles == nil ||
		repos.AccountPermissions == nil || repos.Accounts == nil || repos.AccountTenants == nil {
		return nil, errors.New("identity role directory: every role, permission, assignment, account and membership repository is required")
	}
	return identityRoleDirectory{repos: repos}, nil
}

type identityRoleDirectory struct{ repos IdentityRoleRepositories }

func publicRole(role *authModels.Role) identityaccess.Role {
	return identityaccess.Role{
		ID: role.ID, TenantID: role.TenantID, Name: role.Name, Description: role.Description,
		IsSystem: role.IsSystem, BaseRole: role.BaseRole, CreatedAt: role.CreatedAt, UpdatedAt: role.UpdatedAt,
	}
}

func publicRoles(roles []*authModels.Role) []identityaccess.Role {
	if roles == nil {
		return nil
	}
	result := make([]identityaccess.Role, 0, len(roles))
	for _, role := range roles {
		result = append(result, publicRole(role))
	}
	return result
}

func retainedRole(role identityaccess.Role) *authModels.Role {
	row := &authModels.Role{
		TenantID: role.TenantID, Name: role.Name, Description: role.Description,
		IsSystem: role.IsSystem, BaseRole: role.BaseRole,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = role.ID, role.CreatedAt, role.UpdatedAt
	return row
}

func publicPermission(permission *authModels.Permission) identityaccess.Permission {
	return identityaccess.Permission{
		ID: permission.ID, Name: permission.Name, Description: permission.Description,
		Resource: permission.Resource, Action: permission.Action,
		CreatedAt: permission.CreatedAt, UpdatedAt: permission.UpdatedAt,
	}
}

func publicPermissions(permissions []*authModels.Permission) []identityaccess.Permission {
	if permissions == nil {
		return nil
	}
	result := make([]identityaccess.Permission, 0, len(permissions))
	for _, permission := range permissions {
		result = append(result, publicPermission(permission))
	}
	return result
}

func retainedPermission(permission identityaccess.Permission) *authModels.Permission {
	row := &authModels.Permission{
		Name: permission.Name, Description: permission.Description,
		Resource: permission.Resource, Action: permission.Action,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = permission.ID, permission.CreatedAt, permission.UpdatedAt
	return row
}

// foundRole classifies a role lookup: the repositories translate a missing
// row into the not-found sentinel.
func foundRole(role *authModels.Role, err error) (identityaccess.Role, bool, error) {
	if err != nil {
		if authRepo.IsNotFound(err) {
			return identityaccess.Role{}, false, nil
		}
		return identityaccess.Role{}, false, err
	}
	if role == nil {
		return identityaccess.Role{}, false, nil
	}
	return publicRole(role), true, nil
}

func (d identityRoleDirectory) CreateRole(ctx context.Context, role identityaccess.Role) (identityaccess.Role, error) {
	row := retainedRole(role)
	// tenant_id is set by the repository from the tenant in context.
	if err := d.repos.Roles.Create(ctx, row); err != nil {
		return identityaccess.Role{}, err
	}
	return publicRole(row), nil
}

func (d identityRoleDirectory) FindRole(ctx context.Context, id int64) (identityaccess.Role, bool, error) {
	return foundRole(d.repos.Roles.FindByID(ctx, id))
}

// FindSystemRoleByName resolves the platform system role with that name,
// matched case-insensitively. A school's own role never matches: only a role
// without a tenant that is flagged as a system role qualifies.
func (d identityRoleDirectory) FindSystemRoleByName(ctx context.Context, name string) (identityaccess.Role, bool, error) {
	roles, err := d.repos.Roles.List(ctx, map[string]interface{}{
		"name":      strings.TrimSpace(strings.ToLower(name)),
		"is_system": true,
	})
	if err != nil {
		return identityaccess.Role{}, false, err
	}
	for _, role := range roles {
		if role == nil || role.TenantID != nil || !role.IsSystem || !strings.EqualFold(role.Name, name) {
			continue
		}
		return publicRole(role), true, nil
	}
	return identityaccess.Role{}, false, nil
}

func (d identityRoleDirectory) FindRoleForUpdate(ctx context.Context, id int64) (identityaccess.Role, bool, error) {
	return foundRole(d.repos.Roles.FindByIDForUpdate(ctx, id))
}

func (d identityRoleDirectory) UpdateRole(ctx context.Context, role identityaccess.Role) error {
	return d.repos.Roles.Update(ctx, retainedRole(role))
}

func (d identityRoleDirectory) DeleteRole(ctx context.Context, id int64) error {
	return d.repos.Roles.Delete(ctx, id)
}

func (d identityRoleDirectory) ListRoles(ctx context.Context, filter identityaccess.RoleFilter) ([]identityaccess.Role, error) {
	filters := make(map[string]interface{})
	if filter.Name != "" {
		filters["name"] = filter.Name
	}
	roles, err := d.repos.Roles.List(ctx, filters)
	if err != nil {
		return nil, err
	}
	return publicRoles(roles), nil
}

func (d identityRoleDirectory) ListAccountRoles(ctx context.Context, accountID int64) ([]identityaccess.Role, error) {
	roles, err := d.repos.Roles.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return publicRoles(roles), nil
}

func (d identityRoleDirectory) ListAccountRoleNames(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	return d.repos.Roles.FindRoleNamesByAccountIDs(ctx, accountIDs)
}

// AccountHoldsRole: the repository reports a missing mapping as an error
// whose text carries "no rows".
func (d identityRoleDirectory) AccountHoldsRole(ctx context.Context, accountID, roleID int64) (bool, error) {
	existing, err := d.repos.AccountRoles.FindByAccountAndRole(ctx, accountID, roleID)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return false, err
	}
	return existing != nil, nil
}

func (d identityRoleDirectory) CreateAccountRole(ctx context.Context, accountID, roleID, tenantID int64) error {
	accountRole := &authModels.AccountRole{AccountID: accountID, RoleID: roleID}
	accountRole.SetTenantID(tenantID)
	return d.repos.AccountRoles.Create(ctx, accountRole)
}

func (d identityRoleDirectory) DeleteAccountRole(ctx context.Context, accountID, roleID int64) error {
	return d.repos.AccountRoles.DeleteByAccountAndRole(ctx, accountID, roleID)
}

func (d identityRoleDirectory) DeleteRoleAssignments(ctx context.Context, roleID int64) error {
	return d.repos.AccountRoles.DeleteByRoleID(ctx, roleID)
}

func (d identityRoleDirectory) DeleteRolePermissions(ctx context.Context, roleID int64) error {
	return d.repos.RolePermissions.DeleteByRoleID(ctx, roleID)
}

func (d identityRoleDirectory) CreatePermission(ctx context.Context, permission identityaccess.Permission) (identityaccess.Permission, error) {
	row := retainedPermission(permission)
	if err := d.repos.Permissions.Create(ctx, row); err != nil {
		return identityaccess.Permission{}, err
	}
	return publicPermission(row), nil
}

func (d identityRoleDirectory) FindPermission(ctx context.Context, id int64) (identityaccess.Permission, bool, error) {
	permission, err := d.repos.Permissions.FindByID(ctx, id)
	if err != nil {
		if authRepo.IsNotFound(err) {
			return identityaccess.Permission{}, false, nil
		}
		return identityaccess.Permission{}, false, err
	}
	if permission == nil {
		return identityaccess.Permission{}, false, nil
	}
	return publicPermission(permission), true, nil
}

func (d identityRoleDirectory) FindPermissionByName(ctx context.Context, name string) (identityaccess.Permission, error) {
	permission, err := d.repos.Permissions.FindByName(ctx, name)
	if err != nil {
		return identityaccess.Permission{}, err
	}
	if permission == nil {
		return identityaccess.Permission{}, errors.New("permission not found")
	}
	return publicPermission(permission), nil
}

func (d identityRoleDirectory) UpdatePermission(ctx context.Context, permission identityaccess.Permission) error {
	return d.repos.Permissions.Update(ctx, retainedPermission(permission))
}

func (d identityRoleDirectory) DeletePermission(ctx context.Context, id int64) error {
	return d.repos.Permissions.Delete(ctx, id)
}

func (d identityRoleDirectory) ListPermissions(ctx context.Context, filter identityaccess.PermissionFilter) ([]identityaccess.Permission, error) {
	filters := make(map[string]interface{})
	if filter.Resource != "" {
		filters["resource"] = filter.Resource
	}
	if filter.Action != "" {
		filters["action"] = filter.Action
	}
	permissions, err := d.repos.Permissions.List(ctx, filters)
	if err != nil {
		return nil, err
	}
	return publicPermissions(permissions), nil
}

func (d identityRoleDirectory) ListRolePermissions(ctx context.Context, roleID int64) ([]identityaccess.Permission, error) {
	permissions, err := d.repos.Permissions.FindByRoleID(ctx, roleID)
	if err != nil {
		return nil, err
	}
	return publicPermissions(permissions), nil
}

// ListAccountPermissions combines direct and role-based permissions in one
// query.
func (d identityRoleDirectory) ListAccountPermissions(ctx context.Context, accountID int64) ([]identityaccess.Permission, error) {
	permissions, err := d.repos.Permissions.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return publicPermissions(permissions), nil
}

func (d identityRoleDirectory) ListAccountDirectPermissions(ctx context.Context, accountID int64) ([]identityaccess.Permission, error) {
	permissions, err := d.repos.Permissions.FindDirectByAccountID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return publicPermissions(permissions), nil
}

func (d identityRoleDirectory) AssignRolePermission(ctx context.Context, roleID, permissionID int64) error {
	return d.repos.Permissions.AssignPermissionToRole(ctx, roleID, permissionID)
}

func (d identityRoleDirectory) RemoveRolePermission(ctx context.Context, roleID, permissionID int64) error {
	return d.repos.Permissions.RemovePermissionFromRole(ctx, roleID, permissionID)
}

func (d identityRoleDirectory) DeletePermissionAssignments(ctx context.Context, permissionID int64) error {
	return d.repos.RolePermissions.DeleteByPermissionID(ctx, permissionID)
}

func (d identityRoleDirectory) DeletePermissionGrants(ctx context.Context, permissionID int64) error {
	return d.repos.AccountPermissions.DeleteByPermissionID(ctx, permissionID)
}

func (d identityRoleDirectory) GrantAccountPermission(ctx context.Context, accountID, permissionID int64) error {
	return d.repos.AccountPermissions.GrantPermission(ctx, accountID, permissionID)
}

func (d identityRoleDirectory) DenyAccountPermission(ctx context.Context, accountID, permissionID int64) error {
	return d.repos.AccountPermissions.DenyPermission(ctx, accountID, permissionID)
}

func (d identityRoleDirectory) RemoveAccountPermission(ctx context.Context, accountID, permissionID int64) error {
	return d.repos.AccountPermissions.RemovePermission(ctx, accountID, permissionID)
}

func foundAccount(_ *authModels.Account, err error) (bool, error) {
	if err != nil {
		if authRepo.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d identityRoleDirectory) FindManageableAccount(ctx context.Context, accountID int64) (bool, error) {
	return foundAccount(d.repos.Accounts.FindManageableByID(ctx, accountID))
}

func (d identityRoleDirectory) LockAccount(ctx context.Context, accountID int64) (bool, error) {
	return foundAccount(d.repos.Accounts.FindByIDForUpdate(ctx, accountID))
}

func (d identityRoleDirectory) HasTenantMembership(ctx context.Context, accountID, tenantID int64, share bool) (bool, error) {
	if share {
		return d.repos.AccountTenants.ExistsActiveByAccountAndTenantForShare(ctx, accountID, tenantID)
	}
	return d.repos.AccountTenants.ExistsByAccountAndTenant(ctx, accountID, tenantID)
}

func (d identityRoleDirectory) ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	return d.repos.Accounts.FindEmailsByAccountIDs(ctx, accountIDs)
}

func (d identityRoleDirectory) ListAccountAvatars(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	return d.repos.Accounts.FindAvatarsByAccountIDs(ctx, accountIDs)
}
