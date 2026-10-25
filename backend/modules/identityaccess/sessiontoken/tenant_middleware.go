package sessiontoken

import "net/http"

// TenantMiddleware extracts multi-tenancy fields from JWT claims and sets them
// on the request context. It must be placed AFTER the Authenticator middleware
// in the middleware chain.
//
// Träger-Portal tokens (scope org, ADR 0084) carry no school and are rejected
// like parent and school tokens: they reach only the Träger route group.
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
			// Portal sessions cannot enter tenant endpoints, even when their
			// storage row names a school (ADR 0077, ADR 0084).
			if claims.ID == 0 || claims.Scope == claimScopeParent || claims.Scope == claimScopeSchool || claims.Scope == claimScopeCaterer || claims.Scope == claimScopeOrg {
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
