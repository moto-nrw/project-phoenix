package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

func ProtectedTenantRoutes(r chi.Router, fn func(chi.Router, common.Middleware)) {
	common.ProtectedTenantRoutes(r, fn, sessiontoken.Authenticator, TenantScopeMiddleware)
}

func ProtectedSchoolRoutes(r chi.Router, fn func(chi.Router, common.Middleware)) {
	common.ProtectedSchoolRoutes(r, fn, sessiontoken.Authenticator, SchoolScopeMiddleware)
}

func ProtectedParentGroup(r chi.Router, fn func(chi.Router)) {
	common.ProtectedParentGroup(r, fn, sessiontoken.Authenticator, ParentScopeMiddleware)
}

func TenantSessionChain(r chi.Router) {
	r.Use(sessiontoken.Authenticator, common.ReadOnlyPreviewMiddleware, TenantScopeMiddleware, common.SecurityPrincipalMiddleware)
}

func SchoolSessionChain(r chi.Router) {
	r.Use(sessiontoken.Authenticator, common.ReadOnlyPreviewMiddleware, SchoolScopeMiddleware, common.SecurityPrincipalMiddleware)
}

func ParentSessionChain(r chi.Router) {
	r.Use(sessiontoken.Authenticator, common.ReadOnlyPreviewMiddleware, ParentScopeMiddleware, common.SecurityPrincipalMiddleware)
}
