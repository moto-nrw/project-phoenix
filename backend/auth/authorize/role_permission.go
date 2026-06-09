package authorize

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/models/auth"
)

// adminResource and adminNamePrefix classify a permission as admin-level. The
// classification rule lived on the Permission model (IsAdminPermission); it is
// an authorization concern, not a data fact on the entity (issue #586, Rule 12).
const (
	adminResource   = "admin"
	adminNamePrefix = "admin:"
)

// PermissionIsAdmin reports whether the permission is admin-level, either by its
// resource being "admin" or its name carrying the "admin:" prefix. Moved off the
// Permission model per Rule 12.
func PermissionIsAdmin(p *auth.Permission) bool {
	if p == nil {
		return false
	}
	return p.Resource == adminResource || strings.HasPrefix(p.Name, adminNamePrefix)
}

// RoleHasPermission reports whether the role's loaded permission relations
// include a permission with the given name (case-insensitive). The membership
// decision lives here rather than on the Role model: it is an RBAC concern, not
// a data fact (issue #586, Rule 12).
func RoleHasPermission(r *auth.Role, permission string) bool {
	if r == nil || r.Permissions == nil {
		return false
	}
	for _, p := range r.Permissions {
		if p != nil && strings.EqualFold(p.Name, permission) {
			return true
		}
	}
	return false
}

// RoleAddPermission adds a permission to the role's loaded relation set if it is
// not already present (de-duplicated by ID). The grant mutation is an RBAC
// concern that belongs in the authorize layer, not on the model.
func RoleAddPermission(r *auth.Role, permission *auth.Permission) {
	if r == nil || permission == nil {
		return
	}
	if r.Permissions == nil {
		r.Permissions = make([]*auth.Permission, 0)
	}
	for _, p := range r.Permissions {
		if p != nil && p.ID == permission.ID {
			return // already present
		}
	}
	r.Permissions = append(r.Permissions, permission)
}

// RoleRemovePermission removes the permission with the given ID from the role's
// loaded relation set. It returns true when a permission was removed. The revoke
// mutation is an RBAC concern that belongs in the authorize layer.
func RoleRemovePermission(r *auth.Role, permissionID int64) bool {
	if r == nil || r.Permissions == nil {
		return false
	}
	for i, p := range r.Permissions {
		if p != nil && p.ID == permissionID {
			r.Permissions = append(r.Permissions[:i], r.Permissions[i+1:]...)
			return true
		}
	}
	return false
}
