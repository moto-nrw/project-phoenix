package application

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Permission management (#3314): the permission catalog, direct account
// grants and the permission selection of custom roles.

const opAssignPermissionToRole = "assign permission to role"

func (r *RoleAdministration) CreatePermission(ctx context.Context, name, description, resource, action string) (domain.ManagedPermission, error) {
	permission, err := r.store.CreatePermission(ctx, domain.ManagedPermission{
		Name: name, Description: description, Resource: resource, Action: action,
	})
	if err != nil {
		return domain.ManagedPermission{}, failed("create permission", err)
	}
	return permission, nil
}

func (r *RoleAdministration) GetPermission(ctx context.Context, id int64) (domain.ManagedPermission, error) {
	permission, found, err := r.store.FindPermission(ctx, id)
	if err != nil {
		return domain.ManagedPermission{}, failed("get permission", err)
	}
	if !found {
		return domain.ManagedPermission{}, failed("get permission", domain.ErrPermissionNotFound)
	}
	return permission, nil
}

func (r *RoleAdministration) GetPermissionByName(ctx context.Context, name string) (domain.ManagedPermission, error) {
	permission, err := r.store.FindPermissionByName(ctx, name)
	if err != nil {
		return domain.ManagedPermission{}, failed("get permission by name", err)
	}
	return permission, nil
}

func (r *RoleAdministration) UpdatePermission(ctx context.Context, permission domain.ManagedPermission) error {
	if err := r.store.UpdatePermission(ctx, permission); err != nil {
		return failed("update permission", err)
	}
	return nil
}

// DeletePermission removes the account grants and role mappings of the
// permission before the permission itself.
func (r *RoleAdministration) DeletePermission(ctx context.Context, id int64) error {
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

func (r *RoleAdministration) ListPermissions(ctx context.Context, filter domain.PermissionFilter) ([]domain.ManagedPermission, error) {
	permissions, err := r.store.ListPermissions(ctx, filter)
	if err != nil {
		return nil, failed("list permissions", err)
	}
	return permissions, nil
}

// GrantPermissionToAccount grants a permission directly to an account.
func (r *RoleAdministration) GrantPermissionToAccount(ctx context.Context, accountID, permissionID int64) error {
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		if err := r.lockManageableAccount(txCtx, accountID, "grant permission"); err != nil {
			return err
		}
		if _, found, err := r.store.FindPermission(txCtx, permissionID); err != nil || !found {
			return failed("grant permission", domain.ErrPermissionNotFound)
		}
		if err := r.store.GrantAccountPermission(txCtx, accountID, permissionID); err != nil {
			return failed("grant permission to account", err)
		}
		return nil
	})
}

// DenyPermissionToAccount explicitly denies a permission to an account.
func (r *RoleAdministration) DenyPermissionToAccount(ctx context.Context, accountID, permissionID int64) error {
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		if err := r.lockManageableAccount(txCtx, accountID, "deny permission"); err != nil {
			return err
		}
		if _, found, err := r.store.FindPermission(txCtx, permissionID); err != nil || !found {
			return failed("deny permission", domain.ErrPermissionNotFound)
		}
		if err := r.store.DenyAccountPermission(txCtx, accountID, permissionID); err != nil {
			return failed("deny permission to account", err)
		}
		return nil
	})
}

// RemovePermissionFromAccount removes a direct grant or denial.
func (r *RoleAdministration) RemovePermissionFromAccount(ctx context.Context, accountID, permissionID int64) error {
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		if err := r.lockManageableAccount(txCtx, accountID, "remove permission"); err != nil {
			return err
		}
		if err := r.store.RemoveAccountPermission(txCtx, accountID, permissionID); err != nil {
			return failed("remove permission from account", err)
		}
		return nil
	})
}

// GetAccountPermissions returns the direct and the role-based permissions of
// an account in one query.
func (r *RoleAdministration) GetAccountPermissions(ctx context.Context, accountID int64) ([]domain.ManagedPermission, error) {
	if _, err := r.accounts.FindManageableAccount(ctx, accountID); err != nil {
		return nil, failed("get account permissions", domain.ErrAccountNotFound)
	}
	if err := r.ensureOrganizationRBACMembership(ctx, accountID, "get account permissions", false); err != nil {
		return nil, err
	}
	permissions, err := r.store.ListAccountPermissions(ctx, accountID)
	if err != nil {
		return nil, failed("get account permissions", err)
	}
	return permissions, nil
}

// GetAccountDirectPermissions returns only the direct permissions of an
// account, not the role-based ones.
func (r *RoleAdministration) GetAccountDirectPermissions(ctx context.Context, accountID int64) ([]domain.ManagedPermission, error) {
	if _, err := r.accounts.FindManageableAccount(ctx, accountID); err != nil {
		return nil, failed("get account direct permissions", domain.ErrAccountNotFound)
	}
	if err := r.ensureOrganizationRBACMembership(ctx, accountID, "get account direct permissions", false); err != nil {
		return nil, err
	}
	permissions, err := r.store.ListAccountDirectPermissions(ctx, accountID)
	if err != nil {
		return nil, failed("get account direct permissions", err)
	}
	return permissions, nil
}

// AssignPermissionToRole adds a permission to a custom role.
func (r *RoleAdministration) AssignPermissionToRole(ctx context.Context, roleID, permissionID int64) error {
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		if err := r.lockMutableRole(txCtx, roleID, opAssignPermissionToRole); err != nil {
			return err
		}
		if _, found, err := r.store.FindPermission(txCtx, permissionID); err != nil || !found {
			return failed(opAssignPermissionToRole, domain.ErrPermissionNotFound)
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
		return failed(op, domain.ErrSystemRoleImmutable)
	}
	return nil
}

// ReplaceRolePermissions applies a complete permission selection to a custom
// role in one transaction. All requested permissions are validated before the
// current selection is changed, so an invalid request leaves it untouched.
func (r *RoleAdministration) ReplaceRolePermissions(ctx context.Context, roleID int64, permissionIDs []int64) error {
	const op = "replace role permissions"
	return r.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		// The role row serializes full replacements and the one-at-a-time
		// mutations. Lock it before loading mappings so each operation observes
		// the preceding committed selection.
		if err := r.lockMutableRole(txCtx, roleID, op); err != nil {
			return err
		}

		desired := make(map[int64]struct{}, len(permissionIDs))
		for _, permissionID := range permissionIDs {
			if permissionID <= 0 {
				return failed(op, domain.ErrPermissionNotFound)
			}
			_, found, err := r.store.FindPermission(txCtx, permissionID)
			if err != nil {
				return failed(op, err)
			}
			if !found {
				return failed(op, domain.ErrPermissionNotFound)
			}
			desired[permissionID] = struct{}{}
		}

		currentPermissions, err := r.store.ListRolePermissions(txCtx, roleID)
		if err != nil {
			return failed(op, err)
		}
		current := make(map[int64]struct{}, len(currentPermissions))
		for _, permission := range currentPermissions {
			current[permission.ID] = struct{}{}
			if _, keep := desired[permission.ID]; keep {
				continue
			}
			if err := r.store.RemoveRolePermission(txCtx, roleID, permission.ID); err != nil {
				return failed(op, err)
			}
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
func (r *RoleAdministration) RemovePermissionFromRole(ctx context.Context, roleID, permissionID int64) error {
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

func (r *RoleAdministration) GetRolePermissions(ctx context.Context, roleID int64) ([]domain.ManagedPermission, error) {
	permissions, err := r.store.ListRolePermissions(ctx, roleID)
	if err != nil {
		return nil, failed("get role permissions", err)
	}
	return permissions, nil
}

// GrantStaffDefaultPermission grants the default staff permission to the
// account of a newly created staff member.
//
// The Lehrkraft role (#1772) is deliberately class_day:read only: groups:read
// would open the tenant-wide group list and every group's student names, far
// beyond the classes assigned to that Lehrkraft. The decision is made on the
// account's actual roles, not on the teacher flag, and fails closed: an
// unreadable role set is no licence to widen access.
//
// permissionName is the permission to grant; the caller passes the People
// Directory default so this module needs no staff permission contract.
func (r *RoleAdministration) GrantStaffDefaultPermission(ctx context.Context, accountID int64, isTeacher bool, permissionName string) {
	role := "staff"
	if isTeacher {
		role = "teacher"
	}
	roles, err := r.GetAccountRoles(ctx, accountID)
	if err != nil {
		r.logger.Error("failed to resolve account roles, skipping default permission grant",
			slog.String("role", role),
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
		return
	}
	for _, accountRole := range roles {
		if r.policy.IsLehrkraftSystemRole(accountRole.Facts()) {
			r.logger.Info("skipping groups:read for lehrkraft account", slog.Int64("account_id", accountID))
			return
		}
	}
	permission, err := r.GetPermissionByName(ctx, permissionName)
	if err != nil {
		return
	}
	if err := r.GrantPermissionToAccount(ctx, accountID, permission.ID); err != nil {
		r.logger.Error("failed to grant groups:read permission",
			slog.String("role", role),
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()))
	}
}
