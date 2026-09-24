package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Issue #3645: the kiosks' Sentry tunnel, tested at the production router
// with a real device key, a fake Sentry, and a Sentry client that records
// what the backend itself would report.

const relayTestProjectID = "4242"

// fakeSentry records every envelope posted to it.
type fakeSentry struct {
	*httptest.Server
	mu       sync.Mutex
	requests []*http.Request
	bodies   [][]byte
}

func newFakeSentry(t *testing.T) *fakeSentry {
	t.Helper()
	fake := &fakeSentry{}
	fake.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fake.mu.Lock()
		fake.requests = append(fake.requests, r)
		fake.bodies = append(fake.bodies, body)
		fake.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"9ec79c33ec9942ab8353589fcb2e04dc"}`))
	}))
	t.Cleanup(fake.Close)
	return fake
}

func (f *fakeSentry) received() ([]*http.Request, [][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*http.Request(nil), f.requests...), append([][]byte(nil), f.bodies...)
}

func (f *fakeSentry) dsn(projectID string) string {
	return strings.Replace(f.URL, "http://", "http://public@", 1) + "/" + projectID
}

// recordedEvents keeps everything the backend's Sentry client sends. The
// client sends synchronously, so the buffer only has to hold the events of
// the requests between two takes.
type recordedEvents chan *sentry.Event

func (t recordedEvents) Configure(sentry.ClientOptions)        {}
func (t recordedEvents) Flush(time.Duration) bool              { return true }
func (t recordedEvents) FlushWithContext(context.Context) bool { return true }
func (t recordedEvents) Close()                                {}
func (t recordedEvents) SendEvent(event *sentry.Event)         { t <- event }

// take returns the error events and the transactions sent since the last
// call. Every request through the Sentry chain sends one transaction, so a
// transaction proves the chain saw the request.
func (t recordedEvents) take() (errors, transactions int) {
	for {
		select {
		case event := <-t:
			if event.Type == "transaction" {
				transactions++
			} else {
				errors++
			}
		default:
			return errors, transactions
		}
	}
}

// kioskEnvelope is what PyrePortal's SDK posts through its tunnel: the
// envelope header names the DSN, the event claims a device of its own.
func kioskEnvelope(dsn string) []byte {
	event := `{"event_id":"9ec79c33ec9942ab8353589fcb2e04dc","message":"kiosk crashed","tags":{"platform":"gkt","device_id":"forged"}}`
	return []byte(`{"event_id":"9ec79c33ec9942ab8353589fcb2e04dc","dsn":"` + dsn + `"}` + "\n" +
		`{"type":"event","length":` + strconv.Itoa(len(event)) + `}` + "\n" + event + "\n")
}

// forwardedTags decodes the event item of a forwarded envelope.
func forwardedTags(t *testing.T, envelope []byte) map[string]any {
	t.Helper()
	lines := bytes.Split(bytes.TrimSuffix(envelope, []byte("\n")), []byte("\n"))
	require.Len(t, lines, 3)
	var itemHeader struct {
		Type   string `json:"type"`
		Length int    `json:"length"`
	}
	require.NoError(t, json.Unmarshal(lines[1], &itemHeader))
	assert.Equal(t, "event", itemHeader.Type)
	assert.Equal(t, len(lines[2]), itemHeader.Length, "item length must match the rewritten event")
	var event struct {
		Tags map[string]any `json:"tags"`
	}
	require.NoError(t, json.Unmarshal(lines[2], &event))
	return event.Tags
}

// relayKiosk creates an active kiosk and returns its key, device ID and
// school.
type relayKiosk func(t *testing.T, name string) (apiKey, deviceID string, schoolID int64)

// checkIoTErrorReportsRelay runs against the production router that
// TestFullProductionRouterGolden built with fake's pyreportal DSN. It closes
// fake at the end to take Sentry down.
func checkIoTErrorReportsRelay(t *testing.T, router http.Handler, fake *fakeSentry, newKiosk relayKiosk) {
	t.Parallel()
	dsn := fake.dsn(relayTestProjectID)

	recorder := make(recordedEvents, 256)
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: recorder, EnableTracing: true, TracesSampleRate: 1})
	require.NoError(t, err)
	hub := sentry.NewHub(client, sentry.NewScope())

	post := func(body []byte, apiKey string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/iot/error-reports", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/x-sentry-envelope")
		if apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req.WithContext(sentry.SetHubOnContext(req.Context(), hub)))
		return w
	}

	t.Run("forwards the envelope with device and school", func(t *testing.T) {
		apiKey, deviceID, schoolID := newKiosk(t, "relay-kiosk")

		w := post(kioskEnvelope(dsn), apiKey)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.JSONEq(t, `{"id":"9ec79c33ec9942ab8353589fcb2e04dc"}`, w.Body.String())
		requests, bodies := fake.received()
		require.Len(t, requests, 1)
		assert.Equal(t, "/api/"+relayTestProjectID+"/envelope/", requests[0].URL.Path)
		assert.Equal(t, "application/x-sentry-envelope", requests[0].Header.Get("Content-Type"))
		assert.Empty(t, requests[0].Header.Get("Authorization"), "the device key must not reach Sentry")
		tags := forwardedTags(t, bodies[0])
		assert.Equal(t, deviceID, tags["device_id"], "the relay's device replaces the one the kiosk claimed")
		assert.Equal(t, strconv.FormatInt(schoolID, 10), tags["school_id"])
		assert.Equal(t, "gkt", tags["platform"], "client tags stay")
		errorEvents, _ := recorder.take()
		assert.Zero(t, errorEvents)
	})

	t.Run("refuses another project", func(t *testing.T) {
		apiKey, _, _ := newKiosk(t, "relay-foreign")
		before, _ := fake.received()

		w := post(kioskEnvelope(fake.dsn("999")), apiKey)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "error report project is not allowed")
		after, _ := fake.received()
		assert.Len(t, after, len(before))
	})

	t.Run("refuses a body that is no envelope", func(t *testing.T) {
		apiKey, _, _ := newKiosk(t, "relay-garbage")

		w := post([]byte("not an envelope"), apiKey)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid error report")
	})

	t.Run("refuses an envelope over 1 MB", func(t *testing.T) {
		apiKey, _, _ := newKiosk(t, "relay-large")
		before, _ := fake.received()
		body := append(kioskEnvelope(dsn), bytes.Repeat([]byte("x"), 1<<20)...)

		w := post(body, apiKey)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "error report too large")
		after, _ := fake.received()
		assert.Len(t, after, len(before))
	})

	t.Run("limits a device to 60 envelopes a minute", func(t *testing.T) {
		busy, _, _ := newKiosk(t, "relay-busy")
		quiet, _, _ := newKiosk(t, "relay-quiet")
		envelope := kioskEnvelope(dsn)

		for i := range 60 {
			w := post(envelope, busy)
			require.Equal(t, http.StatusOK, w.Code, "envelope %d", i+1)
		}
		w := post(envelope, busy)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "too many error reports")
		assert.Equal(t, "60", w.Header().Get("Retry-After"))
		assert.Equal(t, http.StatusOK, post(envelope, quiet).Code, "the quota is per device")
	})

	t.Run("requires the device key", func(t *testing.T) {
		w := post(kioskEnvelope(dsn), "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "device API key is required")

		w = post(kioskEnvelope(dsn), "stolen-key")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid device API key")
	})

	// The one exception to the 5xx rule: a relay that cannot reach Sentry
	// answers 502 and does not report that to Sentry.
	t.Run("answers 502 without a Sentry event when Sentry is down", func(t *testing.T) {
		apiKey, _, _ := newKiosk(t, "relay-down")
		fake.Close()
		recorder.take()

		w := post(kioskEnvelope(dsn), apiKey)

		assert.Equal(t, http.StatusBadGateway, w.Code)
		assert.Contains(t, w.Body.String(), "error reporting service unavailable")
		errorEvents, transactions := recorder.take()
		assert.Equal(t, 1, transactions, "the request passed the Sentry chain")
		assert.Zero(t, errorEvents, "an unreachable Sentry must not produce a Sentry event")
	})
}
