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

func (e engine) GetAccountPermissions(ctx context.Context, accountID int64) (value0 []identityaccess.Permission, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) ([]identityaccess.Permission, error) {
		permissions, err := e.roles.GetAccountPermissions(e.attach(ctx), accountID)
		return publicPermissions(permissions), roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

func (e engine) GetAccountDirectPermissions(ctx context.Context, accountID int64) (value0 []identityaccess.Permission, err error) {
	if e.roles == nil {
		return nil, errRoleAdministrationUnavailable
	}
	operation := func(ctx context.Context) ([]identityaccess.Permission, error) {
		permissions, err := e.roles.GetAccountDirectPermissions(e.attach(ctx), accountID)
		return publicPermissions(permissions), roleError(err)
	}
	err = e.roles.runtime.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = operation(txCtx)
		return err
	})
	return
}

var roleSentinels = []struct {
	internal error
	public   error
}{
	{identityaccess.ErrSystemRoleImmutable, identityaccess.ErrSystemRoleImmutable},
	{identityaccess.ErrPermissionNotFound, identityaccess.ErrPermissionNotFound},
	{domain.ErrRoleNameTaken, identityaccess.ErrRoleNameTaken},
	{domain.ErrRoleNotFound, identityaccess.ErrRoleNotFound},
	{identityaccess.ErrRoleCaregiverNeedsProfile, identityaccess.ErrRoleCaregiverNeedsProfile},
	{identityaccess.ErrLehrkraftRoleImmutable, identityaccess.ErrLehrkraftRoleImmutable},
	{identityaccess.ErrRoleLehrkraftCaregiverProfile, identityaccess.ErrRoleLehrkraftCaregiverProfile},
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

// AccountHoldsRole reports whether the account holds the role of that exact
// name. A failed lookup counts as not holding it: the answer only ever
// withholds. It reads through GetAccountRoles, whose transaction boundary the
// register keys on the engine until #4469.
func (e engine) AccountHoldsRole(ctx context.Context, accountID int64, roleName string) bool {
	held, err := e.GetAccountRoles(ctx, accountID)
	if err != nil {
		return false
	}
	for _, role := range held {
		if role.Name == roleName {
			return true
		}
	}
	return false
}
