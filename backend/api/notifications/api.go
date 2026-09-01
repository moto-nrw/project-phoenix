// Package notifications exposes the notification abstraction (#1624) over
// HTTP. Its test endpoint lets staff fire a notification to their own account
// to verify the tenant's setup end to end; real producers (reminders, #669)
// call the service directly.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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

// portalCtxKey carries which portal a request entered through. The handlers
// are shared between the OGS portal (Router) and the school portal
// (SchoolRouter, #2208); the portal decides which catalogue a person sees and
// with which portal a device is registered.
type portalCtxKey struct{}

func withPortal(portal string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), portalCtxKey{}, portal)))
		})
	}
}

// requestPortal is the notification-catalogue portal of the request: the
// school portal when the request came through SchoolRouter, staff otherwise.
func requestPortal(r *http.Request) string {
	if portal, ok := r.Context().Value(portalCtxKey{}).(string); ok && portal != "" {
		return portal
	}
	return notificationsService.PortalStaff
}

// Router mounts the notification routes behind tenant JWT auth.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, rs.registerRoutes)

	return r
}

// SchoolRouter mounts the same routes for school-scope tokens (#2208). A
// Lehrkraft manages her own devices and decisions exactly like a
// Betreuungskraft; only the catalogue is narrowed to what the school portal
// offers, and devices are recorded with portal "school".
func (rs *Resource) SchoolRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(withPortal(notificationsService.PortalSchool))

	common.ProtectedSchoolGroup(r, rs.db, rs.registerRoutes)

	return r
}

func (rs *Resource) registerRoutes(r chi.Router, withTx common.Middleware) {
	// The event is scoped to the logged-in account, so a valid session is
	// sufficient. No user can trigger a test for someone else.
	r.With(withTx).Post("/test", rs.sendTestNotification)

	// Push subscription management for the logged-in user's own devices — no
	// extra permission beyond a valid session.
	// Push services open their own tenant/admin transactions. In particular,
	// school endpoint rebinding must not try to elevate an ambient tenant tx.
	r.With(common.TenantOperationMiddleware).Route("/push", func(r chi.Router) {
		r.Get("/public-key", rs.getPushPublicKey)
		r.Post("/subscriptions", rs.subscribePush)
		r.Delete("/subscriptions", rs.unsubscribePush)
	})

	// Which notifications the logged-in user agreed to receive. Own data, so
	// no permission beyond a valid session — the same reasoning as the push
	// routes above.
	r.With(withTx).Route("/preferences", func(r chi.Router) {
		r.Get("/", rs.listPreferences)
		r.Delete("/", rs.deleteAllPreferences)
		r.Put("/{type}", rs.setPreference)
	})
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
		Type: "test", IdempotencyKey: fmt.Sprintf("notification-test:%d", time.Now().UTC().UnixNano()),
		Audience: notificationsService.Audience{
			TenantID:        tenantID,
			Scope:           notificationsService.ScopeStaff,
			StaffAccountIDs: []int64{*accountID},
		},
		// The test proves the setup of the portal the person is standing in,
		// so it is delivered there and nowhere else (#2208).
		Portal:   requestPortal(r),
		Priority: notificationsService.PriorityNormal,
		Title:    "Testbenachrichtigung",
		Body:     "Die Benachrichtigungen sind korrekt eingerichtet.",
		DeepLink: "/dashboard",
		// The school portal has no dashboard; its root is the Klassenansicht.
		SchoolDeepLink: "/school",
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
