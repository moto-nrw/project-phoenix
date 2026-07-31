package analytics

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNoopIsSafe(t *testing.T) {
	tracker := NewNoop()
	tracker.Capture("school:x", "some_event", map[string]any{"key": "value"})
	require.NoError(t, tracker.Close())
}

func TestNewWithoutAPIKeyReturnsNoop(t *testing.T) {
	tracker, err := New("", "", nil)
	require.NoError(t, err)
	require.IsType(t, noopTracker{}, tracker)
}

func TestNewWithAPIKeyButNoHostFails(t *testing.T) {
	tracker, err := New("phc_test", "", nil)
	require.Error(t, err)
	require.Nil(t, tracker)
}

func TestNewWithInvalidHostFails(t *testing.T) {
	for _, host := range []string{"not a url", "eu.i.posthog.com", "://missing-scheme"} {
		tracker, err := New("phc_test", host, nil)
		require.Error(t, err, "host %q should be rejected", host)
		require.Nil(t, tracker)
	}
}

func TestNewWithAPIKeyAndHost(t *testing.T) {
	tracker, err := New("phc_test", "https://eu.i.posthog.com", nil)
	require.NoError(t, err)
	require.NotNil(t, tracker)
	require.NoError(t, tracker.Close())
}

// recordingServer captures every /batch/ request body the tracker sends.
type recordingServer struct {
	mu     sync.Mutex
	bodies [][]byte
	status int
}

func (rs *recordingServer) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rs.mu.Lock()
		rs.bodies = append(rs.bodies, body)
		rs.mu.Unlock()
		w.WriteHeader(rs.status)
	}
}

func (rs *recordingServer) requests() [][]byte {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([][]byte(nil), rs.bodies...)
}

func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

func TestCapturePostsBatchPayload(t *testing.T) {
	rs := &recordingServer{status: http.StatusOK}
	server := httptest.NewServer(rs.handler())
	defer server.Close()

	logger, logs := newTestLogger()
	tracker, err := New("phc_test", server.URL, logger)
	require.NoError(t, err)

	tracker.Capture("school:42", "student_checked_in", map[string]any{"method": "rfid"})
	require.NoError(t, tracker.Close()) // waits for the in-flight send

	requests := rs.requests()
	require.Len(t, requests, 1)

	var payload struct {
		APIKey string `json:"api_key"`
		Batch  []struct {
			Event      string         `json:"event"`
			DistinctID string         `json:"distinct_id"`
			Timestamp  string         `json:"timestamp"`
			Properties map[string]any `json:"properties"`
		} `json:"batch"`
	}
	require.NoError(t, json.Unmarshal(requests[0], &payload))
	assert.Equal(t, "phc_test", payload.APIKey)
	require.Len(t, payload.Batch, 1)
	assert.Equal(t, "student_checked_in", payload.Batch[0].Event)
	assert.Equal(t, "school:42", payload.Batch[0].DistinctID)
	assert.NotEmpty(t, payload.Batch[0].Timestamp)
	assert.Equal(t, "rfid", payload.Batch[0].Properties["method"])
	assert.Equal(t, "phoenix-backend", payload.Batch[0].Properties["$lib"])
	assert.Equal(t, true, payload.Batch[0].Properties["$geoip_disable"])
	assert.Equal(t, false, payload.Batch[0].Properties["$process_person_profile"])
	assert.Empty(t, logs.String())
}

func TestCaptureDoesNotMutateCallerProps(t *testing.T) {
	rs := &recordingServer{status: http.StatusOK}
	server := httptest.NewServer(rs.handler())
	defer server.Close()

	logger, _ := newTestLogger()
	tracker, err := New("phc_test", server.URL, logger)
	require.NoError(t, err)

	props := map[string]any{"method": "manual"}
	tracker.Capture("school:1", "student_checked_out", props)
	require.NoError(t, tracker.Close())

	assert.Equal(t, map[string]any{"method": "manual"}, props)
}

func TestCaptureWithNilPropsSucceeds(t *testing.T) {
	rs := &recordingServer{status: http.StatusOK}
	server := httptest.NewServer(rs.handler())
	defer server.Close()

	logger, logs := newTestLogger()
	tracker, err := New("phc_test", server.URL, logger)
	require.NoError(t, err)

	tracker.Capture("school:1", "room_transfer", nil)
	require.NoError(t, tracker.Close())

	require.Len(t, rs.requests(), 1)
	assert.Empty(t, logs.String())
}

func TestCaptureLogsWarningOnServerError(t *testing.T) {
	rs := &recordingServer{status: http.StatusInternalServerError}
	server := httptest.NewServer(rs.handler())
	defer server.Close()

	logger, logs := newTestLogger()
	tracker, err := New("phc_test", server.URL, logger)
	require.NoError(t, err)

	tracker.Capture("school:1", "some_event", nil)
	require.NoError(t, tracker.Close())

	assert.Contains(t, logs.String(), "posthog capture failed")
	assert.Contains(t, logs.String(), "status 500")
}

func TestCaptureLogsWarningOnNetworkError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close() // connection refused from here on

	logger, logs := newTestLogger()
	tracker, err := New("phc_test", server.URL, logger)
	require.NoError(t, err)

	tracker.Capture("school:1", "some_event", nil)
	require.NoError(t, tracker.Close())

	assert.Contains(t, logs.String(), "posthog capture failed")
}

func TestCaptureLogsWarningOnUnmarshalableProps(t *testing.T) {
	rs := &recordingServer{status: http.StatusOK}
	server := httptest.NewServer(rs.handler())
	defer server.Close()

	logger, logs := newTestLogger()
	tracker, err := New("phc_test", server.URL, logger)
	require.NoError(t, err)

	tracker.Capture("school:1", "bad_event", map[string]any{"ch": make(chan int)})
	require.NoError(t, tracker.Close())

	assert.Empty(t, rs.requests())
	assert.Contains(t, logs.String(), "posthog capture failed")
}

func TestNilLoggerFallsBackToDefault(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()

	tracker, err := New("phc_test", server.URL, nil)
	require.NoError(t, err)

	// Must not panic despite the nil logger and the failing send.
	tracker.Capture("school:1", "some_event", nil)
	require.NoError(t, tracker.Close())
}
