// Package notifications exposes the notification abstraction (#1624) over
// HTTP. Its test endpoint lets staff fire a notification to their own account
// to verify the tenant's setup end to end; real producers (reminders, #669)
// call the service directly.
package notifications

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Resource wires the notification routes.
type Resource struct {
	NotificationsService notificationsService.Service
	PushService          notificationsService.PushSubscriptionService
	PreferenceService    notificationsService.PreferenceService
	db                   *bun.DB
}

// NewResource builds the notifications HTTP resource.
func NewResource(
	service notificationsService.Service,
	pushService notificationsService.PushSubscriptionService,
	preferenceService notificationsService.PreferenceService,
	db *bun.DB,
) *Resource {
	return &Resource{
		NotificationsService: service,
		PushService:          pushService,
		PreferenceService:    preferenceService,
		db:                   db,
	}
}

// Router mounts the notification routes behind tenant JWT auth.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {
		// The event is scoped to the logged-in staff account, so a valid tenant
		// session is sufficient. No user can trigger a test for someone else.
		r.With(withTx).Post("/test", rs.sendTestNotification)

		// Push subscription management for the logged-in staff user's own
		// devices — no extra permission beyond a valid tenant session.
		r.With(withTx).Route("/push", func(r chi.Router) {
			r.Get("/public-key", rs.getPushPublicKey)
			r.Post("/subscriptions", rs.subscribePush)
			r.Delete("/subscriptions", rs.unsubscribePush)
		})

		// Which notifications the logged-in user agreed to receive. Own data,
		// so no permission beyond a valid tenant session — the same reasoning
		// as the push routes above.
		r.With(withTx).Route("/preferences", func(r chi.Router) {
			r.Get("/", rs.listPreferences)
			r.Delete("/", rs.deleteAllPreferences)
			r.Put("/{type}", rs.setPreference)
		})
	})

	return r
}

// sendTestNotification fires a fixed, display-safe test event only to the
// requesting staff account. This verifies the notification setup (feature flag
// on, SSE connected, toast rendering) without broadcasting an unsolicited test
// to other staff.
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
	accountID := jwt.ActorAccountIDFromCtx(ctx)
	if accountID == nil {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}

	err := rs.NotificationsService.Notify(ctx, notificationsService.Event{
		Type: "test",
		Audience: notificationsService.Audience{
			TenantID:        tenantID,
			Scope:           notificationsService.ScopeStaff,
			StaffAccountIDs: []int64{*accountID},
		},
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
