package sse

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
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

	// Step 2: Resolve staff member from JWT claims
	staff, errMsg, statusCode := rs.resolveStaff(ctx)
	if staff == nil {
		http.Error(w, errMsg, statusCode)
		return
	}
	conn.staffID = staff.ID

	// Step 3: Build subscription topics (active groups + educational groups)
	topics, err := rs.buildSubscriptionTopics(ctx, staff.ID)
	if err != nil {
		http.Error(w, "Failed to determine supervised groups", http.StatusInternalServerError)
		return
	}
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
