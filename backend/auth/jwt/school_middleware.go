package jwt

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// SchoolMiddleware gates a route to school-scope tokens only (#2207).
// Mirrors ParentMiddleware's hard scope check: requires
// claims.Scope == tenant.ScopeSchool and rejects everything else
// (tenant, org, platform, parent). Must be placed AFTER the
// Authenticator middleware in the chain.
//
// Unlike parent tokens, school tokens ARE bound to a single school:
// the class-day surface behind /school/* is tenant-scoped and runs
// under RLS, so this middleware sets tenant.WithTenantID (and org/
// scope) on the context exactly like TenantMiddleware does — the
// downstream TenantTxMiddleware needs the tenant id to open the
// per-request tenant transaction. A school token without a tenant_id
// claim would silently see zero rows, so it is rejected outright.
func SchoolMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromCtx(r.Context())
		if claims.ID == 0 {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		// Hard reject anything that isn't a school-scope token. This is
		// the symmetric guard to TenantMiddleware's school rejection —
		// taken together they guarantee a token can only be used on the
		// surface area its scope was issued for.
		if claims.Scope != tenant.ScopeSchool {
			renderUnauthorized(w, r, ErrTokenUnauthorized)
			return
		}

		// School tokens are tenant-bound by construction (the school
		// login refuses accounts without a lehrkraft mapping and pins
		// the resolved school). A zero tenant_id can only mean a
		// hand-crafted or corrupted token — refuse it instead of
		// letting RLS return empty results downstream.
		if claims.TenantID == 0 {
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
