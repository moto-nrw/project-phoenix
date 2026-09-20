package repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authModels "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	authRepo "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authpostgres"
)

// Identity & Access owns role and permission management (#3314). Permission
// persistence is module-owned. This adapter still binds role rows, account
// assignments and account visibility until the remainder of #3226 cuts over.

// IdentityRoleRepositories are the retained repositories the role storage
// seam binds.
type IdentityRoleRepositories struct {
	Roles          authModels.RoleRepository
	AccountRoles   authModels.AccountRoleRepository
	Accounts       authModels.AccountRepository
	AccountTenants authModels.AccountTenantRepository
}

// NewIdentityRoleDirectory binds the Identity & Access role storage seam to
// the retained repositories.
func NewIdentityRoleDirectory(repos IdentityRoleRepositories) (identityCompose.RoleDirectory, error) {
	if repos.Roles == nil || repos.AccountRoles == nil || repos.Accounts == nil || repos.AccountTenants == nil {
		return nil, errors.New("identity role directory: every role, assignment, account and membership repository is required")
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
