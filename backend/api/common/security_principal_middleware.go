package common

import (
	"context"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// PrincipalFactsReader supplies authenticated values without exposing a token.
type PrincipalFactsReader func(context.Context) securityruntime.PrincipalFacts

// PrincipalFromFacts applies the security contract and renders HTTP failures.
func PrincipalFromFacts(readFacts PrincipalFactsReader) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			facts := readFacts(r.Context())
			principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
				AccountID:      facts.AccountID,
				TenantID:       facts.TenantID,
				OrganizationID: facts.OrganizationID,
				Scope:          facts.Scope,
				Roles:          facts.Roles,
				Permissions:    facts.Permissions,
				Admin:          facts.Admin,
				FamilyID:       facts.FamilyID,
			})
			if err != nil {
				securityruntime.ObserveAuthorization(r.Context(), securityruntime.AuthorizationEvent{Outcome: "invalid", Reason: "invalid_principal", Elapsed: time.Since(started)})
				RenderError(w, r, ErrorUnauthorized(permissions.ErrPrincipalRequired))
				return
			}

			securityruntime.ObserveAuthorization(r.Context(), securityruntime.AuthorizationEvent{Outcome: "resolved", Elapsed: time.Since(started)})
			ctx := permissions.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CurrentPrincipal(ctx context.Context) (permissions.Principal, error) {
	return permissions.PrincipalFromContext(ctx)
}

func HasEffectiveAdminScope(ctx context.Context) bool {
	principal, err := CurrentPrincipal(ctx)
	return err == nil && principal.HasAdminScope()
}

func IsAssignmentBoundPortal(ctx context.Context) bool {
	principal, err := CurrentPrincipal(ctx)
	return err == nil && principal.Scope() == permissions.ScopeSchool
}

func RequiresPermission(permission string) Middleware {
	return requirePermissions(func(principal permissions.Principal) bool {
		return principal.HasPermission(permission)
	})
}

func RequiresAnyPermission(required ...string) Middleware {
	return requirePermissions(func(principal permissions.Principal) bool {
		for _, permission := range required {
			if principal.HasPermission(permission) {
				return true
			}
		}
		return false
	})
}

func RequiresAllPermissions(required ...string) Middleware {
	return requirePermissions(func(principal permissions.Principal) bool {
		for _, permission := range required {
			if !principal.HasPermission(permission) {
				return false
			}
		}
		return true
	})
}

// AuthorizationForbidden is the denial of a permission check, in the shared
// error envelope with the status text as its error.
func AuthorizationForbidden() *ErrResponse {
	return &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		Detail:         http.StatusText(http.StatusForbidden),
	}
}

func requirePermissions(allowed func(permissions.Principal) bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := permissions.PrincipalFromContext(r.Context())
			if err != nil {
				securityruntime.ObserveAuthorization(r.Context(), securityruntime.AuthorizationEvent{Reason: "missing_principal"})
				RenderError(w, r, ErrorUnauthorized(permissions.ErrPrincipalRequired))
				return
			}
			if !allowed(principal) {
				securityruntime.ObserveAuthorization(r.Context(), securityruntime.AuthorizationEvent{Reason: "permission_denied"})
				RenderError(w, r, AuthorizationForbidden())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
