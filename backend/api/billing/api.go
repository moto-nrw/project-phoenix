// Package billing exposes the school-facing contract overview (#1459 demo).
//
// Read-only by design: every value on it is maintained by the moto team, in
// the operator portal. There is no write route here at all, so a school cannot
// raise its own tier or mark its own invoice paid — not even by calling the
// API directly.
package billing

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	billingSvc "github.com/moto-nrw/project-phoenix/services/billing"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/uptrace/bun"
)

// Resource wires the tenant-facing contract routes.
type Resource struct {
	Service  billingSvc.Service
	settings configSvc.SettingsService
	db       *bun.DB
}

// NewResource builds the contract HTTP resource. settings is optional and only
// used to prefetch the vertrag.* keys in one query per request.
func NewResource(service billingSvc.Service, settings configSvc.SettingsService, db *bun.DB) *Resource {
	return &Resource{Service: service, settings: settings, db: db}
}

// Router mounts GET / behind the tenant session.
//
// config:manage, matching the /payroll page: contract and payment data is
// commercial information about the school, not day-to-day care data that every
// Betreuungskraft needs.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		r.With(authorize.RequiresPermission(permissions.ConfigManage), withTx).Get("/", rs.getOverview)
	})

	return r
}

// getOverview returns the contract facts, the live child count and the
// payment schedule in one payload.
func (rs *Resource) getOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if rs.settings != nil {
		// One batched settings read instead of ten single-key round trips.
		ctx = common.PrefetchSettings(ctx, rs.settings, billingSvc.ContractSettingKeys()...)
	}

	overview, err := rs.Service.GetOverview(ctx)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"Vertragsdaten konnten nicht geladen werden.", err))
		return
	}

	common.Respond(w, r, http.StatusOK, overview, "")
}
