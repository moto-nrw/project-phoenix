package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// errExternalCaregiverRoleNeedsTransaction refuses Ensure outside a unit of
// work: the role and its permissions are written by separate commands that
// must commit or roll back together with the caller's write.
var errExternalCaregiverRoleNeedsTransaction = errors.New("ensure external caregiver role: the caller's transaction is required")

// EnsureExternalCaregiverRole gives the school its „Extern“ role (#4573):
// a custom role of base role user with the default permissions. A role of
// that name the school already has, whatever its tier or permissions, is
// returned unchanged. It composes the role administration's own commands,
// which join the caller's transaction, so it requires one; a context scoped
// to another school is refused before anything is written.
func (r *RoleAdministration) EnsureExternalCaregiverRole(ctx context.Context, tenantID int64) (identityaccess.Role, error) {
	if r == nil {
		return identityaccess.Role{}, errRoleAdministrationUnavailable
	}
	if tenantID <= 0 {
		return identityaccess.Role{}, identityaccess.ErrTenantRequired
	}
	if current := r.runtime.TenantID(ctx); current != 0 && current != tenantID {
		return identityaccess.Role{}, fmt.Errorf("ensure external caregiver role: %w: the context belongs to another school", identityaccess.ErrTenantRequired)
	}
	if !r.runtime.HasTransaction(ctx) {
		return identityaccess.Role{}, errExternalCaregiverRoleNeedsTransaction
	}
	ctx = r.runtime.WithTenantID(ctx, tenantID)
	named, err := r.externalCaregiverRoles(ctx, tenantID)
	if err != nil {
		return identityaccess.Role{}, roleError(err)
	}
	if len(named) > 0 {
		return identityaccess.Role(preferredExternalCaregiverRole(named)), nil
	}
	return r.createExternalCaregiverRole(ctx, tenantID)
}

// FindExternalCaregiverRole resolves the „Extern“ role of the school in ctx:
// a custom role of that school, of base role user, named „Extern“
// case-insensitively and trimmed.
func (r *RoleAdministration) FindExternalCaregiverRole(ctx context.Context) (identityaccess.Role, bool, error) {
	if r == nil {
		return identityaccess.Role{}, false, errRoleAdministrationUnavailable
	}
	tenantID := r.runtime.TenantID(ctx)
	if tenantID <= 0 {
		// Without a school the store would read every school's roles.
		return identityaccess.Role{}, false, identityaccess.ErrTenantRequired
	}
	named, err := r.externalCaregiverRoles(ctx, tenantID)
	if err != nil {
		return identityaccess.Role{}, false, roleError(err)
	}
	for _, candidate := range named {
		if usableExternalCaregiverRole(candidate) {
			return identityaccess.Role(candidate), true, nil
		}
	}
	return identityaccess.Role{}, false, nil
}

// externalCaregiverRoles lists the school's own roles named „Extern“, oldest
// first. The school has few roles, so the name match runs here with the
// trimming the store's name filter does not apply.
func (r *RoleAdministration) externalCaregiverRoles(ctx context.Context, tenantID int64) ([]domain.ManagedRole, error) {
	roles, err := r.listRoles(ctx, domain.RoleFilter{})
	if err != nil {
		return nil, err
	}
	var named []domain.ManagedRole
	for _, role := range roles {
		if role.TenantID != nil && *role.TenantID == tenantID && identityaccess.IsExternalCaregiverRoleName(role.Name) {
			named = append(named, role)
		}
	}
	sort.Slice(named, func(i, j int) bool { return named[i].ID < named[j].ID })
	return named, nil
}

// preferredExternalCaregiverRole returns the role Find resolves when there
// is one, otherwise the oldest role of that name.
func preferredExternalCaregiverRole(named []domain.ManagedRole) domain.ManagedRole {
	for _, role := range named {
		if usableExternalCaregiverRole(role) {
			return role
		}
	}
	return named[0]
}

func usableExternalCaregiverRole(role domain.ManagedRole) bool {
	return !role.IsSystem && role.BaseRole != nil && *role.BaseRole == identityaccess.BaseRoleUser
}

// createExternalCaregiverRole writes the role and its permission selection
// through the administration's commands on the caller's transaction.
func (r *RoleAdministration) createExternalCaregiverRole(ctx context.Context, tenantID int64) (identityaccess.Role, error) {
	baseRole := identityaccess.BaseRoleUser
	role, err := r.CreateRole(ctx, identityaccess.ExternalCaregiverRoleName, identityaccess.ExternalCaregiverRoleDescription, &baseRole)
	if err != nil {
		return identityaccess.Role{}, err
	}
	permissionIDs := make([]int64, 0, len(identityaccess.ExternalCaregiverDefaultPermissions))
	for _, name := range identityaccess.ExternalCaregiverDefaultPermissions {
		permission, err := r.GetPermissionByName(ctx, name)
		if err != nil {
			return identityaccess.Role{}, fmt.Errorf("ensure external caregiver role: permission %s: %w", name, err)
		}
		permissionIDs = append(permissionIDs, permission.ID)
	}
	if err := r.ReplaceRolePermissions(ctx, role.ID, permissionIDs); err != nil {
		return identityaccess.Role{}, err
	}
	r.logger.InfoContext(ctx, "external caregiver role provisioned",
		"tenant_id", tenantID,
		"role_id", role.ID,
	)
	return role, nil
}
