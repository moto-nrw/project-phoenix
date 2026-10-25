// Package traegerviewhttp serves the Träger projection's reads in the
// Träger-Portal (ADR 0084). Its routes sit behind the Träger route group,
// which accepts only Träger sessions (scope org), and each route checks a
// Träger right. The Träger comes from the session, never from the request.
package traegerviewhttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
)

// Resource serves the Träger projection routes.
type Resource struct {
	overview traegerview.Overview
}

// NewResource builds the resource.
func NewResource(overview traegerview.Overview) *Resource {
	if overview == nil {
		panic("traeger view http: the overview is required")
	}
	return &Resource{overview: overview}
}

// TraegerRouter returns the routes mounted at /traeger/schools inside the
// Träger route group the portal supplies.
func (rs *Resource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router) {
		r.With(common.RequiresPermission(identityaccess.TraegerRightOverviewRead)).Get("/", rs.listSchools)
	})
	return r
}

// listSchools answers GET /traeger/schools: every school of the Träger of the
// session. School names are no personal data, so the read is not written to
// the access log (ADR 0084 section 7).
func (rs *Resource) listSchools(w http.ResponseWriter, r *http.Request) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return
	}
	schools, err := rs.overview.Schools(r.Context(), principal.OrganizationID())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, schools, "Schools retrieved successfully")
}
