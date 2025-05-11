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
				render.Render(w, r, ErrForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(hfn)
	}
}

// RequiresAnyPermission middleware restricts access to accounts having any of the specified permissions.
func RequiresAnyPermission(permissions ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			// Get permissions from context
			userPermissions := jwt.PermissionsFromCtx(r.Context())

			// Check for any required permission
			hasAny := false
			for _, perm := range permissions {
				if hasPermission(perm, userPermissions) {
					hasAny = true
					break
				}
			}

			if !hasAny {
				render.Render(w, r, ErrForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(hfn)
	}
}

// RequiresAllPermissions middleware restricts access to accounts having all of the specified permissions.
func RequiresAllPermissions(permissions ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			// Get permissions from context
			userPermissions := jwt.PermissionsFromCtx(r.Context())

			// Check for all required permissions
			for _, perm := range permissions {
				if !hasPermission(perm, userPermissions) {
					render.Render(w, r, ErrForbidden)
					return
				}
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
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
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
		// Split permission into resource and action
		parts := strings.Split(perm, ":")
		if len(parts) != 2 {
			continue // Skip invalid permissions
		}

		resource := parts[0]
		action := parts[1]

		// Check resource matches
		resourceMatch := resource == requiredResource ||
			resource == "*" ||
			(strings.HasSuffix(resource, "*") &&
				strings.HasPrefix(requiredResource, strings.TrimSuffix(resource, "*")))

		// Check action matches
		actionMatch := action == requiredAction ||
			action == "*" ||
			(strings.HasSuffix(action, "*") &&
				strings.HasPrefix(requiredAction, strings.TrimSuffix(action, "*")))

		// Both resource and action must match
		if resourceMatch && actionMatch {
			return true
		}
	}

	return false
}
