package sse

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Router returns a configured router for SSE endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// SSE endpoint requires authentication
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)

		r.Get("/events", rs.eventsHandler)
	})

	return r
}

// eventsHandler handles Server-Sent Events connections
// Orchestrates: connection setup → subscription resolution → event streaming
func (rs *Resource) eventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Step 1: Setup SSE connection (headers, flusher validation)
	conn, statusCode := rs.setupSSEConnection(w)
	if conn == nil {
		slog.WarnContext(ctx, "SSE streaming unsupported by client")
		http.Error(w, "Streaming unsupported", statusCode)
		return
	}

	// Step 2: Extract tenant ID from JWT context (set by TenantMiddleware)
	conn.tenantID = tenant.FromContext(ctx)
	conn.isAdmin = hasEffectiveAdminScope(ctx)

	// Step 3: Resolve staff + subscription topics.
	topics, staffID, err := rs.resolveSSESubscription(ctx, conn.tenantID)
	if err != nil {
		rs.writeSSESetupError(w, ctx, err)
		return
	}
	conn.staffID = staffID
	conn.topics = topics

	// Step 4: Send initial "connected" event
	if err := conn.sendConnectedEvent(topics); err != nil {
		slog.ErrorContext(ctx, "SSE failed to send connected event", slog.String("error", err.Error()))
		http.Error(w, "Failed to initialize SSE stream", http.StatusInternalServerError)
		return
	}

	// Step 5: Register every authenticated client (even zero-topic ones
	// need BroadcastToAll events for dashboard count refreshes).
	rs.createAndRegisterClient(conn)
	rs.runEventLoop(ctx, conn)
}

func hasEffectiveAdminScope(ctx context.Context) bool {
	return jwt.ClaimsFromCtx(ctx).IsAdmin ||
		authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx))
}

// resolveSSESubscription resolves the client's topic subscription inside a
// tenant transaction. Resolution requires it because RLS on users.persons and
// users.staff needs app.current_tenant_id to be set — the SSE router uses
// jwt.TenantMiddleware (context only), not the DB-level TenantTxMiddleware.
func (rs *Resource) resolveSSESubscription(ctx context.Context, tenantID int64) (*sseTopics, int64, error) {
	var sub *usercontext.SSESubscription
	err := tenant.WithTenantTx(ctx, rs.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		resolved, resolveErr := rs.userCtx.ResolveSSESubscription(txCtx)
		if resolveErr != nil {
			return resolveErr
		}
		sub = resolved
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return &sseTopics{
		activeGroupIDs: sub.ActiveGroupIDs,
		eduTopics:      sub.EduTopics,
		allTopics:      sub.AllTopics,
	}, sub.StaffID, nil
}

// writeSSESetupError maps a subscription-resolution error to the SSE HTTP
// response: the carried status for an *SSESetupError (401/403), 500 otherwise.
func (rs *Resource) writeSSESetupError(w http.ResponseWriter, ctx context.Context, err error) {
	var setupErr *usercontext.SSESetupError
	if errors.As(err, &setupErr) {
		slog.WarnContext(ctx, "SSE setup failed", slog.String("error", setupErr.Message))
		http.Error(w, setupErr.Message, setupErr.Status)
		return
	}
	slog.ErrorContext(ctx, "SSE failed to determine supervised groups", slog.String("error", err.Error()))
	http.Error(w, "Failed to determine supervised groups", http.StatusInternalServerError)
}
