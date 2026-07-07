// Package display holds the info-point display HTTP layer (issue #1325):
// a public, token-authenticated dashboard endpoint plus JWT-gated admin CRUD
// for managing the displays themselves.
package display

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	displayModels "github.com/moto-nrw/project-phoenix/models/display"
	displayService "github.com/moto-nrw/project-phoenix/services/display"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Resource bundles the display handlers and their dependencies.
type Resource struct {
	Service displayService.Service
	db      *bun.DB
}

// NewResource constructs the display API resource.
func NewResource(svc displayService.Service, db *bun.DB) *Resource {
	return &Resource{Service: svc, db: db}
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
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		// Listing is readable with either permission: display:read enables
		// view-only roles, display:manage must not lock its holders out of
		// the very list their mutations operate on.
		r.With(authorize.RequiresAnyPermission(permissions.DisplayRead, permissions.DisplayManage), withTx).Get("/", rs.listDisplays)
		r.With(authorize.RequiresPermission(permissions.DisplayManage), withTx).Post("/", rs.createDisplay)
		r.Route("/{id}", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.DisplayManage), withTx).Patch("/", rs.updateDisplay)
			r.With(authorize.RequiresPermission(permissions.DisplayManage), withTx).Post("/regenerate", rs.regenerateToken)
			r.With(authorize.RequiresPermission(permissions.DisplayManage), withTx).Delete("/", rs.deleteDisplay)
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

func (rs *Resource) listDisplays(w http.ResponseWriter, r *http.Request) {
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid display ID")))
		return 0, false
	}
	return id, true
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
