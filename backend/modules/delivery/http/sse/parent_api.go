package sse

import (
	"log/slog"
	"net/http"
)

// ParentRouter returns the SSE router for the parents portal. It mirrors the
// staff Router() but authenticates with ParentMiddleware (scope=parent only)
// and subscribes the connection to the tenants of the guardian's children
// rather than to supervised groups. Mounted at /parent-sse.

// ParentEvents streams parent_message and parent_message_read triggers to
// a guardian. The connection is registered against the guardian's own account, so
// it is woken only for its own threads — never another family's, and never by
// staff-oriented tenant/group broadcasts.
func (rs *Resource[E, C]) ParentEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := rs.runtime.TokenDeadline(r.Context())
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

	accountID := rs.runtime.Identity(ctx).AccountID
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

	conn.client = rs.runtime.NewClient(make(chan E, 32), accountID, 0, 0, false, make(map[string]bool))
	rs.hub.RegisterParent(conn.client)
	rs.runEventLoop(ctx, conn)
}
