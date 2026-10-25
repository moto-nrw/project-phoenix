package api

import (
	"context"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	sseHTTP "github.com/moto-nrw/project-phoenix/modules/delivery/http/sse"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewSSEHTTPRuntime binds the stream to the real session, token lifetime and tenant unit of work.
func NewSSEHTTPRuntime() sseHTTP.Runtime[realtime.Event, *realtime.Client] {
	return sseHTTP.Runtime[realtime.Event, *realtime.Client]{
		Identity: func(ctx context.Context) sseHTTP.Identity {
			claims := sessiontoken.ClaimsFromCtx(ctx)
			return sseHTTP.Identity{AccountID: int64(claims.ID), TenantID: tenant.FromContext(ctx), IsAdmin: claims.IsAdmin || securityruntime.HasAdminWildcard(sessiontoken.PermissionsFromCtx(ctx))}
		},
		TokenDeadline: withSSETokenDeadline,
		NewClient:     realtime.NewSSEClient,
		WithinTenant: func(ctx context.Context, tenantID int64, run func(context.Context) error) error {
			id, err := tenant.NewTenantID(tenantID)
			if err != nil {
				return err
			}
			return tenant.WithinTenant(ctx, id, run)
		},
	}
}

func withSSETokenDeadline(ctx context.Context) (context.Context, context.CancelFunc, bool) {
	token, _, err := jwtauth.FromContext(ctx)
	if err != nil || token == nil {
		return ctx, func() {}, false
	}
	expiresAt, ok := token.Expiration()
	if !ok || !expiresAt.After(time.Now()) {
		return ctx, func() {}, false
	}
	deadlineCtx, cancel := context.WithDeadline(ctx, expiresAt)
	return deadlineCtx, cancel, true
}

// NewSSEHTTPRouter mounts staff events behind the existing tenant JWT guard.
func NewSSEHTTPRouter(resource *sseHTTP.Resource[realtime.Event, *realtime.Client]) chi.Router {
	r := chi.NewRouter()

	// Create JWT auth instance for middleware

	// SSE endpoint requires authentication
	r.Group(func(r chi.Router) {
		r.Use(sessiontoken.Authenticator)
		r.Use(sessiontoken.TenantMiddleware(tenant.ClaimScope{}))
		r.Get("/events", resource.StaffEvents)
	})

	return r
}

func newParentSSERouter(resource *sseHTTP.Resource[realtime.Event, *realtime.Client]) chi.Router {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(sessiontoken.Authenticator)
		r.Use(ParentScopeMiddleware)
		r.Get("/events", resource.ParentEvents)
	})

	return r
}

func newSchoolSSERouter(resource *sseHTTP.Resource[realtime.Event, *realtime.Client]) chi.Router {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(sessiontoken.Authenticator)
		r.Use(SchoolScopeMiddleware)
		r.Get("/events", resource.SchoolEvents)
	})

	return r
}
