package tenant

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/render"
)

// RequiresPermission middleware restricts access to users with a specific permission.
// This should be used on routes that require a single specific permission.
//
// Example:
//
//	r.With(tenant.RequiresPermission("student:read")).Get("/", handler.list)
//	r.With(tenant.RequiresPermission("student:create")).Post("/", handler.create)
func RequiresPermission(permission string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !HasPermission(r.Context(), permission) {
				_ = render.Render(w, r, ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequiresAnyPermission middleware restricts access to users with ANY of the specified permissions.
// Useful when an endpoint can be accessed by users with different roles.
//
// Example:
//
//	r.With(tenant.RequiresAnyPermission("staff:create", "staff:update")).Put("/", handler.update)
func RequiresAnyPermission(permissions ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, perm := range permissions {
				if HasPermission(r.Context(), perm) {
					next.ServeHTTP(w, r)
					return
				}
			}
			_ = render.Render(w, r, ErrForbidden)
		})
	}
}

// RequiresAllPermissions middleware restricts access to users with ALL specified permissions.
// Use when an operation requires multiple distinct capabilities.
//
// Example:
//
//	r.With(tenant.RequiresAllPermissions("group:update", "group:assign")).Post("/{id}/students", handler.assignStudent)
func RequiresAllPermissions(permissions ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, perm := range permissions {
				if !HasPermission(r.Context(), perm) {
					_ = render.Render(w, r, ErrForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// HasPermission checks if the current user has a specific permission.
// Supports wildcard matching for resource and action components.
//
// Permission format: "resource:action"
// Wildcards: "*" matches any value, "prefix*" matches any value starting with prefix
//
// Examples:
//   - HasPermission(ctx, "student:read") - exact match
//   - User with "admin:*" can access any admin action
//   - User with "*:*" has superuser access
func HasPermission(ctx context.Context, required string) bool {
	// Empty permission always passes (no authorization required)
	if required == "" {
		return true
	}

	permissions := PermissionsFromCtx(ctx)

	// Check for admin wildcard first (superuser access)
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}

	// Parse required permission
	requiredParts := strings.Split(required, ":")
	if len(requiredParts) != 2 {
		return false // Invalid permission format
	}
	requiredResource := requiredParts[0]
	requiredAction := requiredParts[1]

	// Check each permission
	for _, perm := range permissions {
		parts := strings.Split(perm, ":")
		if len(parts) != 2 {
			continue // Skip invalid permissions
		}

		resource := parts[0]
		action := parts[1]

		// Match resource and action (with wildcard support)
		if matchesPattern(resource, requiredResource) && matchesPattern(action, requiredAction) {
			return true
		}
	}

	return false
}

// HasAnyPermission checks if the current user has any of the specified permissions.
// Returns true if at least one permission matches.
func HasAnyPermission(ctx context.Context, permissions ...string) bool {
	for _, perm := range permissions {
		if HasPermission(ctx, perm) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if the current user has all specified permissions.
// Returns true only if every permission matches.
func HasAllPermissions(ctx context.Context, permissions ...string) bool {
	for _, perm := range permissions {
		if !HasPermission(ctx, perm) {
			return false
		}
	}
	return true
}

// matchesPattern checks if a pattern matches a required value.
// Supports:
// - Exact match: "student" matches "student"
// - Wildcard: "*" matches anything
// - Prefix wildcard: "admin*" matches "admin", "administrator", etc.
func matchesPattern(pattern, required string) bool {
	if pattern == required || pattern == "*" {
		return true
	}

	// Prefix wildcard: "student*" matches "student_extended"
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(required, prefix)
	}

	return false
}

// PermissionChecker provides a fluent API for checking permissions in services.
// Use this when you need to perform multiple permission checks efficiently.
type PermissionChecker struct {
	ctx context.Context
}

// NewPermissionChecker creates a new permission checker for the given context.
func NewPermissionChecker(ctx context.Context) *PermissionChecker {
	return &PermissionChecker{ctx: ctx}
}

// Can checks if the user has a specific permission.
func (pc *PermissionChecker) Can(permission string) bool {
	return HasPermission(pc.ctx, permission)
}

// CanAny checks if the user has any of the specified permissions.
func (pc *PermissionChecker) CanAny(permissions ...string) bool {
	return HasAnyPermission(pc.ctx, permissions...)
}

// CanAll checks if the user has all specified permissions.
func (pc *PermissionChecker) CanAll(permissions ...string) bool {
	return HasAllPermissions(pc.ctx, permissions...)
}

// CanReadLocation checks if the user can see location data (GDPR sensitive).
func (pc *PermissionChecker) CanReadLocation() bool {
	return HasPermission(pc.ctx, "location:read")
}
