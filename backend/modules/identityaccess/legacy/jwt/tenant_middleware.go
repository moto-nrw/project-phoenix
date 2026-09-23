package jwt

import "net/http"

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
func TenantMiddleware(scope TenantScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromCtx(r.Context())
			// Hard reject parent- and school-scope tokens on tenant endpoints
			// (#2207). School tokens carry a real tenant_id and would otherwise
			// pass RLS silently — this is the symmetric guard to
			// SchoolMiddleware rejecting tenant tokens on /school/*.
			if claims.ID == 0 || claims.Scope == claimScopeParent || claims.Scope == claimScopeSchool {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}
			// Only a platform token may travel without a tenant. Every other
			// token binds one; an invalid ID is observed by the tenant runtime.
			if claims.Scope == claimScopePlatform && claims.TenantID <= 0 {
				next.ServeHTTP(w, r.WithContext(scope.BindOrganization(r.Context(), claims.OrgID, claims.Scope)))
				return
			}
			ctx, err := scope.BindTenant(r.Context(), claims.TenantID, claims.OrgID, claims.Scope)
			if err != nil {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
