package authorize

import (
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// RequiresPermission middleware restricts access to accounts having the specific permission.
func RequiresPermission(permission string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			// Get permissions from context
			permissions := jwt.PermissionsFromCtx(r.Context())

			// Check for required permission
			if !hasPermission(permission, permissions) {
				_ = render.Render(w, r, ErrForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(hfn)
	}
}

// hasPermission checks if the specified permission is included in the permissions list.
// Supports wildcard matching for resource and action components.
func hasPermission(required string, permissions []string) bool {
	// Special case: empty required permission always matches
	if required == "" {
		return true
	}

	// Check for admin wildcard permission first - grants all permissions
	if hasAdminWildcard(permissions) {
		return true
	}

	// Split required permission into resource and action
	requiredParts := strings.Split(required, ":")
	if len(requiredParts) != 2 {
		return false // Invalid format
	}

	requiredResource := requiredParts[0]
	requiredAction := requiredParts[1]

	// Check each permission
	for _, perm := range permissions {
		if permissionMatches(perm, requiredResource, requiredAction) {
			return true
		}
	}

	return false
}

// hasAdminWildcard checks if permissions list contains admin wildcard
func hasAdminWildcard(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}

// permissionMatches checks if a permission matches the required resource and action
func permissionMatches(perm, requiredResource, requiredAction string) bool {
	// Split permission into resource and action
	parts := strings.Split(perm, ":")
	if len(parts) != 2 {
		return false // Invalid permission format
	}

	resource := parts[0]
	action := parts[1]

	// Both resource and action must match
	return matchesPattern(resource, requiredResource) && matchesPattern(action, requiredAction)
}

// matchesPattern checks if a pattern matches a required value with wildcard support
func matchesPattern(pattern, required string) bool {
	if pattern == required || pattern == "*" {
		return true
	}

	// Check prefix wildcard (e.g., "users*" matches "users:read")
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(required, prefix)
	}

	return false
}
