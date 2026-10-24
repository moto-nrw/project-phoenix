//go:build testhooks

package careplanhttp

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	render "github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// scopeTestTenantRoutes binds real native gates supplied by the shared
// fixture catalog without adding an internal-test import of the HTTP runtime.
func scopeTestTenantRoutes(authenticate, scope func(http.Handler) http.Handler, preview func(context.Context) bool, principal common.PrincipalFactsReader, settingsCache, identityCache func(http.Handler) http.Handler) func(chi.Router, func(chi.Router, func(http.Handler) http.Handler)) {
	return func(r chi.Router, register func(chi.Router, func(http.Handler) http.Handler)) {
		common.ProtectedTenantRoutes(r, register, authenticate, common.ReadOnlyPreviewGuard(preview), scope, common.PrincipalFromFacts(principal), settingsCache, identityCache)
	}
}

func (rs *Resource) Router(protected func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protected(r, rs.Register)
	return r
}
