package securityruntime

import "github.com/moto-nrw/project-phoenix/auth/authorize"

// GrantedRole are the stored facts of the role being handed out, plus its
// effective permissions. The caller states the facts; Security Runtime owns
// the rule that decides whether they may be granted (#2722, #3364).
type GrantedRole struct {
	Name        string
	BaseRole    *string
	IsSystem    bool
	TenantBound bool
	Permissions []string
}

// AuthorizationGrantData exposes the stored facts the role-grant policy reads.
func (r GrantedRole) AuthorizationGrantData() (bool, string, *string, bool, bool, []string) {
	return true, r.Name, r.BaseRole, r.IsSystem, r.TenantBound, r.Permissions
}

// CanGrantRole answers whether an account holding actorPermissions may hand
// the role out to someone else.
func CanGrantRole(role GrantedRole, actorPermissions []string) bool {
	return authorize.CanGrantRole(role, actorPermissions)
}
