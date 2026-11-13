package sessiontoken

import (
	"context"
	"net/http"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/tokenprimitives"
)

// TenantScope binds verified claims to the tenant runtime, which implements
// it; this package may not import the tenant runtime (ADR 0054).
type TenantScope interface {
	BindTenant(ctx context.Context, tenantID, orgID int64, scope string) (context.Context, error)
	BindOrganization(ctx context.Context, orgID int64, scope string) context.Context
	BindScope(ctx context.Context, scope string) context.Context
}

// TenantRoutePortals are the portals tenant routes accept. Platform tokens
// stay admitted because operator endpoints cross tenants through WithAdminTx
// and check IsPlatformScope at the handler.
var TenantRoutePortals = []tokenprimitives.PortalClaims{tokenprimitives.TenantClaims, tokenprimitives.PlatformClaims}

// PortalGate admits only tokens of the accepted portals; every other scope,
// including one no portal issues, is answered 401. It binds what the
// portal's token shape names: the school, the Träger, or only the scope.
// It must follow the Authenticator.
func PortalGate(scope TenantScope, accepted ...tokenprimitives.PortalClaims) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromCtx(r.Context())
			portal, ok := acceptedPortal(claims.Scope, accepted)
			if claims.ID == 0 || !ok {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}
			ctx, ok := bindPortal(r.Context(), scope, portal, claims)
			if !ok {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func acceptedPortal(claimScope string, accepted []tokenprimitives.PortalClaims) (tokenprimitives.PortalClaims, bool) {
	portal, ok := tokenprimitives.PortalOfClaim(claimScope)
	if !ok {
		return tokenprimitives.PortalClaims{}, false
	}
	for _, candidate := range accepted {
		if candidate.Name == portal.Name {
			return portal, true
		}
	}
	return tokenprimitives.PortalClaims{}, false
}

// bindPortal lets the tenant runtime observe a missing school before the
// shape refuses it.
func bindPortal(ctx context.Context, scope TenantScope, portal tokenprimitives.PortalClaims, claims AppClaims) (context.Context, bool) {
	switch {
	case portal.Tenant == tokenprimitives.Required || (portal.Tenant == tokenprimitives.Optional && claims.TenantID > 0):
		bound, err := scope.BindTenant(ctx, claims.TenantID, claims.OrgID, claims.Scope)
		return bound, err == nil && portal.Holds(claims.TenantID, claims.OrgID)
	case !portal.Holds(claims.TenantID, claims.OrgID):
		return ctx, false
	case portal.Tenant == tokenprimitives.Ignored:
		return scope.BindScope(ctx, claims.Scope), true
	default:
		return scope.BindOrganization(ctx, claims.OrgID, claims.Scope), true
	}
}
