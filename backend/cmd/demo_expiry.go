package cmd

import (
	"context"
	"log/slog"
	"slices"
	"time"
)

// demoExpiryInterval is how often the demo process looks for demo accesses
// past their end (#3470). An access ends 14 days after its last use, so an
// hour's delay changes nothing a visitor notices.
const demoExpiryInterval = time.Hour

// demoExpiryRuntime is what the expiry needs from the demo runtime.
type demoExpiryRuntime interface {
	ExpireDemoAccesses(ctx context.Context) (deleted int, orphanedSlugs []string, err error)
	RetireDemoSchools(ctx context.Context, slugs []string) (hidden int, err error)
}

// demoExpiry deletes demo accesses 14 days after their last use and hides
// the demo schools left without any access (#3470). It runs once per
// interval on the injected clock; the first run is due at once. The standing
// school is shared by every visitor and is never hidden.
type demoExpiry struct {
	runtime  demoExpiryRuntime
	now      func() time.Time
	interval time.Duration
	next     time.Time
	// pending are schools whose accesses are gone but that could not be
	// hidden yet; the next run tries again, so no school outlives its links.
	pending []string
}

func newDemoExpiry(runtime demoExpiryRuntime, now func() time.Time) *demoExpiry {
	return &demoExpiry{runtime: runtime, now: now, interval: demoExpiryInterval}
}

// run performs the expiry when it is due. A failed run is due again at the
// next poll.
func (e *demoExpiry) run(ctx context.Context) error {
	at := e.now()
	if at.Before(e.next) {
		return nil
	}
	deleted, orphaned, err := e.runtime.ExpireDemoAccesses(ctx)
	if err != nil {
		return err
	}
	for _, slug := range orphaned {
		if slug != standingDemoSlug && !slices.Contains(e.pending, slug) {
			e.pending = append(e.pending, slug)
		}
	}
	hidden := 0
	if len(e.pending) > 0 {
		if hidden, err = e.runtime.RetireDemoSchools(ctx, e.pending); err != nil {
			return err
		}
		e.pending = nil
	}
	e.next = at.Add(e.interval)
	if deleted > 0 || hidden > 0 {
		slog.Info("demo accesses expired",
			"accesses", deleted,
			"schools", hidden,
		)
	}
	return nil
}
