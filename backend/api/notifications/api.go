// Package notifications exposes the notification abstraction (#1624) over
// HTTP. Its single endpoint lets an admin fire a test notification to verify
// the tenant's setup end to end; real producers (reminders, #669) call the
// service directly.
package notifications

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Resource wires the notification routes.
type Resource struct {
	NotificationsService notificationsService.Service
	db                   *bun.DB
}

// NewResource builds the notifications HTTP resource.
func NewResource(service notificationsService.Service, db *bun.DB) *Resource {
	return &Resource{NotificationsService: service, db: db}
}

// Router mounts the notification routes behind tenant JWT auth.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		// config:update — the same permission that guards the feature flag —
		// so exactly the admins who can enable notifications can test them.
		r.With(authorize.RequiresPermission(permissions.ConfigUpdate), withTx).Post("/test", rs.sendTestNotification)
	})

	return r
}

// sendTestNotification fires a fixed, display-safe test event to the caller's
// whole tenant so an admin can verify the notification setup (feature flag on,
// SSE connected, toast rendering) without waiting for a real producer.
func (rs *Resource) sendTestNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if rs.NotificationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("notifications service is not configured")))
		return
	}

	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("missing tenant context")))
		return
	}

	err := rs.NotificationsService.Notify(ctx, notificationsService.Event{
		Type:     "test",
		Audience: notificationsService.Audience{TenantID: tenantID, Scope: notificationsService.ScopeTenant},
		Priority: notificationsService.PriorityNormal,
		Title:    "Testbenachrichtigung",
		Body:     "Die Benachrichtigungen sind korrekt eingerichtet.",
		DeepLink: "/dashboard",
	})
	switch {
	case errors.Is(err, notificationsService.ErrDisabled):
		common.RenderError(w, r, common.ErrorConflict(errors.New("notifications are disabled for this tenant")))
		return
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Test notification dispatched")
}
