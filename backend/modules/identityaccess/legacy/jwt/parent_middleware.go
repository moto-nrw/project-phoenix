package jwt

import "net/http"

// ParentMiddleware gates a route to parent-scope tokens only. Mirrors
// TenantMiddleware but inverts the scope check: requires
// claims.Scope == claimScopeParent and rejects everything else
// (tenant, org, platform). Must be placed AFTER the Authenticator
// middleware in the chain.
//
// Binds only the scope but intentionally does NOT
// call WithTenantID — parent tokens carry tenant_id=0 and are not
// bound to a single school. Endpoints that want a specific tenant
// resolve it from the URL or request body and validate the parent
// account is mapped to that tenant via auth.account_tenants.
func ParentMiddleware(scope TenantScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromCtx(r.Context())
			if claims.ID == 0 {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}

			// Hard reject anything that isn't a parent-scope token. This is
			// the symmetric guard to TenantMiddleware's parent rejection —
			// taken together they guarantee a token can only be used on the
			// surface area its scope was issued for.
			if claims.Scope != claimScopeParent {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}

			ctx := r.Context()
			ctx = scope.BindScope(ctx, claims.Scope)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
