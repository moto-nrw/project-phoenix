// Package analytics provides a thin product-analytics tracker backed by
// PostHog. Capture is fire-and-forget: errors are logged and swallowed so
// analytics can never affect business logic. Events must not contain
// student PII — no student IDs, only tenant-level properties (GDPR).
package analytics

import (
	"fmt"
	"log/slog"

	"github.com/posthog/posthog-go"
)

// Tracker captures product analytics events. Services depend on this
// interface, never on the PostHog client directly.
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

// New returns a PostHog-backed Tracker when apiKey is set, or a no-op
// Tracker when it is empty. A set apiKey with an empty host is a
// configuration error (no silent default host).
func New(apiKey, host string, logger *slog.Logger) (Tracker, error) {
	if apiKey == "" {
		return NewNoop(), nil
	}
	if host == "" {
		return nil, fmt.Errorf("POSTHOG_HOST is required when POSTHOG_API_KEY is set")
	}

	client, err := posthog.NewWithConfig(apiKey, posthog.Config{Endpoint: host})
	if err != nil {
		return nil, fmt.Errorf("initializing posthog client: %w", err)
	}

	return &posthogTracker{client: client, logger: logger}, nil
}

type posthogTracker struct {
	client posthog.Client
	logger *slog.Logger
}

func (t *posthogTracker) Capture(distinctID, event string, props map[string]any) {
	// Enqueue normally just writes to a buffered channel, but it blocks when
	// the buffer is full (e.g. during a PostHog outage). Run it in a goroutine
	// so analytics backpressure can never stall the calling request.
	go func() {
		if err := t.client.Enqueue(posthog.Capture{
			DistinctId: distinctID,
			Event:      event,
			Properties: posthog.Properties(props),
		}); err != nil {
			t.getLogger().Warn("posthog capture failed",
				slog.String("event", event),
				slog.String("error", err.Error()),
			)
		}
	}()
}

func (t *posthogTracker) Close() error {
	return t.client.Close()
}

func (t *posthogTracker) getLogger() *slog.Logger {
	if t.logger != nil {
		return t.logger
	}
	return slog.Default()
}
