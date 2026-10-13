package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth/v5"

	"github.com/moto-nrw/project-phoenix/realtime"
)

// sseConnection holds all state for an active SSE connection
type sseConnection struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	staffID int64
	// accountID is auth.accounts.id. Carried separately from staffID because
	// personal notifications address accounts, and a staff-less effective admin
	// has staffID 0.
	accountID int64
	tenantID  int64
	isAdmin   bool
	client    *realtime.Client
	topics    *sseTopics
	logger    *slog.Logger
}

// sseTopics holds subscription topic information
type sseTopics struct {
	activeGroupIDs []string
	eduTopics      []string
	allTopics      []string
}

// withSSETokenDeadline ensures an accepted stream cannot outlive the access
// token that authorized its subscriptions. The client reconnects through the
// normal authentication path and receives current roles and permissions.
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

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (conn *sseConnection) getLogger() *slog.Logger {
	return loggerOrDefault(conn.logger)
}

// connectedEvent is the initial event sent when SSE connection is established
type connectedEvent struct {
	Status                   string   `json:"status"`
	SupervisedGroupCount     int      `json:"supervisedGroupCount"`
	ActiveGroupIDs           []string `json:"activeGroupIds"`
	EducationalGroupTopics   []string `json:"educationalGroupTopics"`
	SubscribedTopicCount     int      `json:"subscribedTopicCount"`
	SubscribedTopicSnapshots []string `json:"subscribedTopics"`
}

// setupSSEConnection validates the connection and sets up SSE headers
// Returns an error response code if setup fails (caller should return immediately)
func (rs *Resource) setupSSEConnection(w http.ResponseWriter) (*sseConnection, int) {
	// Check if response writer supports flushing (required for SSE)
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, http.StatusInternalServerError
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	return &sseConnection{
		writer:  w,
		flusher: flusher,
		logger:  rs.getLogger(),
	}, 0
}

// sendConnectedEvent sends the initial "connected" event to the client
func (conn *sseConnection) sendConnectedEvent(topics *sseTopics) error {
	event := connectedEvent{
		Status:                   "ready",
		SupervisedGroupCount:     len(topics.activeGroupIDs),
		ActiveGroupIDs:           topics.activeGroupIDs,
		EducationalGroupTopics:   topics.eduTopics,
		SubscribedTopicCount:     len(topics.allTopics),
		SubscribedTopicSnapshots: topics.allTopics,
	}

	data, err := json.Marshal(event)
	if err != nil {
		conn.getLogger().Error("failed to marshal initial SSE event",
			slog.String("error", err.Error()),
			slog.Int64("staff_id", conn.staffID),
		)
		return err
	}

	return conn.writeSSEMessage("connected", data)
}

// writeSSEMessage writes a formatted SSE message to the connection
func (conn *sseConnection) writeSSEMessage(eventType string, data []byte) error {
	if _, err := fmt.Fprintf(conn.writer, "event: %s\n", eventType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(conn.writer, "data: %s\n\n", data); err != nil {
		return err
	}
	conn.flusher.Flush()
	return nil
}

// sendHeartbeat sends a heartbeat comment to keep the connection alive
func (conn *sseConnection) sendHeartbeat() error {
	if _, err := fmt.Fprintf(conn.writer, ": heartbeat\n\n"); err != nil {
		return err
	}
	conn.flusher.Flush()
	return nil
}

// createAndRegisterClient creates the SSE client and registers it with the hub
func (rs *Resource) createAndRegisterClient(conn *sseConnection) {
	conn.client = &realtime.Client{
		Channel:          make(chan realtime.Event, 32), // Buffer up to 32 events (issue #848: headroom for bursts, e.g. admin-overview clients subscribed to every group)
		UserID:           conn.staffID,
		AccountID:        conn.accountID,
		TenantID:         conn.tenantID,
		IsAdmin:          conn.isAdmin,
		SubscribedGroups: make(map[string]bool),
	}
	rs.hub.Register(conn.client, conn.tenantID, conn.topics.allTopics)
}

// runEventLoop runs the main SSE event streaming loop
func (rs *Resource) runEventLoop(ctx context.Context, conn *sseConnection) {
	defer rs.hub.Unregister(conn.client)

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case event := <-conn.client.Channel:
			if ctx.Err() != nil {
				return
			}
			if conn.sendEvent(event) != nil {
				return // Client disconnected
			}

		case <-heartbeat.C:
			if ctx.Err() != nil {
				return
			}
			if conn.sendHeartbeat() != nil {
				return // Client disconnected
			}
		}
	}
}

// sendEvent marshals and sends a single SSE event
func (conn *sseConnection) sendEvent(event realtime.Event) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		conn.getLogger().Error("failed to marshal SSE event",
			slog.String("error", err.Error()),
			slog.Int64("staff_id", conn.staffID),
			slog.String("event_type", string(event.Type)),
		)
		return nil // Don't disconnect on marshal error, just skip this event
	}

	return conn.writeSSEMessage(string(event.Type), eventData)
}

// Registered class codes (error-registry.json) of the stream's refusals.
// This package may not import api/common, so it names them once here.
const (
	codeGeneralPermission  = "general.permission"
	codeGeneralUnavailable = "general.unavailable"
	codeGeneralServer      = "general.server"
)

// sseProblem is the shared error envelope (ADR 0006) of a stream that could
// not start: the HTTP status number, the diagnostic text and the registered
// code. ProblemResponseMiddleware adds type, title and instance like for
// every error answer.
type sseProblem struct {
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

// writeSSEProblem refuses a stream before its first event. It replaces the
// event-stream content type the connection setup may already have set.
func writeSSEProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Del("Cache-Control")
	w.Header().Del("Connection")
	w.Header().Del("X-Accel-Buffering")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(sseProblem{Status: status, Detail: detail, Code: code}); err != nil {
		slog.Default().Error("failed to write SSE refusal", slog.String("error", err.Error()))
	}
}

// setupProblemCode is the registered code of a subscription refusal: 401 and
// 403 are permission refusals, anything else a server fault.
func setupProblemCode(status int) string {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return codeGeneralPermission
	}
	return codeGeneralServer
}
