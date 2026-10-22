package application

import (
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

var roleSentinels = []struct {
	internal error
	public   error
}{
	{identityaccess.ErrSystemRoleImmutable, identityaccess.ErrSystemRoleImmutable},
	{identityaccess.ErrPermissionNotFound, identityaccess.ErrPermissionNotFound},
	{identityaccess.ErrPlatformPermissionNotAllowed, identityaccess.ErrPlatformPermissionNotAllowed},
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
