package jwt

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantMiddleware extracts multi-tenancy fields from JWT claims and sets them
// on the request context. It must be placed AFTER the Authenticator middleware
// in the middleware chain.
//
// Defense-in-depth: rejects parent-scope tokens outright. Parent tokens carry
// tenant_id=0 and an empty role set for any tenant — they could bypass RLS
// silently if smuggled into a tenant route. The host-only parent cookie + the
// proxy already make this leak path impossible in practice, but a hard 403 here
// closes the gap if either fails. Operator (platform) tokens are NOT rejected
// here because some platform endpoints intentionally cross-tenant via WithAdminTx
// (operator dashboards) — they use IsPlatformScope checks at the handler level
// instead.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromCtx(r.Context())
		if claims.ID == 0 {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		// Hard reject parent-scope tokens on tenant endpoints.
		if claims.Scope == tenant.ScopeParent {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		// Hard reject school-scope tokens on tenant endpoints (#2207).
		// School tokens carry a real tenant_id and would otherwise pass
		// RLS silently — this is the symmetric guard to SchoolMiddleware
		// rejecting tenant tokens on /school/*.
		if claims.Scope == tenant.ScopeSchool {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}
		if claims.Scope != tenant.ScopePlatform && claims.TenantID <= 0 {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		ctx := r.Context()
		if claims.TenantID > 0 {
			tenantID, err := tenant.NewTenantID(claims.TenantID)
			if err != nil {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}
			ctx = tenant.WithTenant(ctx, tenantID)
		}
		ctx = tenant.WithOrgID(ctx, claims.OrgID)
		ctx = tenant.WithScope(ctx, claims.Scope)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
