package sessiontoken

import "net/http"

// CatererMiddleware gates a route to caterer-scope tokens only (ADR 0077)
// and rejects every other scope; the tenant, school and parent middlewares
// reject the caterer scope in turn. Must be placed AFTER the Authenticator
// middleware in the chain.
//
// It binds only the scope: caterer tokens carry no school. The caterer
// portal resolves the schools the caterer supplies and opens each in its own
// tenant transaction.
func CatererMiddleware(scope TenantScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromCtx(r.Context())
			if claims.ID == 0 || claims.Scope != claimScopeCaterer {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(scope.BindScope(r.Context(), claims.Scope)))
		})
	}
}
