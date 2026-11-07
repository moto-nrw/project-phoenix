//go:build testhooks

package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayhttp "github.com/moto-nrw/project-phoenix/modules/classday/http"
)

// NewSlotListTestRouter mounts the supplied class-day composition behind the
// same school-scope and permission boundary as the production timetable host.
func NewSlotListTestRouter(service classday.SlotLists) chi.Router {
	router := chi.NewRouter()
	ProtectedSchoolRoutes(router, func(r chi.Router, withTx common.Middleware) {
		classdayhttp.NewSlotListResource(service).Register(r, withTx)
	})
	return router
}
