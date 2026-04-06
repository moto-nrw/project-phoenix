package sse

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
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

// sseSetupError carries HTTP error info out of a tenant transaction.
type sseSetupError struct {
	msg    string
	status int
}

func (e *sseSetupError) Error() string { return fmt.Sprintf("SSE setup: %s", e.msg) }

// eventsHandler handles Server-Sent Events connections
// Orchestrates: connection setup → staff resolution → topic subscription → event streaming
func (rs *Resource) eventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Step 1: Setup SSE connection (headers, flusher validation)
	conn, statusCode := rs.setupSSEConnection(w)
	if conn == nil {
		http.Error(w, "Streaming unsupported", statusCode)
		return
	}

	// Step 2: Extract tenant ID from JWT context (set by TenantMiddleware)
	conn.tenantID = tenant.FromContext(ctx)

	// Steps 3-4 require a tenant transaction because RLS on users.persons
	// and users.staff requires app.current_tenant_id to be set.
	var staff *users.Staff
	var topics *sseTopics
	err := tenant.WithTenantTx(ctx, rs.db, conn.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Step 3: Resolve staff member from JWT claims
		resolved, errMsg, code := rs.resolveStaff(txCtx)
		if resolved == nil {
			return &sseSetupError{msg: errMsg, status: code}
		}
		staff = resolved

		// Step 4: Build subscription topics (active groups + educational groups)
		built, err := rs.buildSubscriptionTopics(txCtx, staff.ID)
		if err != nil {
			return err
		}
		topics = built
		return nil
	})
	if err != nil {
		if setupErr, ok := err.(*sseSetupError); ok {
			http.Error(w, setupErr.msg, setupErr.status)
		} else {
			http.Error(w, "Failed to determine supervised groups", http.StatusInternalServerError)
		}
		return
	}
	conn.staffID = staff.ID
	conn.topics = topics

	// Step 5: Send initial "connected" event
	if conn.sendConnectedEvent(topics) != nil {
		http.Error(w, "Failed to initialize SSE stream", http.StatusInternalServerError)
		return
	}

	// Step 5: Register every authenticated client (even zero-topic ones
	// need BroadcastToAll events for dashboard count refreshes).
	rs.createAndRegisterClient(conn)
	rs.runEventLoop(ctx, conn)
}

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// EventsHandler returns the eventsHandler
func (rs *Resource) EventsHandler() http.HandlerFunc { return rs.eventsHandler }
