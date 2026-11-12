// Package datareturn serves the return package of a school or Träger to its
// customer and to a target provider the customer names (Data Act, ADR 0201 and ADR 0203):
// the register that describes the package, the OpenAPI description of the
// retrieval, and the released package to the holder of its token.
package datareturn

import (
	_ "embed"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
)

//go:embed openapi.json
var openAPI []byte

// Resource serves the register and the OpenAPI description without a
// session (no customer data; the schema is public with the source code),
// and a released package to the bearer of its token.
type Resource struct {
	register []byte
	packages organizationtenancy.DataReturn
	limit    func(http.Handler) http.Handler
}

// NewResource reads the register embedded in this build, so the register
// always describes the package this build writes. packages serves the
// released packages; limit is the rate limit of their routes, nil when the
// deployment runs without rate limits.
func NewResource(packages organizationtenancy.DataReturn, limit func(http.Handler) http.Handler) *Resource {
	if packages == nil {
		panic("data return resource: the package retrieval is required")
	}
	register, _ := organizationtenancy.ReturnRegisterSource.ReadFile(organizationtenancy.ReturnRegisterFile)
	return &Resource{register: register, packages: packages, limit: limit}
}

// Router mounts GET /register, GET /openapi.json, GET /package and
// GET /package/archive.
func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Get("/register", serveJSON(rs.register))
	router.Get("/openapi.json", serveJSON(openAPI))
	router.Group(func(router chi.Router) {
		if rs.limit != nil {
			router.Use(rs.limit)
		}
		router.Get("/package", rs.describe)
		router.Get("/package/archive", rs.download)
	})
	return router
}

func serveJSON(body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
}
