package sse

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	mf := newMockFlusher()
	conn := &sseConnection{
		writer:  mf,
		flusher: mf,
		staffID: 300,
	}

	event := realtime.Event{
		Type: realtime.EventStudentCheckIn,
		Data: realtime.EventData{
			StudentID:   ptr("123"),
			StudentName: ptr("Test Student"),
		},
	}

	err := conn.sendEvent(event)
	require.NoError(t, err)

	body := mf.Body.String()
	assert.Contains(t, body, "event: student_checkin\n")
	assert.Contains(t, body, "Test Student")
}

// =============================================================================
// SETUP CONNECTION TESTS
// =============================================================================

func TestSetupSSEConnection_Success(t *testing.T) {
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
	rs := &Resource{}

	// Regular ResponseRecorder doesn't implement http.Flusher interface directly
	// when accessed as http.ResponseWriter - only mockFlusher adds it
	w := &nonFlusherResponseWriter{}
	conn, statusCode := rs.setupSSEConnection(w)

	assert.Nil(t, conn, "Connection should be nil for non-flusher")
	assert.Equal(t, http.StatusInternalServerError, statusCode)
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
// LOGGING HELPER TESTS
// =============================================================================

func TestLogError_NilLogger(t *testing.T) {
	// With nil logger, should not panic
	assert.NotPanics(t, func() {
		logError("test error", assert.AnError, 123)
	})
}

func TestLogWarning_NilLogger(t *testing.T) {
	// With nil logger, should not panic
	assert.NotPanics(t, func() {
		logWarning("test warning", assert.AnError, 456)
	})
}

func TestLogInfo_NilLogger(t *testing.T) {
	// With nil logger, should not panic
	assert.NotPanics(t, func() {
		logInfo("test info", 789)
	})
}

func TestLogEventError_NilLogger(t *testing.T) {
	// With nil logger, should not panic
	assert.NotPanics(t, func() {
		logEventError("test event error", assert.AnError, 100, realtime.EventStudentCheckIn)
	})
}

// =============================================================================
// RESOURCE TESTS
// =============================================================================

func TestNewResource(t *testing.T) {
	hub := realtime.NewHub()

	// Test with nil services (should not panic)
	resource := NewResource(hub, nil, nil, nil, nil)
	assert.NotNil(t, resource)
	assert.Equal(t, hub, resource.hub)
}

func TestResource_Router(t *testing.T) {
	hub := realtime.NewHub()
	resource := NewResource(hub, nil, nil, nil, nil)

	router := resource.Router()
	assert.NotNil(t, router)
}

func TestResource_EventsHandler(t *testing.T) {
	hub := realtime.NewHub()
	resource := NewResource(hub, nil, nil, nil, nil)

	handler := resource.EventsHandler()
	assert.NotNil(t, handler)
}

// Helper function
func ptr(s string) *string {
	return &s
}
