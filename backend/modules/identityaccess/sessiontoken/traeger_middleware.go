package sessiontoken

import "net/http"

// TraegerMiddleware gates a route to Träger-Portal tokens only (ADR 0084):
// scope org with a Träger and without a school. It rejects tenant, school,
// parent and platform tokens, the symmetric guard to TenantMiddleware's org
// rejection. It binds the Träger and the scope but no tenant: a Träger session
// opens no tenant transaction, and the Träger projection filters on the
// schools of the Träger itself. Must be placed AFTER the Authenticator.
func TraegerMiddleware(scope TenantScope) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := ClaimsFromCtx(r.Context())
			if claims.ID == 0 || claims.Scope != claimScopeOrg || claims.OrgID <= 0 || claims.TenantID != 0 {
				renderUnauthorized(w, r, ErrTokenUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(scope.BindOrganization(r.Context(), claims.OrgID, claims.Scope)))
		})
	}
}
