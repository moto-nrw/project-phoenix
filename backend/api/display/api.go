// Package display serves the established /api/display contract through the
// Device Fleet capability (issue #1325): a public, token-authenticated
// dashboard endpoint plus JWT-gated admin CRUD for the screens themselves.
// Authentication, tenant transactions, rendering, and the display.enabled
// toggle are supplied by the composition root.
package display

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// Middleware is the chi middleware shape the composition root supplies.
type Middleware = func(http.Handler) http.Handler

// FailureKind classifies a handler failure for the composition root's
// renderer, so this package never builds an HTTP error body itself.
type FailureKind string

// The failure kinds the display contract can produce.
const (
	FailureInvalid   FailureKind = "invalid"
	FailureForbidden FailureKind = "forbidden"
	FailureNotFound  FailureKind = "not_found"
	FailureInternal  FailureKind = "internal"
)

// Runtime carries everything outside this owner that the routes need.
type Runtime struct {
	// Protected wraps the JWT-authenticated admin group and hands back the
	// tenant-transaction middleware those routes run inside.
	Protected func(chi.Router, func(chi.Router, Middleware))
	// AnyPermission and Permission build the permission middleware.
	AnyPermission func(...string) Middleware
	Permission    func(string) Middleware
	ParseID       func(http.ResponseWriter, *http.Request, string, string) (int64, bool)
	Failure       func(http.ResponseWriter, *http.Request, FailureKind, error, string)
	// FeatureEnabled resolves the opt-in display.enabled toggle for the
	// current tenant. It fails closed: a resolution error must not open the
	// admin routes for a tenant that never enabled the feature.
	FeatureEnabled func(context.Context) (bool, error)
	Read           string
	Manage         string
}

// Resource bundles the display handlers and their dependencies.
type Resource struct {
	displays devicefleet.Capability
	runtime  Runtime
}

// NewResource constructs the display API resource.
func NewResource(displays devicefleet.Capability, runtime Runtime) *Resource {
	if displays == nil || runtime.Protected == nil || runtime.AnyPermission == nil ||
		runtime.Permission == nil || runtime.ParseID == nil || runtime.Failure == nil ||
		runtime.FeatureEnabled == nil || runtime.Read == "" || runtime.Manage == "" {
		panic("display HTTP: all dependencies are required")
	}
	return &Resource{displays: displays, runtime: runtime}
}

// Router returns a chi router scoped to /display. The dashboard route is
// public (token-only auth via the X-Display-Token header); everything else
// requires a staff JWT plus display permissions.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Public route: the token is the only auth signal, carried in the
	// X-Display-Token header — never in the URL, so it cannot leak into
	// request-path logs (backend, Next.js proxy, reverse proxy). It sits
	// outside the auth group so the JWT middleware does not reject the TV
	// browser. Tenant scoping happens inside the owner (admin token lookup →
	// tenant-scoped aggregation), so no tenant middleware here.
	r.Get("/dashboard", rs.getDashboard)

	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		// Listing is readable with either permission: display:read enables
		// view-only roles, and display:manage must not lock its holders out
		// of the very list their mutations operate on.
		r.With(rs.runtime.AnyPermission(rs.runtime.Read, rs.runtime.Manage), withTx).Get("/", rs.listDisplays)
		r.With(rs.runtime.Permission(rs.runtime.Manage), withTx).Post("/", rs.createDisplay)
		r.Route("/{id}", func(r chi.Router) {
			manage := rs.runtime.Permission(rs.runtime.Manage)
			r.With(manage, withTx).Patch("/", rs.updateDisplay)
			r.With(manage, withTx).Post("/regenerate", rs.regenerateToken)
			r.With(manage, withTx).Delete("/", rs.deleteDisplay)
		})
	})

	return r
}

// displayView is the wire shape of one screen. It is spelled out here so the
// owner's value can change without moving the public contract.
type displayView struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
}

func newDisplayView(value devicefleet.Display) displayView {
	return displayView{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt,
		UpdatedAt: value.UpdatedAt, Name: value.Name, IsActive: value.IsActive,
	}
}

// getDashboard serves the public dashboard aggregate for a display token,
// read from the X-Display-Token header.
func (rs *Resource) getDashboard(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-Display-Token")
	if token == "" {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("missing display token"), "")
		return
	}

	payload, err := rs.displays.Dashboard(r.Context(), token)
	if err != nil {
		switch {
		case errors.Is(err, devicefleet.ErrDisplayInactive):
			// A known-but-deactivated display is a valid screen state, not an
			// error: the TV renders "Display deaktiviert" from this.
			render.JSON(w, r, map[string]string{"status": "inactive"})
		case errors.Is(err, devicefleet.ErrDisplayNotFound):
			// Only a genuinely unknown/revoked token 404s — the TV replaces
			// its dashboard with the deactivated-link screen on 404.
			rs.runtime.Failure(w, r, FailureNotFound, errors.New("display not found"), "")
		default:
			// Infrastructure failures surface as 500 so the TV keeps its last
			// good data and retries instead of going dark.
			rs.runtime.Failure(w, r, FailureInternal, err, "failed to load display dashboard")
		}
		return
	}

	render.JSON(w, r, NewDashboardResponse(payload))
}

// featureEnabled reports whether the info-point dashboard is enabled for the
// current tenant, rendering the failure and returning false when it is not.
// The feature is opt-in (defaults off), unlike the meal plan's opt-out.
//
// Fails CLOSED on a resolution error: a transient/RLS/config-table failure
// while reading the override must NOT silently open admin routes for a tenant
// that never enabled the feature.
func (rs *Resource) featureEnabled(w http.ResponseWriter, r *http.Request) bool {
	enabled, err := rs.runtime.FeatureEnabled(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, FailureInternal, err, "failed to resolve display setting")
		return false
	}
	if enabled {
		return true
	}
	rs.runtime.Failure(w, r, FailureForbidden, errors.New("feature_disabled"), "")
	return false
}

func (rs *Resource) listDisplays(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	displays, err := rs.displays.ListDisplays(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, FailureInternal, err, "failed to list displays")
		return
	}
	views := make([]displayView, 0, len(displays))
	for _, value := range displays {
		views = append(views, newDisplayView(value))
	}
	render.JSON(w, r, map[string]any{"displays": views})
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
		rs.runtime.Failure(w, r, FailureInvalid, err, "")
		return
	}
	if req.Name == nil {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("name is required"), "")
		return
	}

	created, rawToken, err := rs.displays.CreateDisplay(r.Context(), *req.Name)
	if err != nil {
		rs.renderError(w, r, err)
		return
	}

	render.Status(r, http.StatusCreated)
	// The raw token appears exactly once, in this response. It is never
	// persisted or logged; the admin UI shows it in a one-time modal.
	render.JSON(w, r, map[string]any{"display": newDisplayView(created), "token": rawToken})
}

func (rs *Resource) updateDisplay(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	id, ok := rs.parseDisplayID(w, r)
	if !ok {
		return
	}

	var req displayWriteRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err, "")
		return
	}
	if req.Name == nil && req.IsActive == nil {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("nothing to update"), "")
		return
	}

	updated, err := rs.displays.UpdateDisplay(r.Context(), id, req.Name, req.IsActive)
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	render.JSON(w, r, map[string]any{"display": newDisplayView(updated)})
}

func (rs *Resource) regenerateToken(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	id, ok := rs.parseDisplayID(w, r)
	if !ok {
		return
	}

	rawToken, err := rs.displays.RegenerateDisplayToken(r.Context(), id)
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	// Same one-time contract as createDisplay.
	render.JSON(w, r, map[string]any{"token": rawToken})
}

func (rs *Resource) deleteDisplay(w http.ResponseWriter, r *http.Request) {
	if !rs.featureEnabled(w, r) {
		return
	}
	id, ok := rs.parseDisplayID(w, r)
	if !ok {
		return
	}

	if err := rs.displays.DeleteDisplay(r.Context(), id); err != nil {
		rs.renderError(w, r, err)
		return
	}
	render.NoContent(w, r)
}

func (rs *Resource) parseDisplayID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return rs.runtime.ParseID(w, r, "id", "invalid display ID")
}

func (rs *Resource) renderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, devicefleet.ErrDisplayNotFound):
		rs.runtime.Failure(w, r, FailureNotFound, err, "")
	case errors.Is(err, devicefleet.ErrInvalidDisplayInput):
		rs.runtime.Failure(w, r, FailureInvalid, err, "")
	default:
		rs.runtime.Failure(w, r, FailureInternal, err, "display operation failed")
	}
}
