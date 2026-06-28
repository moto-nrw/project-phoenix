package sse

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// ParentRouter returns the SSE router for the parents portal. It mirrors the
// staff Router() but authenticates with ParentMiddleware (scope=parent only)
// and subscribes the connection to the tenants of the guardian's children
// rather than to supervised groups. Mounted at /parent-sse.
func (rs *Resource) ParentRouter() chi.Router {
	r := chi.NewRouter()

	tokenAuth := jwt.MustNewTokenAuth()
	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth.JwtAuth))
		r.Use(jwt.Authenticator)
		r.Use(jwt.ParentMiddleware)

		r.Get("/events", rs.parentEventsHandler)
	})

	return r
}

// parentEventsHandler streams parent_message and parent_message_read triggers to
// a guardian. The connection is registered against the guardian's own account, so
// it is woken only for its own threads — never another family's, and never by
// staff-oriented tenant/group broadcasts.
func (rs *Resource) parentEventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	conn, statusCode := rs.setupSSEConnection(w)
	if conn == nil {
		slog.WarnContext(ctx, "parent SSE streaming unsupported by client")
		http.Error(w, "Streaming unsupported", statusCode)
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(ctx).ID)
	if accountID <= 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	conn.staffID = accountID // reused as the connection's user id for logging

	if err := conn.sendConnectedEvent(&sseTopics{}); err != nil {
		slog.ErrorContext(ctx, "parent SSE failed to send connected event",
			slog.String("error", err.Error()))
		http.Error(w, "Failed to initialize SSE stream", http.StatusInternalServerError)
		return
	}

	conn.client = &realtime.Client{
		Channel:          make(chan realtime.Event, 32),
		UserID:           accountID,
		SubscribedGroups: make(map[string]bool),
	}
	rs.hub.RegisterParent(conn.client)
	rs.runEventLoop(ctx, conn)
}
