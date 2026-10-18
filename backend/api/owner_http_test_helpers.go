//go:build testhooks

package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// NewOwnerHTTPTestRouter uses the host's production tenant chain at an owner's mount.
func NewOwnerHTTPTestRouter(register func(chi.Router, common.Middleware), prefix string) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	ProtectedTenantRoutes(r, register)
	if prefix == "" {
		return r
	}
	router := chi.NewRouter()
	router.Use(RequestIdentityCacheMiddleware)
	router.Mount(prefix, r)
	return router
}
