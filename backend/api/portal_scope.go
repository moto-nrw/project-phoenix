package api

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The scope gates of the token adapter, bound to the tenant runtime. Each
// accepts exactly one portal's tokens and must follow sessiontoken.Authenticator.

// TenantScopeMiddleware rejects parent and school tokens, binds the tenant
// and then refuses a route of a module the school has not booked (#4893).
func TenantScopeMiddleware(next http.Handler) http.Handler {
	return sessiontoken.TenantMiddleware(tenant.ClaimScope{})(requireBookedModule(next))
}

// ParentScopeMiddleware accepts parents-portal tokens only and binds no tenant.
func ParentScopeMiddleware(next http.Handler) http.Handler {
	return sessiontoken.ParentMiddleware(tenant.ClaimScope{})(next)
}

// SchoolScopeMiddleware accepts school-portal tokens only and binds their school.
func SchoolScopeMiddleware(next http.Handler) http.Handler {
	return sessiontoken.SchoolMiddleware(tenant.ClaimScope{})(next)
}
