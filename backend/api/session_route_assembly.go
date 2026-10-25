package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

func ProtectedTenantRoutes(r chi.Router, fn func(chi.Router, common.Middleware)) {
	common.ProtectedTenantRoutes(r, fn, sessiontoken.Authenticator, ReadOnlyPreviewMiddleware, TenantScopeMiddleware, SecurityPrincipalMiddleware, RequestSettingsCacheMiddleware, RequestIdentityCacheMiddleware)
}

func ProtectedSchoolRoutes(r chi.Router, fn func(chi.Router, common.Middleware)) {
	common.ProtectedSchoolRoutes(r, fn, sessiontoken.Authenticator, ReadOnlyPreviewMiddleware, SchoolScopeMiddleware, SecurityPrincipalMiddleware, RequestSettingsCacheMiddleware, RequestIdentityCacheMiddleware)
}

// ProtectedTraegerRoutes registers the Träger route group (ADR 0084): only
// Träger sessions pass, the security principal carries the Träger rights,
// and no tenant transaction is opened. Routes on tables of the Träger take
// withTx, the Träger transaction of the session's Träger (ADR 0105).
func ProtectedTraegerRoutes(r chi.Router, fn func(chi.Router, common.Middleware)) {
	r.Group(func(gr chi.Router) {
		gr.Use(sessiontoken.Authenticator, ReadOnlyPreviewMiddleware, TraegerScopeMiddleware, SecurityPrincipalMiddleware)
		fn(gr, common.TraegerTxMiddleware)
	})
}

func ProtectedParentGroup(r chi.Router, fn func(chi.Router)) {
	common.ProtectedParentGroup(r, fn, sessiontoken.Authenticator, ParentScopeMiddleware)
}

func TenantSessionChain(r chi.Router) {
	r.Use(sessiontoken.Authenticator, ReadOnlyPreviewMiddleware, TenantScopeMiddleware, SecurityPrincipalMiddleware)
}

func SchoolSessionChain(r chi.Router) {
	r.Use(sessiontoken.Authenticator, ReadOnlyPreviewMiddleware, SchoolScopeMiddleware, SecurityPrincipalMiddleware)
}

func ParentSessionChain(r chi.Router) {
	r.Use(sessiontoken.Authenticator, ReadOnlyPreviewMiddleware, ParentScopeMiddleware, SecurityPrincipalMiddleware)
}
