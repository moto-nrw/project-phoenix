package sse

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/realtime"
)

// sseConnection holds all state for an active SSE connection
type sseConnection struct {
	writer   http.ResponseWriter
	flusher  http.Flusher
	staffID  int64
	tenantID int64
	isAdmin  bool
	client   *realtime.Client
	topics   *sseTopics
	logger   *slog.Logger
}

// sseTopics holds subscription topic information
type sseTopics struct {
	activeGroupIDs []string
	eduTopics      []string
	allTopics      []string
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (conn *sseConnection) getLogger() *slog.Logger {
	return cmp.Or(conn.logger, slog.Default())
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
			if conn.sendEvent(event) != nil {
				return // Client disconnected
			}

		case <-heartbeat.C:
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
