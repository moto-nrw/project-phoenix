// Package display holds the info-point display HTTP layer (issue #1325):
// a public, token-authenticated dashboard endpoint plus JWT-gated admin CRUD
// for managing the displays themselves.
package display

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	displayModels "github.com/moto-nrw/project-phoenix/models/display"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	displayService "github.com/moto-nrw/project-phoenix/services/display"
)

// Resource bundles the display handlers and their dependencies.
type Resource struct {
	Service         displayService.Service
	SettingsService configService.SettingsService
	db              *bun.DB
}

// NewResource constructs the display API resource.
func NewResource(svc displayService.Service, settingsService configService.SettingsService, db *bun.DB) *Resource {
	return &Resource{Service: svc, SettingsService: settingsService, db: db}
}

// Router returns a chi router scoped to /display. The dashboard route is
// public (token-only auth via the X-Display-Token header); everything else
// requires a staff JWT + display permissions.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Public route: the token is the only auth signal, carried in the
	// X-Display-Token header — never in the URL, so it cannot leak into
	// request-path logs (backend, Next.js proxy, reverse proxy). Sits outside
	// the auth group below so the JWT middleware doesn't reject the TV
	// browser. Tenant scoping happens inside the service (WithAdminTx token
	// lookup → WithTenantTx aggregation) — no tenant tx middleware here.
	r.Get("/dashboard", rs.getDashboard)

	// Authenticated admin endpoints.
	tokenAuth := jwt.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(common.ReadOnlyPreviewMiddleware)
		r.Use(jwt.TenantMiddleware)
		r.Use(common.SecurityPrincipalMiddleware)
		withTx := common.TenantTxMiddleware

		// Listing is readable with either permission: display:read enables
		// view-only roles, display:manage must not lock its holders out of
		// the very list their mutations operate on.
		r.With(common.RequiresAnyPermission(permissions.DisplayRead, permissions.DisplayManage), withTx).Get("/", rs.listDisplays)
		r.With(common.RequiresPermission(permissions.DisplayManage), withTx).Post("/", rs.createDisplay)
		r.Route("/{id}", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.DisplayManage), withTx).Patch("/", rs.updateDisplay)
			r.With(common.RequiresPermission(permissions.DisplayManage), withTx).Post("/regenerate", rs.regenerateToken)
			r.With(common.RequiresPermission(permissions.DisplayManage), withTx).Delete("/", rs.deleteDisplay)
		})
	})

	return r
}

// getDashboard serves the public dashboard aggregate for a display token,
// read from the X-Display-Token header.
func (rs *Resource) getDashboard(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Display-Token")
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("missing display token")))
		return
	}

	payload, err := rs.Service.Dashboard(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, displayModels.ErrInactive):
			// A known-but-deactivated display is a valid screen state, not
			// an error: the TV renders "Display deaktiviert" from this.
			render.JSON(w, r, map[string]string{"status": "inactive"})
		case errors.Is(err, displayModels.ErrNotFound):
			// Only a genuinely unknown/revoked token 404s — the TV replaces
			// its dashboard with the deactivated-link screen on 404.
			common.RenderError(w, r, common.ErrorNotFound(errors.New("display not found")))
		default:
			// Infrastructure failures surface as 500 so the TV keeps its
			// last good data and retries instead of going dark.
			common.RenderError(w, r, common.ErrorInternalServerWrap("failed to load display dashboard", err))
		}
		return
	}

	render.JSON(w, r, payload)
}

// featureEnabled reports whether the info-point dashboard is enabled for the
// current tenant, rendering a 403 and returning false when it is not. The
// feature is opt-in (defaults off), unlike the meal plan's opt-out pattern.
//
// Fails CLOSED on a settings lookup error: a transient/RLS/config-table
// failure while reading the override must NOT silently open admin routes for
// a tenant that never enabled the feature, so the error is rendered (500)
// rather than defaulted to disabled-but-passthrough. Routes run inside
// TenantTxMiddleware, so ResolveBool resolves against the current tenant.
func (rs *Resource) featureEnabled(w http.ResponseWriter, r *http.Request) bool {
	enabled, err := rs.SettingsService.ResolveBool(r.Context(), configModel.KeyDisplayEnabled)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to resolve display setting", err))
		return false
	}
	if enabled {
		return true
	}
	common.RenderError(w, r, common.ErrorForbidden(errors.New("feature_disabled")))
	return false
}

func (rs *Resource) listDisplays(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	displays, err := rs.Service.List(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to list displays", err))
		return
	}
	render.JSON(w, r, map[string]any{"displays": displays})
}

type displayWriteRequest struct {
	Name     *string `json:"name"`
	IsActive *bool   `json:"is_active"`
}

func (rs *Resource) createDisplay(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	var req displayWriteRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.Name == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("name is required")))
		return
	}

	d, rawToken, err := rs.Service.Create(r.Context(), *req.Name)
	if err != nil {
		renderDisplayError(w, r, err)
		return
	}

	render.Status(r, http.StatusCreated)
	// The raw token appears exactly once, in this response. It is never
	// persisted or logged; the admin UI shows it in a one-time modal.
	render.JSON(w, r, map[string]any{"display": d, "token": rawToken})
}

func (rs *Resource) updateDisplay(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	id, ok := parseDisplayID(w, r)
	if !ok {
		return
	}

	var req displayWriteRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.Name == nil && req.IsActive == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("nothing to update")))
		return
	}

	d, err := rs.Service.Update(r.Context(), id, req.Name, req.IsActive)
	if err != nil {
		renderDisplayError(w, r, err)
		return
	}
	render.JSON(w, r, map[string]any{"display": d})
}

func (rs *Resource) regenerateToken(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	id, ok := parseDisplayID(w, r)
	if !ok {
		return
	}

	rawToken, err := rs.Service.Regenerate(r.Context(), id)
	if err != nil {
		renderDisplayError(w, r, err)
		return
	}
	// Same one-time contract as createDisplay.
	render.JSON(w, r, map[string]any{"token": rawToken})
}

func (rs *Resource) deleteDisplay(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	id, ok := parseDisplayID(w, r)
	if !ok {
		return
	}

	if err := rs.Service.Delete(r.Context(), id); err != nil {
		renderDisplayError(w, r, err)
		return
	}
	render.NoContent(w, r)
}

func parseDisplayID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParseInt64IDWithError(w, r, "id", "invalid display ID")
}

func renderDisplayError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, displayModels.ErrNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, displayModels.ErrInvalidInput):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("display operation failed", err))
	}
}
