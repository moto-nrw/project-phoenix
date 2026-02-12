package jwt

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantMiddleware extracts multi-tenancy fields from JWT claims and sets them
// on the request context. It must be placed AFTER the Authenticator middleware
// in the middleware chain.
//
// In Phase 1, this middleware is created but NOT yet mounted in the router.
// It will be wired into the Chi middleware chain in Phase 3.
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromCtx(r.Context())
		if claims.ID == 0 {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = tenant.WithTenantID(ctx, claims.TenantID)
		ctx = tenant.WithOrgID(ctx, claims.OrgID)
		ctx = tenant.WithScope(ctx, claims.Scope)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
