package common

import (
	"context"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

type AuthorizationEvent struct {
	Outcome string
	Reason  string
	Elapsed time.Duration
}

type AuthorizationObserver func(AuthorizationEvent)

type authorizationObserverContextKey struct{}

func AuthorizationObserverMiddleware(observer AuthorizationObserver) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), authorizationObserverContextKey{}, observer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SecurityPrincipalMiddleware translates authenticated JWT claims into the
// security contract consumed below the inbound adapter. JWT parsing remains in
// auth/jwt; authorization code receives only the validated principal.
func SecurityPrincipalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		claims := jwt.ClaimsFromCtx(r.Context())
		principal, err := permissions.NewPrincipal(permissions.PrincipalInput{
			AccountID:      int64(claims.ID),
			TenantID:       claims.TenantID,
			OrganizationID: claims.OrgID,
			Scope:          claims.Scope,
			Roles:          claims.Roles,
			Permissions:    claims.Permissions,
			Admin:          claims.IsAdmin,
			FamilyID:       claims.FamilyID,
		})
		if err != nil {
			observeAuthorization(r.Context(), AuthorizationEvent{Outcome: "invalid", Reason: "invalid_principal", Elapsed: time.Since(started)})
			RenderError(w, r, ErrorUnauthorized(permissions.ErrPrincipalRequired))
			return
		}

		observeAuthorization(r.Context(), AuthorizationEvent{Outcome: "resolved", Elapsed: time.Since(started)})
		ctx := permissions.WithPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

func AuthorizationForbidden() *ErrResponse {
	return &ErrResponse{
		HTTPStatusCode: http.StatusForbidden,
		Status:         http.StatusText(http.StatusForbidden),
	}
}

func requirePermissions(allowed func(permissions.Principal) bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := permissions.PrincipalFromContext(r.Context())
			if err != nil {
				observeAuthorization(r.Context(), AuthorizationEvent{Reason: "missing_principal"})
				RenderError(w, r, ErrorUnauthorized(permissions.ErrPrincipalRequired))
				return
			}
			if !allowed(principal) {
				observeAuthorization(r.Context(), AuthorizationEvent{Reason: "permission_denied"})
				RenderError(w, r, AuthorizationForbidden())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func observeAuthorization(ctx context.Context, event AuthorizationEvent) {
	observer, _ := ctx.Value(authorizationObserverContextKey{}).(AuthorizationObserver)
	if observer != nil {
		observer(event)
	}
}
