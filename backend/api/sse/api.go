package sse

import (
	"context"
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
	tokenAuth, _ := jwt.NewTokenAuth()

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

	// Step 3: Resolve staff and build topics inside a short-lived tenant transaction.
	// SSE is a long-lived streaming connection, so we cannot wrap the entire handler
	// in TenantTxMiddleware (that would hold a DB connection open for the stream's lifetime).
	// Instead we open a scoped transaction for the setup queries only.
	var staff *users.Staff
	var topics *sseTopics
	if conn.tenantID > 0 {
		err := tenant.WithTenantTx(ctx, rs.db, conn.tenantID, func(txCtx context.Context, _ bun.Tx) error {
			var errMsg string
			var statusCode int
			staff, errMsg, statusCode = rs.resolveStaff(txCtx)
			if staff == nil {
				http.Error(w, errMsg, statusCode)
				return nil
			}

			var topicErr error
			topics, topicErr = rs.buildSubscriptionTopics(txCtx, staff.ID)
			return topicErr
		})
		if err != nil {
			http.Error(w, "Failed to initialize SSE connection", http.StatusInternalServerError)
			return
		}
	} else {
		var errMsg string
		var statusCode int
		staff, errMsg, statusCode = rs.resolveStaff(ctx)
		if staff == nil {
			http.Error(w, errMsg, statusCode)
			return
		}
		var err error
		topics, err = rs.buildSubscriptionTopics(ctx, staff.ID)
		if err != nil {
			http.Error(w, "Failed to determine supervised groups", http.StatusInternalServerError)
			return
		}
	}
	if staff == nil {
		// Error response already sent inside the tx callback
		return
	}
	conn.staffID = staff.ID
	conn.topics = topics

	// Step 4: Send initial "connected" event
	if conn.sendConnectedEvent(topics) != nil {
		http.Error(w, "Failed to initialize SSE stream", http.StatusInternalServerError)
		return
	}

	// Step 5: Run appropriate event loop based on subscription state
	if len(topics.allTopics) == 0 {
		conn.runHeartbeatOnlyLoop(ctx)
		return
	}

	// Step 6: Register client and run main event loop
	rs.createAndRegisterClient(conn)
	rs.runEventLoop(ctx, conn)
}

// =============================================================================
// HANDLER ACCESSOR METHODS (for testing)
// =============================================================================

// EventsHandler returns the eventsHandler
func (rs *Resource) EventsHandler() http.HandlerFunc { return rs.eventsHandler }
