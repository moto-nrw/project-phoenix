//go:build testhooks

package careplanhttp

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	render "github.com/go-chi/render"
	"github.com/moto-ogs/project-phoenix/api/common"
)

// ScopeTestTenantRoutes binds real native gates supplied by the shared
// fixture catalog without adding a test import of the HTTP runtime.
func ScopeTestTenantRoutes(authenticate, scope func(http.Handler) http.Handler, preview func(context.Context) bool, principal common.PrincipalFactsReader, settingsCache, identityCache func(http.Handler) http.Handler) func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, register func(chi.Router, func(http.Handler) http.Handler)) {
		common.ProtectedTenantRoutes(r, register, common.SessionLinks{Authenticate: authenticate, Preview: common.ReadOnlyPreviewGuard(preview), Scope: scope, Principal: common.PrincipalFromFacts(principal)}.Chain(), settingsCache, identityCache)
	}
}

func (rs *Resource) Router(protected func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protected(r, rs.Register)
	return r
}
