package api

import (
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/tokenprimitives"
)

// portalGates are the scope gates of the account portals' session routes,
// keyed by the portal table. The operator portal mounts its own chain.
var portalGates = map[string]common.Middleware{
	tokenprimitives.PortalTenant:  TenantScopeMiddleware,
	tokenprimitives.PortalSchool:  SchoolScopeMiddleware,
	tokenprimitives.PortalParent:  ParentScopeMiddleware,
	tokenprimitives.PortalCaterer: CatererScopeMiddleware,
	tokenprimitives.PortalOrg:     TraegerScopeMiddleware,
}

// SessionChain is the middleware chain of a portal's session routes.
func SessionChain(portal tokenprimitives.PortalClaims) []common.Middleware {
	gate, ok := portalGates[portal.Name]
	if !ok {
		panic(fmt.Sprintf("portal %q has no session routes", portal.Name))
	}
	return common.SessionLinks{
		Authenticate: sessiontoken.Authenticator, Preview: ReadOnlyPreviewMiddleware, Scope: gate, Principal: SecurityPrincipalMiddleware,
	}.Chain()
}

func sessionChainOf(portal tokenprimitives.PortalClaims) func(chi.Router) {
	chain := SessionChain(portal)
	return func(r chi.Router) { r.Use(chain...) }
}

func ProtectedTenantRoutes(r chi.Router, fn func(chi.Router, common.Middleware)) {
	common.ProtectedTenantRoutes(r, fn, SessionChain(tokenprimitives.TenantClaims), RequestSettingsCacheMiddleware, RequestIdentityCacheMiddleware)
}

func ProtectedSchoolRoutes(r chi.Router, fn func(chi.Router, common.Middleware)) {
	common.ProtectedTenantRoutes(r, fn, SessionChain(tokenprimitives.SchoolClaims), RequestSettingsCacheMiddleware, RequestIdentityCacheMiddleware)
}

// ProtectedTraegerRoutes registers the Träger route group (ADR 0084): only
// Träger sessions pass, the security principal carries the Träger rights,
// and no tenant transaction is opened. Routes on tables of the Träger take
// withTx, the Träger transaction of the session's Träger (ADR 0105).
func ProtectedTraegerRoutes(r chi.Router, fn func(chi.Router, common.Middleware)) {
	r.Group(func(gr chi.Router) {
		gr.Use(SessionChain(tokenprimitives.OrgClaims)...)
		fn(gr, common.TraegerTxMiddleware)
	})
}

// ProtectedParentGroup registers a parents-portal route group. It opens no
// tenant transaction: a parent token is cross-tenant, because a guardian's
// children can attend several schools, so every handler resolves the school
// from the request and checks the account's mapping there.
func ProtectedParentGroup(r chi.Router, fn func(chi.Router)) {
	r.Group(func(gr chi.Router) {
		ParentSessionChain(gr)
		fn(gr)
	})
}

var (
	TenantSessionChain  = sessionChainOf(tokenprimitives.TenantClaims)
	SchoolSessionChain  = sessionChainOf(tokenprimitives.SchoolClaims)
	ParentSessionChain  = sessionChainOf(tokenprimitives.ParentClaims)
	CatererSessionChain = sessionChainOf(tokenprimitives.CatererClaims)
)
