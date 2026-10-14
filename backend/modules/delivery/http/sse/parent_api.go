package sse

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ParentRouter returns the SSE router for the parents portal. It mirrors the
// staff Router() but authenticates with ParentMiddleware (scope=parent only)
// and subscribes the connection to the tenants of the guardian's children
// rather than to supervised groups. Mounted at /parent-sse.
func (rs *Resource) ParentRouter() chi.Router {
	r := chi.NewRouter()

	r.Group(func(r chi.Router) {
		r.Use(sessiontoken.Authenticator)
		r.Use(sessiontoken.ParentMiddleware(tenant.ClaimScope{}))
		r.Get("/events", rs.parentEventsHandler)
	})

	return r
}

// parentEventsHandler streams parent_message and parent_message_read triggers to
// a guardian. The connection is registered against the guardian's own account, so
// it is woken only for its own threads — never another family's, and never by
// staff-oriented tenant/group broadcasts.
func (rs *Resource) parentEventsHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := withSSETokenDeadline(r.Context())
	if !ok {
		writeSSEProblem(w, http.StatusUnauthorized, codeGeneralPermission, "Unauthorized")
		return
	}
	defer cancel()

	conn, statusCode := rs.setupSSEConnection(w)
	if conn == nil {
		slog.WarnContext(ctx, "parent SSE streaming unsupported by client")
		writeSSEProblem(w, statusCode, codeGeneralServer, "Streaming unsupported")
		return
	}

	accountID := int64(sessiontoken.ClaimsFromCtx(ctx).ID)
	if accountID <= 0 {
		writeSSEProblem(w, http.StatusUnauthorized, codeGeneralPermission, "Unauthorized")
		return
	}
	conn.staffID = accountID // reused as the connection's user id for logging

	if err := conn.sendConnectedEvent(&sseTopics{}); err != nil {
		slog.ErrorContext(ctx, "parent SSE failed to send connected event",
			slog.String("error", err.Error()))
		writeSSEProblem(w, http.StatusInternalServerError, codeGeneralServer, "Failed to initialize SSE stream")
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
