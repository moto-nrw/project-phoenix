package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

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

func (e engine) GetRole(ctx context.Context, id int64) (value0 identityaccess.Role, err error) {
	if e.roles == nil {
		return identityaccess.Role{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.Role, error) {
		role, err := e.roles.GetRole(e.attach(ctx), id)
		return identityaccess.Role(role), roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) ListRoles(ctx context.Context, filter identityaccess.RoleFilter) ([]identityaccess.Role, error) {
	if e.accountRoleQueries == nil {
		return nil, errRoleAdministrationUnavailable
	}
	roles, err := e.accountRoleQueries.ListRoles(e.attach(ctx), domain.RoleFilter(filter))
	return publicRoles(roles), roleError(err)
}

func (e engine) ResolveAssignableSchoolRole(ctx context.Context, roleID, tenantID int64) (value0 *identityaccess.AssignableSchoolRole, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (*identityaccess.AssignableSchoolRole, error) {
		role, permissions, err := e.roles.ResolveAssignableSchoolRole(e.attach(ctx), roleID, tenantID)
		if err != nil {
			return nil, roleError(err)
		}
		return &identityaccess.AssignableSchoolRole{Role: identityaccess.Role(role), Permissions: permissions}, nil
	}
	err = globalIdentityRead(ctx, e.roles.runtime, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) GetAccountRoles(ctx context.Context, accountID int64) (value0 []identityaccess.Role, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) ([]identityaccess.Role, error) {
		roles, err := e.roles.GetAccountRoles(e.attach(ctx), accountID)
		return publicRoles(roles), roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) AccountsHoldingRole(ctx context.Context, accountIDs []int64, roleName string) (value0 map[int64]bool, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	if len(accountIDs) == 0 {
		return map[int64]bool{}, nil
	}
	if roleName == "" {
		return map[int64]bool{}, nil
	}
	operation := func(ctx context.Context) (map[int64]bool, error) {
		matches, err := e.roles.AccountsHoldingRole(e.attach(ctx), accountIDs, roleName)
		return matches, roleError(err)
	}
	err = globalIdentityRead(ctx, e.roles.runtime, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (value0 bool, err error) {
	if e.roles == nil {
		return false, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (bool, error) {
		holds, err := e.roles.AccountHoldsLehrkraftRole(e.attach(ctx), accountID)
		return holds, roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) GetAccountRoleNames(ctx context.Context, accountIDs []int64) (value0 map[int64]string, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (map[int64]string, error) {
		names, err := e.roles.GetAccountRoleNames(e.attach(ctx), accountIDs)
		return names, roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) GetAccountEmails(ctx context.Context, accountIDs []int64) (value0 map[int64]string, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (map[int64]string, error) {
		emails, err := e.roles.GetAccountEmails(e.attach(ctx), accountIDs)
		return emails, roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) GetAccountAvatars(ctx context.Context, accountIDs []int64) (value0 map[int64]string, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (map[int64]string, error) {
		avatars, err := e.roles.GetAccountAvatars(e.attach(ctx), accountIDs)
		return avatars, roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) CreateRole(ctx context.Context, name, description string, baseRole *string) (value0 identityaccess.Role, err error) {
	if e.roles == nil {
		return identityaccess.Role{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.Role, error) {
		role, err := e.roles.CreateRole(e.attach(ctx), name, description, baseRole)
		return identityaccess.Role(role), roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) UpdateRole(ctx context.Context, role identityaccess.Role) (err error) {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) error {
		return roleError(e.roles.UpdateRole(e.attach(ctx), domain.ManagedRole(role)))
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
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

func (e engine) GetPermission(ctx context.Context, id int64) (value0 identityaccess.Permission, err error) {
	if e.roles == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.Permission, error) {
		permission, err := e.roles.GetPermission(e.attach(ctx), id)
		return identityaccess.Permission(permission), roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) GetPermissionByName(ctx context.Context, name string) (value0 identityaccess.Permission, err error) {
	if e.roles == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.Permission, error) {
		permission, err := e.roles.GetPermissionByName(e.attach(ctx), name)
		return identityaccess.Permission(permission), roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) ListPermissions(ctx context.Context, filter identityaccess.PermissionFilter) ([]identityaccess.Permission, error) {
	if e.accountRoleQueries == nil {
		return nil, errRoleAdministrationUnavailable
	}
	permissions, err := e.accountRoleQueries.ListPermissions(e.attach(ctx), domain.PermissionFilter(filter))
	return publicPermissions(permissions), roleError(err)
}

func (e engine) GetRolePermissions(ctx context.Context, roleID int64) (value0 []identityaccess.Permission, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) ([]identityaccess.Permission, error) {
		permissions, err := e.roles.GetRolePermissions(e.attach(ctx), roleID)
		return publicPermissions(permissions), roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) CreatePermission(ctx context.Context, name, description, resource, action string) (value0 identityaccess.Permission, err error) {
	if e.roles == nil {
		return identityaccess.Permission{}, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) (identityaccess.Permission, error) {
		permission, err := e.roles.CreatePermission(e.attach(ctx), name, description, resource, action)
		return identityaccess.Permission(permission), roleError(err)
	}
	err = globalIdentityRead(ctx, e.roles.runtime, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) UpdatePermission(ctx context.Context, permission identityaccess.Permission) (err error) {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) error {
		return roleError(e.roles.UpdatePermission(e.attach(ctx), domain.ManagedPermission(permission)))
	}
	err = globalIdentityRead(ctx, e.roles.runtime, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
}

func (e engine) DeletePermission(ctx context.Context, id int64) (err error) {
	if e.roles == nil {
		return errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) error {
		return roleError(e.roles.DeletePermission(e.attach(ctx), id))
	}
	err = globalIdentityRead(ctx, e.roles.runtime, func(txCtx context.Context) error {
		err = operation(txCtx)
		return err
	})
	return
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

var roleSentinels = []struct {
	internal error
	public   error
}{
	{domain.ErrSystemRoleImmutable, identityaccess.ErrSystemRoleImmutable},
	{domain.ErrPermissionNotFound, identityaccess.ErrPermissionNotFound},
	{domain.ErrRoleNameTaken, identityaccess.ErrRoleNameTaken},
	{domain.ErrRoleNotFound, identityaccess.ErrRoleNotFound},
	{domain.ErrRoleCaregiverNeedsProfile, identityaccess.ErrRoleCaregiverNeedsProfile},
	{domain.ErrLehrkraftRoleImmutable, identityaccess.ErrLehrkraftRoleImmutable},
	{domain.ErrRoleLehrkraftCaregiverProfile, identityaccess.ErrRoleLehrkraftCaregiverProfile},
}

// roleError translates an administration error to the public contract: the
// operation envelope keeps its text, a role sentinel gains its public twin,
// and everything else falls through to the lifecycle mapping (the school
// identity and session sentinels an assignment shares).
func roleError(err error) error { return recordMissing(translateRoleError(err)) }

func translateRoleError(err error) error {
	if err == nil {
		return nil
	}
	var operation *OperationError
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
