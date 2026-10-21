package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Permission management (#3314): the permission catalog, direct account
// grants and the permission selection of custom roles.

const opAssignPermissionToRole = "assign permission to role"

func (r *RoleAdministration) GetPermission(ctx context.Context, id int64) (value0 identityaccess.Permission, err error) {
	if r == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (domain.ManagedPermission, error) {
		permission, found, err := r.store.FindPermission(ctx, id)
		if err != nil {
			return domain.ManagedPermission{}, failed("get permission", err)
		}
		if !found {
			return domain.ManagedPermission{}, failed("get permission", identityaccess.ErrPermissionNotFound)
		}
		return permission, nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err := operation(bindRuntime(r.attach, txCtx))
		value0 = identityaccess.Permission(result)
		return roleError(err)
	})
	return
}

func (r *RoleAdministration) GetPermissionByName(ctx context.Context, name string) (value0 identityaccess.Permission, err error) {
	if r == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (domain.ManagedPermission, error) {
		permission, err := r.store.FindPermissionByName(ctx, name)
		if err != nil {
			return domain.ManagedPermission{}, failed("get permission by name", err)
		}
		return permission, nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err := operation(bindRuntime(r.attach, txCtx))
		value0 = identityaccess.Permission(result)
		return roleError(err)
	})
	return
}

func (r *RoleAdministration) deletePermission(ctx context.Context, id int64) error {
	if err := r.store.DeletePermissionGrants(ctx, id); err != nil {
		return failed("delete account permissions", err)
	}
	if err := r.store.DeletePermissionAssignments(ctx, id); err != nil {
		return failed("delete role permissions", err)
	}
	if err := r.store.DeletePermission(ctx, id); err != nil {
		return failed("delete permission", err)
	}
	return nil
}

// AssignPermissionToRole adds a permission to a custom role.
func (r *RoleAdministration) AssignPermissionToRole(ctx context.Context, roleID, permissionID int64) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	defer func() { err = roleError(err) }()
	ctx = bindRuntime(r.attach, ctx)
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		if err := r.lockMutableRole(txCtx, roleID, opAssignPermissionToRole); err != nil {
			return err
		}
		if _, found, err := r.store.FindPermission(txCtx, permissionID); err != nil || !found {
			return failed(opAssignPermissionToRole, identityaccess.ErrPermissionNotFound)
		}
		if err := r.store.AssignRolePermission(txCtx, roleID, permissionID); err != nil {
			return failed(opAssignPermissionToRole, err)
		}
		return nil
	})
}

// lockMutableRole serializes every role-permission mutation on the role row.
// It preserves infrastructure failures so callers do not turn them into a
// misleading not-found response.
func (r *RoleAdministration) lockMutableRole(ctx context.Context, roleID int64, op string) error {
	role, found, err := r.store.FindRoleForUpdate(ctx, roleID)
	if err != nil {
		return failed(op, err)
	}
	if !found {
		return failed(op, domain.ErrRoleNotFound)
	}
	if role.IsSystem {
		return failed(op, identityaccess.ErrSystemRoleImmutable)
	}
	return nil
}

// ReplaceRolePermissions applies a complete permission selection to a custom
// role in one transaction. All requested permissions are validated before the
// current selection is changed, so an invalid request leaves it untouched.
func (r *RoleAdministration) ReplaceRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	defer func() { err = roleError(err) }()
	ctx = bindRuntime(r.attach, ctx)
	const op = "replace role permissions"
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		// The role row serializes full replacements and the one-at-a-time
		// mutations. Lock it before loading mappings so each operation observes
		// the preceding committed selection.
		if err := r.lockMutableRole(txCtx, roleID, op); err != nil {
			return err
		}

		desired, err := r.desiredRolePermissions(txCtx, permissionIDs)
		if err != nil {
			return err
		}
		current, err := r.removeUnselectedPermissions(txCtx, roleID, desired)
		if err != nil {
			return err
		}
		for permissionID := range desired {
			if _, assigned := current[permissionID]; assigned {
				continue
			}
			if err := r.store.AssignRolePermission(txCtx, roleID, permissionID); err != nil {
				return failed(op, err)
			}
		}
		return nil
	})
}

// RemovePermissionFromRole removes a permission from a custom role.
func (r *RoleAdministration) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int64) (err error) {
	if r == nil {
		return errRoleAdministrationUnavailable
	}
	defer func() { err = roleError(err) }()
	ctx = bindRuntime(r.attach, ctx)
	const op = "remove permission from role"
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		if err := r.lockMutableRole(txCtx, roleID, op); err != nil {
			return err
		}
		if err := r.store.RemoveRolePermission(txCtx, roleID, permissionID); err != nil {
			return failed(op, err)
		}
		return nil
	})
}

func (r *RoleAdministration) GetRolePermissions(ctx context.Context, roleID int64) (value0 []identityaccess.Permission, err error) {
	if r == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) ([]domain.ManagedPermission, error) {
		permissions, err := r.store.ListRolePermissions(ctx, roleID)
		if err != nil {
			return nil, failed("get role permissions", err)
		}
		return permissions, nil
	}
	err = r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		result, err := operation(bindRuntime(r.attach, txCtx))
		value0 = publicPermissions(result)
		return roleError(err)
	})
	return
}

func (r *RoleAdministration) desiredRolePermissions(txCtx context.Context, permissionIDs []int64) (map[int64]struct{}, error) {
	desired := make(map[int64]struct{}, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		if permissionID <= 0 {
			return nil, failed("replace role permissions", identityaccess.ErrPermissionNotFound)
		}
		_, found, err := r.store.FindPermission(txCtx, permissionID)
		if err != nil {
			return nil, failed("replace role permissions", err)
		}
		if !found {
			return nil, failed("replace role permissions", identityaccess.ErrPermissionNotFound)
		}
		desired[permissionID] = struct{}{}
	}

	return desired, nil
}

func (r *RoleAdministration) removeUnselectedPermissions(txCtx context.Context, roleID int64, desired map[int64]struct{}) (map[int64]struct{}, error) {
	currentPermissions, err := r.store.ListRolePermissions(txCtx, roleID)
	if err != nil {
		return nil, failed("replace role permissions", err)
	}
	current := make(map[int64]struct{}, len(currentPermissions))
	for _, permission := range currentPermissions {
		current[permission.ID] = struct{}{}
		if _, keep := desired[permission.ID]; keep {
			continue
		}
		if err := r.store.RemoveRolePermission(txCtx, roleID, permission.ID); err != nil {
			return nil, failed("replace role permissions", err)
		}
	}

	return current, nil
}
