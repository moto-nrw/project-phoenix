// Package analytics provides a thin product-analytics tracker that posts
// events to PostHog's /batch/ HTTP endpoint. Capture is fire-and-forget:
// errors are logged and swallowed so analytics can never affect business
// logic. Events must not contain student PII — no student IDs, only
// tenant-level properties (GDPR).
//
// The tracker deliberately speaks the HTTP API directly instead of using
// the official posthog-go SDK: the SDK depends on MPL-2.0-licensed code
// (hashicorp/golang-lru), which the license policy for this
// source-available project disallows.
//
// Concurrency shape: Capture only enqueues. One background worker drains a
// bounded queue and posts the events in batches, at the latest after the
// flush interval. A full queue drops the event with a warning instead of
// blocking the request (a PostHog outage must never stall check-ins).
// Close sends what is still queued.
package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Tracker captures product analytics events. Services depend on this
// interface, never on the HTTP client directly.
type Tracker interface {
	Capture(distinctID, event string, props map[string]any)
	// CaptureContext is Capture for a request: the browser session in ctx
	// (WithSessionID) links the event to the page views of that session.
	CaptureContext(ctx context.Context, distinctID, event string, props map[string]any)
	Close() error
}

// NewNoop returns a Tracker that discards all events. Used when no
// POSTHOG_API_KEY is configured (dev, CI, tests).
func NewNoop() Tracker {
	return noopTracker{}
}

type noopTracker struct{}

func (noopTracker) Capture(string, string, map[string]any)                         {}
func (noopTracker) CaptureContext(context.Context, string, string, map[string]any) {}
func (noopTracker) Close() error                                                   { return nil }

const (
	// sendTimeout bounds each batch post so a PostHog outage cannot hold
	// the worker, and with it the shutdown, indefinitely.
	sendTimeout = 5 * time.Second
	// flushInterval is how long an event waits at most for its batch.
	flushInterval = 5 * time.Second
	// batchSize sends a batch early once this many events are queued.
	batchSize = 50
	// queueSize caps the events waiting for the worker; more are dropped.
	queueSize = 1000
)

// New returns a PostHog-backed Tracker when apiKey is set, or a no-op
// Tracker when it is empty. A set apiKey with an empty or invalid host, or
// without a deployment, is a configuration error (no silent defaults).
//
// deployment is stamped on every event: the tenant domain of this instance,
// or "demo" for the public demo. It keeps demo and school events apart in
// the one PostHog project, which is why the demo may send too.
func New(apiKey, host, deployment string, logger *slog.Logger) (Tracker, error) {
	if apiKey == "" {
		return NewNoop(), nil
	}
	if host == "" {
		return nil, fmt.Errorf("POSTHOG_HOST is required when POSTHOG_API_KEY is set")
	}
	parsed, err := url.Parse(host)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("POSTHOG_HOST %q is not a valid URL", host)
	}
	if strings.TrimSpace(deployment) == "" {
		return nil, fmt.Errorf("the analytics deployment (TENANT_DOMAIN, or APP_ENV=demo) is required when POSTHOG_API_KEY is set")
	}

	return newHTTPTracker(
		strings.TrimRight(host, "/")+"/batch/",
		apiKey,
		strings.TrimSpace(deployment),
		httpSender{client: &http.Client{Timeout: sendTimeout}},
		logger,
		flushInterval,
	), nil
}

type batchSender interface {
	Post(endpoint string, body []byte) (int, error)
}

type httpSender struct{ client *http.Client }

func (s httpSender) Post(endpoint string, body []byte) (int, error) {
	resp, err := s.client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

type httpTracker struct {
	endpoint   string
	apiKey     string
	deployment string
	sender     batchSender
	logger     *slog.Logger
	interval   time.Duration

	queue     chan batchMessage
	done      chan struct{}
	stopped   chan struct{}
	closeOnce sync.Once
}

func newHTTPTracker(endpoint, apiKey, deployment string, sender batchSender, logger *slog.Logger, interval time.Duration) *httpTracker {
	t := &httpTracker{
		endpoint:   endpoint,
		apiKey:     apiKey,
		deployment: deployment,
		sender:     sender,
		logger:     logger,
		interval:   interval,
		queue:      make(chan batchMessage, queueSize),
		done:       make(chan struct{}),
		stopped:    make(chan struct{}),
	}
	go t.run()
	return t
}

// batchPayload mirrors the wire format of PostHog's /batch/ endpoint (the
// same one posthog-go uses).
type batchPayload struct {
	APIKey string         `json:"api_key"`
	Batch  []batchMessage `json:"batch"`
}

type batchMessage struct {
	Event      string         `json:"event"`
	DistinctID string         `json:"distinct_id"`
	Timestamp  time.Time      `json:"timestamp"`
	Properties map[string]any `json:"properties"`
}

func (t *httpTracker) Capture(distinctID, event string, props map[string]any) {
	t.CaptureContext(context.Background(), distinctID, event, props)
}

func (t *httpTracker) CaptureContext(ctx context.Context, distinctID, event string, props map[string]any) {
	// Copy props so the caller's map is never mutated or read concurrently.
	properties := make(map[string]any, len(props)+5)
	maps.Copy(properties, props)
	properties["$lib"] = "phoenix-backend"
	properties["deployment"] = t.deployment
	if sessionID := SessionIDFromContext(ctx); sessionID != "" {
		properties["$session_id"] = sessionID
	}
	// Product analytics is aggregated by school. Disable IP enrichment and
	// person-profile processing for every backend event, regardless of caller.
	properties["$geoip_disable"] = true
	properties["$process_person_profile"] = false

	// Unmarshalable properties fail here, per event, not for the whole batch.
	if _, err := json.Marshal(properties); err != nil {
		t.warnCaptureFailed(event, err)
		return
	}

	message := batchMessage{
		Event:      event,
		DistinctID: distinctID,
		Timestamp:  time.Now().UTC(),
		Properties: properties,
	}
	select {
	case <-t.done:
		// Capture after Close: the worker is gone, the event is dropped.
		return
	default:
	}
	select {
	case t.queue <- message:
	default:
		t.warnCaptureFailed(event, fmt.Errorf("queue full, event dropped"))
	}
}

// run is the single worker: it collects queued events and sends them as
// one batch when the batch is full, when the interval elapses, and once
// more on Close.
func (t *httpTracker) run() {
	defer close(t.stopped)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	batch := make([]batchMessage, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		t.send(batch)
		batch = batch[:0]
	}
	add := func(message batchMessage) {
		batch = append(batch, message)
		if len(batch) >= batchSize {
			flush()
		}
	}

	for {
		select {
		case message := <-t.queue:
			add(message)
		case <-ticker.C:
			flush()
		case <-t.done:
			for {
				select {
				case message := <-t.queue:
					add(message)
				default:
					flush()
					return
				}
			}
		}
	}
}

func (t *httpTracker) send(batch []batchMessage) {
	body, err := json.Marshal(batchPayload{APIKey: t.apiKey, Batch: batch})
	if err != nil {
		t.warnBatchFailed(len(batch), err)
		return
	}
	status, err := t.sender.Post(t.endpoint, body)
	if err != nil {
		t.warnBatchFailed(len(batch), err)
		return
	}
	if status >= http.StatusMultipleChoices {
		t.warnBatchFailed(len(batch), fmt.Errorf("posthog returned status %d", status))
	}
}

// Close sends the queued events and stops the worker. Each send is bounded
// by the client timeout. Events captured after Close are dropped.
func (t *httpTracker) Close() error {
	t.closeOnce.Do(func() { close(t.done) })
	<-t.stopped
	return nil
}

func (t *httpTracker) warnCaptureFailed(event string, err error) {
	loggerOrDefault(t.logger).Warn("posthog capture failed",
		slog.String("event", event),
		slog.String("error", err.Error()),
	)
}

func (t *httpTracker) warnBatchFailed(events int, err error) {
	loggerOrDefault(t.logger).Warn("posthog capture failed",
		slog.Int("events", events),
		slog.String("error", err.Error()),
	)
}

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
