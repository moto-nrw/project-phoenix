package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDeployment = "moto-app.de"
	testSessionID  = "0199a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b"
)

func TestNewNoopIsSafe(t *testing.T) {
	t.Parallel()

	tracker := NewNoop()
	tracker.Capture("school:x", "some_event", map[string]any{"key": "value"})
	tracker.CaptureContext(context.Background(), "school:x", "some_event", nil)
	require.NoError(t, tracker.Close())
}

func TestNewWithoutAPIKeyReturnsNoop(t *testing.T) {
	t.Parallel()

	tracker, err := New("", "", "", nil)
	require.NoError(t, err)
	require.IsType(t, noopTracker{}, tracker)
}

func TestNewWithAPIKeyButNoHostFails(t *testing.T) {
	t.Parallel()

	tracker, err := New("phc_test", "", testDeployment, nil)
	require.Error(t, err)
	require.Nil(t, tracker)
}

func TestNewWithInvalidHostFails(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"not a url", "eu.i.posthog.com", "://missing-scheme"} {
		tracker, err := New("phc_test", host, testDeployment, nil)
		require.Error(t, err, "host %q should be rejected", host)
		require.Nil(t, tracker)
	}
}

// Without a deployment, demo and school events would mix in the one project.
func TestNewWithAPIKeyButNoDeploymentFails(t *testing.T) {
	t.Parallel()

	for _, deployment := range []string{"", "   "} {
		tracker, err := New("phc_test", "https://eu.i.posthog.com", deployment, nil)
		require.Error(t, err)
		require.Nil(t, tracker)
	}
}

func TestNewWithAPIKeyAndHost(t *testing.T) {
	t.Parallel()

	tracker, err := New("phc_test", "https://eu.i.posthog.com", testDeployment, nil)
	require.NoError(t, err)
	require.NotNil(t, tracker)
	require.NoError(t, tracker.Close())
}

type sentEvent struct {
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Timestamp  string         `json:"timestamp"`
	Properties map[string]any `json:"properties"`
}

type sentBatch struct {
	APIKey string      `json:"api_key"`
	Batch  []sentEvent `json:"batch"`
}

// posthogServer stands in for PostHog's /batch/ endpoint and records every
// request the tracker posts.
type posthogServer struct {
	*httptest.Server
	mu      sync.Mutex
	paths   []string
	batches []sentBatch
}

func newPostHogServer(t *testing.T, status int) *posthogServer {
	t.Helper()
	server := &posthogServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			var batch sentBatch
			if json.Unmarshal(body, &batch) == nil {
				server.mu.Lock()
				server.paths = append(server.paths, r.URL.Path)
				server.batches = append(server.batches, batch)
				server.mu.Unlock()
			}
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return server
}

func (s *posthogServer) requests() []sentBatch {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]sentBatch(nil), s.batches...)
}

func (s *posthogServer) events() []sentEvent {
	var events []sentEvent
	for _, batch := range s.requests() {
		events = append(events, batch.Batch...)
	}
	return events
}

// newServerTracker posts to server through the production constructor.
func newServerTracker(t *testing.T, server *posthogServer, logger *slog.Logger) Tracker {
	t.Helper()
	tracker, err := New("phc_test", server.URL, testDeployment, logger)
	require.NoError(t, err)
	return tracker
}

func newTestLogger() (*slog.Logger, *syncBuffer) {
	buf := &syncBuffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

// syncBuffer is a log sink the worker goroutine may write to while the test
// reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestCapturePostsBatchPayload(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	logger, logs := newTestLogger()
	tracker := newServerTracker(t, server, logger)

	tracker.Capture("school:42", "student_checked_in", map[string]any{"method": "rfid"})
	require.NoError(t, tracker.Close()) // sends the queued event

	requests := server.requests()
	require.Len(t, requests, 1)
	assert.Equal(t, []string{"/batch/"}, server.paths)
	assert.Equal(t, "phc_test", requests[0].APIKey)
	require.Len(t, requests[0].Batch, 1)
	event := requests[0].Batch[0]
	assert.Equal(t, "student_checked_in", event.Event)
	assert.Equal(t, "school:42", event.DistinctID)
	assert.NotEmpty(t, event.Timestamp)
	assert.Equal(t, "rfid", event.Properties["method"])
	assert.Equal(t, "phoenix-backend", event.Properties["$lib"])
	assert.Equal(t, testDeployment, event.Properties["deployment"])
	assert.Equal(t, true, event.Properties["$geoip_disable"])
	assert.Equal(t, false, event.Properties["$process_person_profile"])
	assert.NotContains(t, event.Properties, "$session_id", "an event outside a browser request has no session")
	assert.Empty(t, logs.String())
}

func TestCaptureContextLinksTheBrowserSession(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newServerTracker(t, server, nil)

	ctx := WithSessionID(context.Background(), testSessionID)
	tracker.CaptureContext(ctx, "school:7", "group_created", map[string]any{"surface": "ogs"})
	require.NoError(t, tracker.Close())

	events := server.events()
	require.Len(t, events, 1)
	assert.Equal(t, testSessionID, events[0].Properties["$session_id"])
	assert.Equal(t, testDeployment, events[0].Properties["deployment"])
}

// A caller cannot override what the tracker enforces.
func TestCaptureEnforcesDeploymentAndPersonFlags(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newServerTracker(t, server, nil)

	tracker.Capture("school:1", "some_event", map[string]any{
		"deployment":              "other-host.example",
		"$geoip_disable":          false,
		"$process_person_profile": true,
	})
	require.NoError(t, tracker.Close())

	events := server.events()
	require.Len(t, events, 1)
	assert.Equal(t, testDeployment, events[0].Properties["deployment"])
	assert.Equal(t, true, events[0].Properties["$geoip_disable"])
	assert.Equal(t, false, events[0].Properties["$process_person_profile"])
}

func TestWithSessionIDIgnoresAnythingButASessionUUID(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"", "not-a-session", "Max Mustermann", testSessionID + "x", "../../etc"} {
		assert.Empty(t, SessionIDFromContext(WithSessionID(context.Background(), value)), "value %q", value)
	}
	assert.Equal(t, testSessionID, SessionIDFromContext(WithSessionID(context.Background(), testSessionID)))
}

// Events wait in the queue and leave together: one request, not one per event.
func TestCaptureSendsEventsInOneBatch(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newServerTracker(t, server, nil)

	for _, event := range []string{"first", "second", "third"} {
		tracker.Capture("school:1", event, nil)
	}
	require.NoError(t, tracker.Close())

	requests := server.requests()
	require.Len(t, requests, 1)
	require.Len(t, requests[0].Batch, 3)
	assert.Equal(t, "first", requests[0].Batch[0].Event)
	assert.Equal(t, "third", requests[0].Batch[2].Event)
}

// A full batch leaves at once instead of waiting for the interval.
func TestCaptureSendsAFullBatchBeforeTheInterval(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newHTTPTracker(server.URL+"/batch/", "phc_test", testDeployment, httpSender{client: server.Client()}, nil, time.Hour)
	t.Cleanup(func() { _ = tracker.Close() })

	for range batchSize {
		tracker.Capture("school:1", "some_event", nil)
	}

	require.Eventually(t, func() bool { return len(server.requests()) == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Len(t, server.requests()[0].Batch, batchSize)
}

// Without Close, the interval sends what is queued.
func TestCaptureFlushesAfterTheInterval(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newHTTPTracker(server.URL+"/batch/", "phc_test", testDeployment, httpSender{client: server.Client()}, nil, 20*time.Millisecond)
	t.Cleanup(func() { _ = tracker.Close() })

	tracker.Capture("school:1", "some_event", nil)

	require.Eventually(t, func() bool { return len(server.events()) == 1 }, 5*time.Second, 10*time.Millisecond)
}

// Shutdown sends the queue; an event captured after Close is dropped
// without a panic.
func TestCloseFlushesAndLaterCapturesAreDropped(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newServerTracker(t, server, nil)

	tracker.Capture("school:1", "before_close", nil)
	require.NoError(t, tracker.Close())
	tracker.Capture("school:1", "after_close", nil)
	require.NoError(t, tracker.Close(), "Close is idempotent")

	events := server.events()
	require.Len(t, events, 1)
	assert.Equal(t, "before_close", events[0].Event)
}

func TestCaptureDoesNotMutateCallerProps(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newServerTracker(t, server, nil)

	props := map[string]any{"method": "manual"}
	tracker.Capture("school:1", "student_checked_out", props)
	require.NoError(t, tracker.Close())

	assert.Equal(t, map[string]any{"method": "manual"}, props)
}

func TestCaptureWithNilPropsSucceeds(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	logger, logs := newTestLogger()
	tracker := newServerTracker(t, server, logger)

	tracker.Capture("school:1", "room_transfer", nil)
	require.NoError(t, tracker.Close())

	require.Len(t, server.events(), 1)
	assert.Empty(t, logs.String())
}

func TestCaptureLogsWarningOnServerError(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusInternalServerError)
	logger, logs := newTestLogger()
	tracker := newServerTracker(t, server, logger)

	tracker.Capture("school:1", "some_event", nil)
	require.NoError(t, tracker.Close())

	assert.Contains(t, logs.String(), "posthog capture failed")
	assert.Contains(t, logs.String(), "status 500")
}

func TestCaptureLogsWarningOnNetworkError(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	server.Close() // connection refused from now on
	logger, logs := newTestLogger()
	tracker := newServerTracker(t, server, logger)

	tracker.Capture("school:1", "some_event", nil)
	require.NoError(t, tracker.Close())

	assert.Contains(t, logs.String(), "posthog capture failed")
}

// One bad event is dropped on its own and does not take its batch along.
func TestCaptureLogsWarningOnUnmarshalableProps(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	logger, logs := newTestLogger()
	tracker := newServerTracker(t, server, logger)

	tracker.Capture("school:1", "bad_event", map[string]any{"ch": make(chan int)})
	tracker.Capture("school:1", "good_event", nil)
	require.NoError(t, tracker.Close())

	events := server.events()
	require.Len(t, events, 1)
	assert.Equal(t, "good_event", events[0].Event)
	assert.Contains(t, logs.String(), "posthog capture failed")
}

func TestNilLoggerFallsBackToDefault(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	server.Close()
	tracker := newServerTracker(t, server, nil)

	// Must not panic despite the nil logger and the failing send.
	tracker.Capture("school:1", "some_event", nil)
	require.NoError(t, tracker.Close())
}
