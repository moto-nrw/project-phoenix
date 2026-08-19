// Package pwa exposes the PWA standalone-usage report endpoint (#2189).
// Portals POST here once per session when running in standalone display
// mode; the operator dashboard reads the aggregate elsewhere.
package pwa

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	pwaService "github.com/moto-nrw/project-phoenix/services/pwa"
	"github.com/uptrace/bun"
)

// Resource wires the PWA usage routes.
type Resource struct {
	Service pwaService.UsageService
	db      *bun.DB
}

// NewResource builds the PWA usage HTTP resource.
func NewResource(service pwaService.UsageService, db *bun.DB) *Resource {
	return &Resource{Service: service, db: db}
}

// Router mounts the PWA usage routes behind tenant JWT auth.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		// The report is scoped to the logged-in account; no body, no
		// parameters — a valid tenant session is all it takes.
		r.With(withTx).Post("/usage", rs.reportUsage)
	})

	return r
}

// reportUsage records that the caller's session runs in standalone display
// mode. Idempotent: repeated reports only advance last_seen_at.
func (rs *Resource) reportUsage(w http.ResponseWriter, r *http.Request) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}
	if err := rs.Service.ReportStaff(r.Context(), int64(claims.ID)); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("App-Nutzung konnte nicht gespeichert werden.", err))
		return
	}
	common.Respond(w, r, http.StatusNoContent, nil, "")
}
