package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/jwtauth/v5"
	jwxjwt "github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFlusher wraps a ResponseRecorder and implements http.Flusher
type mockFlusher struct {
	*httptest.ResponseRecorder
	flushCount int
}

func (mf *mockFlusher) Flush() {
	mf.flushCount++
	// ResponseRecorder doesn't need actual flush
}

func newMockFlusher() *mockFlusher {
	return &mockFlusher{
		ResponseRecorder: httptest.NewRecorder(),
	}
}

// =============================================================================
// SSE CONNECTION TESTS
// =============================================================================

func TestSSEConnection_WriteSSEMessage(t *testing.T) {
	t.Parallel()

	mf := newMockFlusher()
	conn := &sseConnection{
		writer:  mf,
		flusher: mf,
		staffID: 123,
	}

	err := conn.writeSSEMessage("test-event", []byte(`{"message":"hello"}`))
	require.NoError(t, err)

	body := mf.Body.String()
	assert.Contains(t, body, "event: test-event\n")
	assert.Contains(t, body, `data: {"message":"hello"}`)
	assert.Equal(t, 1, mf.flushCount, "Should have flushed once")
}

func TestSSEConnection_WriteSSEMessage_EmptyData(t *testing.T) {
	t.Parallel()

	mf := newMockFlusher()
	conn := &sseConnection{
		writer:  mf,
		flusher: mf,
		staffID: 456,
	}

	err := conn.writeSSEMessage("empty-event", []byte{})
	require.NoError(t, err)

	body := mf.Body.String()
	assert.Contains(t, body, "event: empty-event\n")
	assert.Contains(t, body, "data: \n")
}

func TestSSEConnection_SendHeartbeat(t *testing.T) {
	t.Parallel()

	mf := newMockFlusher()
	conn := &sseConnection{
		writer:  mf,
		flusher: mf,
		staffID: 789,
	}

	err := conn.sendHeartbeat()
	require.NoError(t, err)

	body := mf.Body.String()
	assert.Contains(t, body, ": heartbeat\n\n")
	assert.Equal(t, 1, mf.flushCount, "Should have flushed once")
}

func TestSSEConnection_SendConnectedEvent(t *testing.T) {
	t.Parallel()

	mf := newMockFlusher()
	conn := &sseConnection{
		writer:  mf,
		flusher: mf,
		staffID: 100,
	}

	topics := &sseTopics{
		activeGroupIDs: []string{"1", "2"},
		eduTopics:      []string{"edu:10", "edu:20"},
		allTopics:      []string{"1", "2", "edu:10", "edu:20"},
	}

	err := conn.sendConnectedEvent(topics)
	require.NoError(t, err)

	body := mf.Body.String()
	assert.Contains(t, body, "event: connected\n")

	// Verify the JSON data
	var event connectedEvent
	dataStart := bytes.Index([]byte(body), []byte("data: ")) + 6
	dataEnd := bytes.Index([]byte(body[dataStart:]), []byte("\n\n")) + dataStart
	err = json.Unmarshal([]byte(body[dataStart:dataEnd]), &event)
	require.NoError(t, err)

	assert.Equal(t, "ready", event.Status)
	assert.Equal(t, 2, event.SupervisedGroupCount)
	assert.Equal(t, []string{"1", "2"}, event.ActiveGroupIDs)
	assert.Equal(t, []string{"edu:10", "edu:20"}, event.EducationalGroupTopics)
	assert.Equal(t, 4, event.SubscribedTopicCount)
}

func TestSSEConnection_SendConnectedEvent_EmptyTopics(t *testing.T) {
	t.Parallel()

	mf := newMockFlusher()
	conn := &sseConnection{
		writer:  mf,
		flusher: mf,
		staffID: 200,
	}

	topics := &sseTopics{
		activeGroupIDs: []string{},
		eduTopics:      []string{},
		allTopics:      []string{},
	}

	err := conn.sendConnectedEvent(topics)
	require.NoError(t, err)

	body := mf.Body.String()
	assert.Contains(t, body, "event: connected\n")
	assert.Contains(t, body, `"status":"ready"`)
}

func TestSSEConnection_SendEvent(t *testing.T) {
	t.Parallel()

	mf := newMockFlusher()
	conn := &sseConnection{
		writer:  mf,
		flusher: mf,
		staffID: 300,
	}

	event := realtime.Event{
		Type: realtime.EventStudentCheckIn,
		Data: realtime.EventData{
			StudentID: ptr("123"),
		},
	}

	err := conn.sendEvent(event)
	require.NoError(t, err)

	body := mf.Body.String()
	assert.Contains(t, body, "event: student_checkin\n")
	assert.Contains(t, body, `"student_id":"123"`)
}

func TestRunEventLoopDoesNotSendBufferedEventAfterDeadline(t *testing.T) {
	t.Parallel()

	for range 32 {
		hub := realtime.NewHub(slog.Default())
		rs := &Resource{hub: hub}
		mf := newMockFlusher()
		client := &realtime.Client{
			Channel:          make(chan realtime.Event, 1),
			SubscribedGroups: make(map[string]bool),
		}
		conn := &sseConnection{
			writer:  mf,
			flusher: mf,
			client:  client,
		}
		hub.Register(client, 1, nil)
		client.Channel <- realtime.Event{Type: realtime.EventStudentCheckIn}

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		rs.runEventLoop(ctx, conn)
		cancel()

		assert.Empty(t, mf.Body.String())
		assert.Zero(t, mf.flushCount)
	}
}

// =============================================================================
// SETUP CONNECTION TESTS
// =============================================================================

func TestSetupSSEConnection_Success(t *testing.T) {
	t.Parallel()

	rs := &Resource{}

	// Create a mock ResponseWriter that implements http.Flusher
	mf := newMockFlusher()
	conn, statusCode := rs.setupSSEConnection(mf)

	assert.NotNil(t, conn, "Connection should be created")
	assert.Equal(t, 0, statusCode, "Status code should be 0 for success")

	// Verify headers
	assert.Equal(t, "text/event-stream", mf.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", mf.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", mf.Header().Get("Connection"))
	assert.Equal(t, "no", mf.Header().Get("X-Accel-Buffering"))
}

func TestSetupSSEConnection_NonFlusher(t *testing.T) {
	t.Parallel()

	rs := &Resource{}

	// Regular ResponseRecorder doesn't implement http.Flusher interface directly
	// when accessed as http.ResponseWriter - only mockFlusher adds it
	w := &nonFlusherResponseWriter{}
	conn, statusCode := rs.setupSSEConnection(w)

	assert.Nil(t, conn, "Connection should be nil for non-flusher")
	assert.Equal(t, http.StatusInternalServerError, statusCode)
}

func TestWithSSETokenDeadlineAppliesAccessTokenExpiry(t *testing.T) {
	t.Parallel()

	expiresAt := time.Now().Add(time.Hour)
	token, err := jwxjwt.NewBuilder().Expiration(expiresAt).Build()
	require.NoError(t, err)

	ctx := jwtauth.NewContext(context.Background(), token, nil)
	deadlineCtx, cancel, ok := withSSETokenDeadline(ctx)
	require.True(t, ok)
	defer cancel()

	deadline, hasDeadline := deadlineCtx.Deadline()
	require.True(t, hasDeadline)
	assert.WithinDuration(t, expiresAt, deadline, time.Millisecond)
}

func TestWithSSETokenDeadlineRejectsTokenWithoutExpiry(t *testing.T) {
	t.Parallel()

	token, err := jwxjwt.NewBuilder().Build()
	require.NoError(t, err)

	ctx := jwtauth.NewContext(context.Background(), token, nil)
	_, cancel, ok := withSSETokenDeadline(ctx)
	defer cancel()

	assert.False(t, ok)
}

func TestCreateAndRegisterClientPreservesAdminScope(t *testing.T) {
	t.Parallel()

	hub := realtime.NewHub(slog.Default())
	rs := &Resource{hub: hub}
	conn := &sseConnection{
		staffID:  123,
		tenantID: 41,
		isAdmin:  true,
		topics:   &sseTopics{},
	}

	rs.createAndRegisterClient(conn)
	t.Cleanup(func() { hub.Unregister(conn.client) })

	require.NotNil(t, conn.client)
	assert.True(t, conn.client.IsAdmin)
	assert.Equal(t, int64(41), conn.client.TenantID)
}

func TestHasEffectiveAdminScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		isAdmin     bool
		permissions []string
		want        bool
	}{
		{name: "literal admin", isAdmin: true, want: true},
		{name: "admin wildcard", permissions: []string{"admin:*"}, want: true},
		{name: "full wildcard", permissions: []string{"*:*"}, want: true},
		{name: "scoped caregiver", permissions: []string{"users:read"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{IsAdmin: tt.isAdmin})
			ctx = context.WithValue(ctx, jwt.CtxPermissions, tt.permissions)
			assert.Equal(t, tt.want, authorize.HasEffectiveAdminScope(ctx))
		})
	}
}

// nonFlusherResponseWriter is a ResponseWriter that doesn't implement Flusher
type nonFlusherResponseWriter struct{}

func (w *nonFlusherResponseWriter) Header() http.Header {
	return http.Header{}
}

func (w *nonFlusherResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (w *nonFlusherResponseWriter) WriteHeader(int) {}

// =============================================================================
// SSE TOPICS TESTS
// =============================================================================

func TestSSETopics_EmptyState(t *testing.T) {
	t.Parallel()

	topics := &sseTopics{
		activeGroupIDs: []string{},
		eduTopics:      []string{},
		allTopics:      []string{},
	}

	assert.Empty(t, topics.activeGroupIDs)
	assert.Empty(t, topics.eduTopics)
	assert.Empty(t, topics.allTopics)
}

func TestSSETopics_WithData(t *testing.T) {
	t.Parallel()

	topics := &sseTopics{
		activeGroupIDs: []string{"1", "2", "3"},
		eduTopics:      []string{"edu:5", "edu:6"},
		allTopics:      []string{"1", "2", "3", "edu:5", "edu:6"},
	}

	assert.Len(t, topics.activeGroupIDs, 3)
	assert.Len(t, topics.eduTopics, 2)
	assert.Len(t, topics.allTopics, 5)
}

// =============================================================================
// CONNECTED EVENT TESTS
// =============================================================================

func TestConnectedEvent_Marshaling(t *testing.T) {
	t.Parallel()

	event := connectedEvent{
		Status:                   "ready",
		SupervisedGroupCount:     3,
		ActiveGroupIDs:           []string{"1", "2", "3"},
		EducationalGroupTopics:   []string{"edu:10"},
		SubscribedTopicCount:     4,
		SubscribedTopicSnapshots: []string{"1", "2", "3", "edu:10"},
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var unmarshaled connectedEvent
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, event.Status, unmarshaled.Status)
	assert.Equal(t, event.SupervisedGroupCount, unmarshaled.SupervisedGroupCount)
	assert.Equal(t, event.ActiveGroupIDs, unmarshaled.ActiveGroupIDs)
	assert.Equal(t, event.EducationalGroupTopics, unmarshaled.EducationalGroupTopics)
	assert.Equal(t, event.SubscribedTopicCount, unmarshaled.SubscribedTopicCount)
}

// =============================================================================
// RESOURCE TESTS
// =============================================================================

func TestNewResource(t *testing.T) {
	t.Parallel()

	hub := realtime.NewHub(slog.Default())

	// Test with nil services (should not panic)
	resource := NewResource(hub, nil, nil, slog.Default())
	assert.NotNil(t, resource)
	assert.Equal(t, hub, resource.hub)
}

func TestResource_Router(t *testing.T) {
	t.Parallel()

	hub := realtime.NewHub(slog.Default())
	resource := NewResource(hub, nil, nil, slog.Default())

	router := resource.Router()
	assert.NotNil(t, router)
}

func TestResource_EventsHandler(t *testing.T) {
	t.Parallel()

	hub := realtime.NewHub(slog.Default())
	resource := NewResource(hub, nil, nil, slog.Default())

	handler := resource.eventsHandler
	assert.NotNil(t, handler)
}

// Helper function
func ptr(s string) *string {
	return &s
}
