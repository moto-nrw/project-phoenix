package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/realtime"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// sseConnection holds all state for an active SSE connection
type sseConnection struct {
	writer  http.ResponseWriter
	flusher http.Flusher
	staffID int64
	client  *realtime.Client
	topics  *sseTopics
}

// sseTopics holds subscription topic information
type sseTopics struct {
	activeGroupIDs []string
	eduTopics      []string
	allTopics      []string
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
	}, 0
}

// resolveStaff extracts JWT claims and resolves the staff member
// Returns error message and HTTP status code on failure
func (rs *Resource) resolveStaff(ctx context.Context) (*users.Staff, string, int) {
	claims := jwt.ClaimsFromCtx(ctx)

	// Get person from account ID
	person, err := rs.personSvc.FindByAccountID(ctx, int64(claims.ID))
	if err != nil || person == nil {
		return nil, "Account not found", http.StatusUnauthorized
	}

	// Get staff from person ID
	staff, err := rs.personSvc.GetStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		return nil, "User is not a staff member", http.StatusForbidden
	}

	return staff, "", 0
}

// buildSubscriptionTopics builds the list of topics to subscribe to
func (rs *Resource) buildSubscriptionTopics(ctx context.Context, staffID int64) (*sseTopics, error) {
	// Get supervised active groups for this staff member
	supervisions, err := rs.activeSvc.GetStaffActiveSupervisions(ctx, staffID)
	if err != nil {
		logError("Failed to get staff active supervisions for SSE", err, staffID)
		return nil, err
	}

	// Prepare subscription topics (active groups + derived educational groups)
	activeGroupIDs := make([]string, 0, len(supervisions))
	eduTopics := make([]string, 0)
	allTopics := make([]string, 0)
	topicSet := make(map[string]struct{})

	addTopic := func(topic string) {
		if topic == "" {
			return
		}
		if _, exists := topicSet[topic]; exists {
			return
		}
		topicSet[topic] = struct{}{}
		allTopics = append(allTopics, topic)
	}

	for _, supervision := range supervisions {
		groupTopic := strconv.FormatInt(supervision.GroupID, 10)
		activeGroupIDs = append(activeGroupIDs, groupTopic)
		addTopic(groupTopic)
	}

	// Load educational groups if usercontext service is available
	if rs.userCtx != nil {
		eduGroups, err := rs.userCtx.GetMyGroups(ctx)
		if err != nil {
			logWarning("Failed to load educational groups for SSE subscription", err, staffID)
		} else {
			eduTopics = make([]string, 0, len(eduGroups))
			for _, group := range eduGroups {
				topic := fmt.Sprintf("edu:%d", group.ID)
				eduTopics = append(eduTopics, topic)
				addTopic(topic)
			}
		}
	}

	return &sseTopics{
		activeGroupIDs: activeGroupIDs,
		eduTopics:      eduTopics,
		allTopics:      allTopics,
	}, nil
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
		logError("Failed to marshal initial SSE event", err, conn.staffID)
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

// runHeartbeatOnlyLoop runs the event loop when there are no topics to subscribe to
func (conn *sseConnection) runHeartbeatOnlyLoop(ctx context.Context) {
	logInfo("SSE connection - no available topics (heartbeat only)", conn.staffID)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if conn.sendHeartbeat() != nil {
				return // Client disconnected
			}
		}
	}
}

// createAndRegisterClient creates the SSE client and registers it with the hub
func (rs *Resource) createAndRegisterClient(conn *sseConnection) {
	conn.client = &realtime.Client{
		Channel:          make(chan port.Event, 10), // Buffer up to 10 events
		UserID:           conn.staffID,
		SubscribedGroups: make(map[string]bool),
	}
	rs.hub.Register(conn.client, conn.topics.allTopics)
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
func (conn *sseConnection) sendEvent(event port.Event) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		logEventError("Failed to marshal SSE event", err, conn.staffID, event.Type)
		return nil // Don't disconnect on marshal error, just skip this event
	}

	return conn.writeSSEMessage(string(event.Type), eventData)
}

// Logging helpers with defensive nil checks

func logError(msg string, err error, staffID int64) {
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]any{
			"error":    err.Error(),
			"staff_id": staffID,
		}).Error(msg)
	}
}

func logWarning(msg string, err error, staffID int64) {
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]any{
			"error":    err.Error(),
			"staff_id": staffID,
		}).Warn(msg)
	}
}

func logInfo(msg string, staffID int64) {
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]any{
			"staff_id": staffID,
		}).Info(msg)
	}
}

func logEventError(msg string, err error, staffID int64, eventType port.EventType) {
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]any{
			"error":      err.Error(),
			"staff_id":   staffID,
			"event_type": string(eventType),
		}).Error(msg)
	}
}
