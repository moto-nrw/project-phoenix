package parentbillinghttp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	capability "github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// DatevExportResource serves the DATEV booking batch of the Träger (#4694)
// at /traeger/datev-export. For now it only checks the settings; the file
// follows with the export protocol (#4693). Every route sits behind the
// Träger session, needs the Träger right "Abrechnung" and runs in the
// Träger transaction of the session's Träger.
type DatevExportResource struct {
	export         capability.DatevExport
	requireBilling common.Middleware
}

// NewDatevExportResource builds the resource. requireBilling admits only a
// session with the Träger right "Abrechnung".
func NewDatevExportResource(export capability.DatevExport, requireBilling common.Middleware) *DatevExportResource {
	if export == nil || requireBilling == nil {
		panic("parent billing http: the DATEV export and the right check are required")
	}
	return &DatevExportResource{export: export, requireBilling: requireBilling}
}

// Router returns the routes mounted at /traeger/datev-export.
func (rs *DatevExportResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		r.With(rs.requireBilling, withTx).Get("/readiness", rs.readiness)
	})
	return r
}

// DatevReadinessResponse names the keys of the settings that are missing
// or invalid, never their values.
type DatevReadinessResponse struct {
	Ready           bool     `json:"ready"`
	MissingSettings []string `json:"missing_settings"`
	InvalidSettings []string `json:"invalid_settings"`
}

func (rs *DatevExportResource) readiness(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	readiness, err := rs.export.DatevReadiness(r.Context(), reviewer)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, DatevReadinessResponse(readiness), "DATEV readiness retrieved successfully")
}
