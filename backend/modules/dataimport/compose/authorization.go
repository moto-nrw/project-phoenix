package compose

import (
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// NewAuthorization binds import decisions to the canonical Security Runtime
// checks, including wildcards and the fail-closed role-grant rules.
func NewAuthorization() dataimport.Authorization { return authorization{} }

type authorization struct{}

func (authorization) HasPermission(required string, permissions []string) bool {
	return authorize.HasPermission(required, permissions)
}

func (authorization) CanGrantRole(role dataimport.RoleGrantSource, permissions []string) bool {
	return authorize.CanGrantRole(role, permissions)
}
