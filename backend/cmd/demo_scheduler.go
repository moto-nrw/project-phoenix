package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	backendapi "github.com/moto-nrw/project-phoenix/api"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
)

const (
	// demoSeedWorkers bounds the seed runs at a time; further orders wait.
	demoSeedWorkers = 3
	// demoSeedAttempts is the first run plus one repetition.
	demoSeedAttempts = 2
	// demoInUseWindow is how long an entry keeps a demo school simulated
	// (#3464). A visitor enters by redeeming the link; coming back later
	// redeems it again and brings the simulation back.
	demoInUseWindow = 30 * time.Minute
)

// demoScheduler gives every demo access its own demo school (#3463): it
// seeds queued orders with a bounded number of workers and keeps one ticker
// per school in use (#3464). It runs under the demo lease, so it is the only one.
type demoScheduler struct {
	schools   *backendapi.DemoRuntime
	baseURL   string
	heartbeat string
	adapter   *sharedOperatorSession

	tickers *demoTickers
	work    sync.WaitGroup
	seeds   chan struct{}
}

func runDemoSchools(ctx context.Context, schools *backendapi.DemoRuntime, baseURL, heartbeat string, once bool) error {
	adapter := newSeedCommandAdapter(baseURL, false)
	if err := waitDemoServer(ctx, adapter); err != nil {
		return err
	}
	if err := schools.ReleaseDemoSchoolOrders(ctx); err != nil {
		return err
	}
	scheduler := &demoScheduler{
		schools: schools, baseURL: baseURL, heartbeat: heartbeat, adapter: newSharedOperatorSession(adapter),
		tickers: newDemoTickers(), seeds: make(chan struct{}, demoSeedWorkers),
	}
	defer scheduler.work.Wait()
	if once {
		return scheduler.drain(ctx)
	}
	for {
		err := errors.Join(scheduler.startOrders(ctx), scheduler.startTickers(ctx))
		if err != nil && ctx.Err() == nil {
			slog.Warn("demo scheduler poll failed; retrying", "error", err)
		}
		if err == nil {
			// An idle scheduler is healthy: the heartbeat follows the poll.
			if err := writeDemoHeartbeat(heartbeat); err != nil {
				slog.Warn("demo heartbeat not written; the healthcheck will report unhealthy", "error", err)
			}
		}
		if waitDemoInterval(ctx, time.Second) != nil {
			return nil
		}
	}
}

// startOrders claims waiting orders while a seed worker is free.
func (s *demoScheduler) startOrders(ctx context.Context) error {
	for {
		if s.adapter.paused() {
			return nil // No operator session, no seed: the orders wait.
		}
		select {
		case s.seeds <- struct{}{}:
		default:
			return nil
		}
		order, err := s.schools.ClaimDemoSchoolOrder(ctx)
		if order == nil || err != nil {
			<-s.seeds
			return err
		}
		s.work.Go(func() {
			defer func() { <-s.seeds }()
			s.prepare(ctx, *order)
		})
	}
}

// prepare seeds the ordered school, runs its first tick and opens it. The
// school is ready only once it is in its running state, whatever the hour
// and weekday. A failed order is queued once more, then closed as failed.
func (s *demoScheduler) prepare(ctx context.Context, order backendapi.DemoSchoolOrder) {
	started := time.Now()
	err := s.seedAndTick(ctx, order)
	if ctx.Err() != nil {
		return // The next process releases the claim and tries again.
	}
	if err == nil {
		slog.Info("demo school ready",
			"school", order.Slug,
			"attempt", order.Attempts,
			"seconds", time.Since(started).Seconds(),
		)
		return
	}
	if s.adapter.paused() {
		// The operator could not sign in. That is not the order's fault, so
		// it goes back uncounted and waits with the others.
		returnErr := s.schools.ReturnDemoSchoolOrder(context.WithoutCancel(ctx), order.Slug)
		slog.Warn("demo school order waits for the operator login",
			"school", order.Slug,
			"error", errors.Join(err, returnErr),
		)
		return
	}
	s.adapter.forget()
	failed, failErr := s.schools.FailDemoSchoolOrder(context.WithoutCancel(ctx), order.Slug, demoSeedAttempts)
	slog.Warn("demo school order failed",
		"school", order.Slug,
		"attempt", order.Attempts,
		"final", failed,
		"error", errors.Join(err, failErr),
	)
}

func (s *demoScheduler) seedAndTick(ctx context.Context, order backendapi.DemoSchoolOrder) error {
	if order.Attempts > demoSeedAttempts {
		return fmt.Errorf("demo school order used its attempts")
	}
	if !order.Seeded {
		options := seedapi.SeedOptions{
			TenantSlug: order.Slug, SchoolName: order.SchoolName, VisitorName: order.PersonName,
			AccountScope: demoAccountScope(order.Slug, order.Attempts),
			// The broken attempt keeps its school and accounts; move both aside.
			ReplaceAbandoned: order.Attempts > 1,
		}
		if err := provisionDemoSchool(ctx, s.schools, s.adapter, options); err != nil {
			return err
		}
	}
	school, err := loadDemoSchool(ctx, s.schools, order.Slug, s.baseURL)
	if err != nil {
		return err
	}
	if err := school.run(ctx, true, true, func() {}); err != nil {
		return err
	}
	return s.schools.FinishDemoSchoolOrder(ctx, order.Slug, school.visitorID)
}

// demoAccountScope is what the school's account emails and usernames carry.
// Usernames end at 30 characters, so it is the slug's random suffix, not the
// slug. A repetition gets its own scope: the abandoned school keeps its
// accounts, and accounts are unique across the database.
func demoAccountScope(slug string, attempt int) string {
	scope := slug[strings.LastIndex(slug, "-")+1:]
	if attempt > 1 {
		scope = fmt.Sprintf("%s-%d", scope, attempt)
	}
	return scope
}

// startTickers keeps one ticker per school in use and stops the others. A
// new ticker ticks at once, so an entered school moves within the next poll.
func (s *demoScheduler) startTickers(ctx context.Context) error {
	slugs, err := s.schools.ActiveDemoSchools(ctx, time.Now().Add(-demoInUseWindow))
	if err != nil {
		return err
	}
	for _, slug := range s.tickers.keepOnly(slugs) {
		tickCtx, stop := context.WithCancel(ctx)
		ticker := s.tickers.add(slug, stop)
		s.work.Go(func() {
			defer s.tickers.remove(slug, ticker)
			school, err := loadDemoSchool(tickCtx, s.schools, slug, s.baseURL)
			if err == nil {
				err = school.run(tickCtx, false, false, func() {})
			}
			if err != nil && tickCtx.Err() == nil {
				slog.Warn("demo school ticker stopped; the next poll restarts it",
					"school", slug,
					"error", err,
				)
			}
		})
	}
	return nil
}

// demoTickers are the running tickers by school slug.
type demoTickers struct {
	mu      sync.Mutex
	running demoTickerSet
}

type demoTicker struct{ stop context.CancelFunc }

// demoTickerSet is runtime state, not wiring: tickers come and go with the
// schools in use.
type demoTickerSet map[string]*demoTicker

func (set demoTickerSet) put(slug string, ticker *demoTicker) { set[slug] = ticker }

func newDemoTickers() *demoTickers {
	return &demoTickers{running: demoTickerSet{}}
}

// keepOnly stops the tickers of schools that are no longer in use and returns
// the schools in use that have no ticker yet.
func (t *demoTickers) keepOnly(inUse []string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	wanted := make(map[string]bool, len(inUse))
	var missing []string
	for _, slug := range inUse {
		wanted[slug] = true
		if t.running[slug] == nil {
			missing = append(missing, slug)
		}
	}
	for slug, ticker := range t.running {
		if !wanted[slug] {
			ticker.stop()
			delete(t.running, slug)
		}
	}
	return missing
}

func (t *demoTickers) add(slug string, stop context.CancelFunc) *demoTicker {
	t.mu.Lock()
	defer t.mu.Unlock()
	ticker := &demoTicker{stop: stop}
	t.running.put(slug, ticker)
	return ticker
}

// remove forgets a ticker that ended on its own. A ticker that was stopped
// may already have a successor for its school, which stays.
func (t *demoTickers) remove(slug string, ticker *demoTicker) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ticker.stop()
	if t.running[slug] == ticker {
		delete(t.running, slug)
	}
}

// drain is --once: it empties the queue, repetitions included, and ticks
// every ready school once.
func (s *demoScheduler) drain(ctx context.Context) error {
	for {
		// A worker that was busy before the poll may still queue its order again.
		idle := len(s.seeds) == 0
		if err := s.startOrders(ctx); err != nil {
			return err
		}
		if idle && len(s.seeds) == 0 {
			if s.adapter.paused() {
				return fmt.Errorf("the operator login is refused; queued demo schools keep waiting")
			}
			break
		}
		if err := waitDemoInterval(ctx, 200*time.Millisecond); err != nil {
			return err
		}
	}
	slugs, err := s.schools.ReadyDemoSchools(ctx)
	if err != nil {
		return err
	}
	for _, slug := range slugs {
		school, err := loadDemoSchool(ctx, s.schools, slug, s.baseURL)
		if err != nil {
			return err
		}
		if err := school.run(ctx, true, true, func() {}); err != nil {
			return fmt.Errorf("demo school %s: %w", slug, err)
		}
	}
	return writeDemoHeartbeat(s.heartbeat)
}
