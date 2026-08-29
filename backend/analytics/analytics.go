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
// Concurrency shape: each Capture spawns one goroutine, bounded per send by
// the client timeout but not small in aggregate — during a PostHog outage a
// check-in burst can hold up to rate×timeout goroutines at once. Acceptable
// at this product's scale; if it ever matters, the sturdier shape is a
// single background worker draining a bounded channel that drops events on
// overflow.
package analytics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	Close() error
}

// NewNoop returns a Tracker that discards all events. Used when no
// POSTHOG_API_KEY is configured (dev, CI, tests).
func NewNoop() Tracker {
	return noopTracker{}
}

type noopTracker struct{}

func (noopTracker) Capture(string, string, map[string]any) {}
func (noopTracker) Close() error                           { return nil }

// captureTimeout bounds each fire-and-forget send so a PostHog outage can
// never pile up goroutines indefinitely.
const captureTimeout = 5 * time.Second

// New returns a PostHog-backed Tracker when apiKey is set, or a no-op
// Tracker when it is empty. A set apiKey with an empty or invalid host is
// a configuration error (no silent default host).
func New(apiKey, host string, logger *slog.Logger) (Tracker, error) {
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

	return &httpTracker{
		endpoint: strings.TrimRight(host, "/") + "/batch/",
		apiKey:   apiKey,
		sender:   httpSender{client: &http.Client{Timeout: captureTimeout}},
		logger:   logger,
	}, nil
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
	endpoint string
	apiKey   string
	sender   batchSender
	logger   *slog.Logger
	wg       sync.WaitGroup
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
	// Copy props so the caller's map is never mutated or read concurrently.
	properties := make(map[string]any, len(props)+1)
	for k, v := range props {
		properties[k] = v
	}
	properties["$lib"] = "phoenix-backend"
	// Product analytics is aggregated by school. Disable IP enrichment and
	// person-profile processing for every backend event, regardless of caller.
	properties["$geoip_disable"] = true
	properties["$process_person_profile"] = false

	body, err := json.Marshal(batchPayload{
		APIKey: t.apiKey,
		Batch: []batchMessage{{
			Event:      event,
			DistinctID: distinctID,
			Timestamp:  time.Now().UTC(),
			Properties: properties,
		}},
	})
	if err != nil {
		t.warnCaptureFailed(event, err)
		return
	}

	// Send in a goroutine so analytics latency or backpressure can never
	// stall the calling request. The client timeout bounds each send.
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.send(event, body)
	}()
}

func (t *httpTracker) send(event string, body []byte) {
	status, err := t.sender.Post(t.endpoint, body)
	if err != nil {
		t.warnCaptureFailed(event, err)
		return
	}
	if status >= http.StatusMultipleChoices {
		t.warnCaptureFailed(event, fmt.Errorf("posthog returned status %d", status))
	}
}

// Close waits for all in-flight captures to finish (each bounded by the
// client timeout). Contract: Capture must not be called concurrently with
// or after Close — wg.Add racing wg.Wait is WaitGroup misuse. Today this
// holds because Close runs only after the HTTP server has stopped.
func (t *httpTracker) Close() error {
	t.wg.Wait()
	return nil
}

func (t *httpTracker) warnCaptureFailed(event string, err error) {
	loggerOrDefault(t.logger).Warn("posthog capture failed",
		slog.String("event", event),
		slog.String("error", err.Error()),
	)
}

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
