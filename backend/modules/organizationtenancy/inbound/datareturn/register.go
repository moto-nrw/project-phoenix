// Package datareturn serves the return package of a school or Träger to its
// customer and to a target provider the customer names (Data Act, ADR 0200):
// the register that describes the package.
package datareturn

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// Resource serves the register without a session: it holds no customer
// data, and the schema it names is public with the source code.
type Resource struct {
	register []byte
}

// NewResource reads the register embedded in this build, so the register
// always describes the package this build writes. The file is part of the
// binary; a build without it does not compile.
func NewResource() *Resource {
	register, _ := organizationtenancy.ReturnRegisterSource.ReadFile(organizationtenancy.ReturnRegisterFile)
	return &Resource{register: register}
}

// Router mounts GET /register.
func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Get("/register", rs.serveRegister)
	return router
}

func (rs *Resource) serveRegister(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rs.register)
}
