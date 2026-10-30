package sse

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

// Router returns a configured router for SSE endpoints

// StaffEvents handles Server-Sent Events connections
// Orchestrates: connection setup → subscription resolution → event streaming
func (rs *Resource[E, C]) StaffEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel, ok := rs.runtime.TokenDeadline(r.Context())
	if !ok {
		writeSSEProblem(w, http.StatusUnauthorized, codeGeneralPermission, "Unauthorized")
		return
	}
	defer cancel()

	// Step 1: Setup SSE connection (headers, flusher validation)
	conn, statusCode := rs.setupSSEConnection(w)
	if conn == nil {
		slog.WarnContext(ctx, "SSE streaming unsupported by client")
		writeSSEProblem(w, statusCode, codeGeneralServer, "Streaming unsupported")
		return
	}

	// Step 2: Extract tenant ID from JWT context (set by TenantMiddleware)
	identity := rs.runtime.Identity(ctx)
	conn.tenantID = identity.TenantID
	conn.isAdmin = identity.IsAdmin
	conn.readsLeadership = identity.ReadsLeadershipMessages
	// The login account, needed to address this person individually. Already in
	// the claims, so no extra lookup.
	conn.accountID = identity.AccountID

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
		writeSSEProblem(w, http.StatusInternalServerError, codeGeneralServer, "Failed to initialize SSE stream")
		return
	}

	// Step 5: Register every authenticated client (even zero-topic ones
	// need BroadcastToAll events for dashboard count refreshes).
	rs.createAndRegisterClient(conn)
	rs.runEventLoop(ctx, conn)
}

// resolveSSESubscription resolves the client's topic subscription inside a
// tenant transaction. Resolution requires it because RLS on users.persons and
// users.staff needs app.current_tenant_id to be set — the SSE router uses
// sessiontoken.TenantMiddleware (context only), not the DB-level TenantTxMiddleware.
func (rs *Resource[E, C]) resolveSSESubscription(ctx context.Context, tenantID int64) (*sseTopics, int64, error) {
	var staffID int64
	topics := &sseTopics{}
	err := rs.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		var resolveErr error
		staffID, topics.activeGroupIDs, topics.eduTopics, topics.allTopics, resolveErr = rs.userCtx.ResolveSSETopics(txCtx)
		return resolveErr
	})
	if err != nil {
		return nil, 0, err
	}
	return topics, staffID, nil
}

// writeSSESetupError maps a subscription-resolution error to the SSE HTTP
// response: the carried status for an *SSESetupError (401/403), 500 otherwise.
func (rs *Resource[E, C]) writeSSESetupError(w http.ResponseWriter, ctx context.Context, err error) {
	var setupErr SetupRejection
	if errors.As(err, &setupErr) {
		slog.WarnContext(ctx, "SSE setup failed", slog.String("error", setupErr.SetupMessage()))
		writeSSEProblem(w, setupErr.SetupStatus(), setupProblemCode(setupErr.SetupStatus()), setupErr.SetupMessage())
		return
	}
	slog.ErrorContext(ctx, "SSE failed to determine supervised groups", slog.String("error", err.Error()))
	writeSSEProblem(w, http.StatusInternalServerError, codeGeneralServer, "Failed to determine supervised groups")
}
