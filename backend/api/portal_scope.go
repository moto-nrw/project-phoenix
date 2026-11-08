package api

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The scope gates of the route groups, bound to the tenant runtime. Each
// names the portals it accepts and must follow sessiontoken.Authenticator.

// TenantScopeMiddleware accepts tenant and operator tokens, binds the tenant
// and then refuses a route of a module the school has not booked (#4893).
func TenantScopeMiddleware(next http.Handler) http.Handler {
	return sessiontoken.PortalGate(tenant.ClaimScope{}, sessiontoken.TenantRoutePortals...)(requireBookedModule(next))
}

// ParentScopeMiddleware accepts parents-portal tokens only, binds no tenant
// and refuses an account none of whose guardian schools books Eltern-App
// (#4895). Parent tokens are not bound to one school: endpoints resolve the
// school from the request and check the account's mapping there.
func ParentScopeMiddleware(next http.Handler) http.Handler {
	return sessiontoken.PortalGate(tenant.ClaimScope{}, tokenprimitives.ParentClaims)(requireParentPortal(next))
}

// TraegerScopeMiddleware accepts Träger-Portal tokens only and binds their
// Träger, never a school (ADR 0084).
func TraegerScopeMiddleware(next http.Handler) http.Handler {
	return sessiontoken.PortalGate(tenant.ClaimScope{}, tokenprimitives.OrgClaims)(next)
}

// SchoolScopeMiddleware accepts school-portal tokens only, binds their
// school and refuses the portal of a school that has not booked Unterricht
// am Vormittag (#4895).
func SchoolScopeMiddleware(next http.Handler) http.Handler {
	return sessiontoken.PortalGate(tenant.ClaimScope{}, tokenprimitives.SchoolClaims)(requireBookedModule(next))
}
