package api

import (
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// NewImportAuthorization binds Data Import's decision port at the root.
func NewImportAuthorization() dataimport.Authorization { return importAuthorization{} }

type importAuthorization struct{}

func (importAuthorization) HasPermission(required string, permissions []string) bool {
	return securityruntime.HasPermission(required, permissions)
}

func (importAuthorization) CanGrantRole(role dataimport.RoleGrantSource, permissions []string) bool {
	if role == nil {
		return false
	}
	present, name, base, system, bound, granted := role.AuthorizationGrantData()
	if !present {
		return false
	}
	return securityruntime.CanGrantRole(securityruntime.GrantedRole{
		Name: name, BaseRole: base, IsSystem: system, TenantBound: bound, Permissions: granted,
	}, permissions)
}
