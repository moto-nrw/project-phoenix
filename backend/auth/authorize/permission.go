package authorize

import (
	"context"
	"log/slog"
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
			if !HasPermission(permission, permissions) {
				if err := render.Render(w, r, ErrForbidden); err != nil {
					slog.Error("failed to render forbidden response", slog.String("error", err.Error()))
				}
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
				if HasPermission(perm, userPermissions) {
					hasAny = true
					break
				}
			}

			if !hasAny {
				if err := render.Render(w, r, ErrForbidden); err != nil {
					slog.Error("failed to render forbidden response", slog.String("error", err.Error()))
				}
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
				if !HasPermission(perm, userPermissions) {
					if err := render.Render(w, r, ErrForbidden); err != nil {
						slog.Error("failed to render forbidden response", slog.String("error", err.Error()))
					}
					return
				}
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(hfn)
	}
}

// HasPermission checks if the specified permission is included in the permissions list.
// Supports wildcard matching for resource and action components (e.g. "admin:*", "config:*").
// Also used by service-layer permission checks that need wildcard support.
func HasPermission(required string, permissions []string) bool {
	// Special case: empty required permission always matches
	if required == "" {
		return true
	}

	// Check for admin wildcard permission first - grants all permissions
	if HasAdminWildcard(permissions) {
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

// HasAdminWildcard reports whether the permission set carries a system-wide
// admin scope (admin:* or *:*). Handlers use this to decide admin-vs-scoped
// behavior consistently with the rest of the authorization layer instead of
// relying on the literal "admin" role name (claims.IsAdmin), which misses
// custom roles and service accounts granted the wildcard.
func HasAdminWildcard(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}

// HasEffectiveAdminScope reports whether the authenticated caller has the
// literal admin role or a system-wide admin permission.
func HasEffectiveAdminScope(ctx context.Context) bool {
	return jwt.ClaimsFromCtx(ctx).IsAdmin ||
		HasAdminWildcard(jwt.PermissionsFromCtx(ctx))
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
