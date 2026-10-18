package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// ProtectedTenantRoutesWith mounts owner resources inside the unchanged tenant chain.
func ProtectedTenantRoutesWith(mounts ...func(chi.Router, common.Middleware)) func(chi.Router, func(chi.Router, common.Middleware)) {
	return func(r chi.Router, register func(chi.Router, common.Middleware)) {
		ProtectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
			register(r, withTx)
			for _, mount := range mounts {
				mount(r, withTx)
			}
		})
	}
}
