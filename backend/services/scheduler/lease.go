package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ErrLeaseNotHeld reports that a lease term ended: it expired, was released,
// or a newer holder took it over. A job transaction that sees it must not
// commit.
var ErrLeaseNotHeld = errors.New("worker lease is not held")

// LeaseTerm is one holder's tenure of the Worker lease. Until is database
// time and is evidence only; the holder decides locally with its own
// monotonic clock when to stop (see leadership.validUntil).
type LeaseTerm struct {
	Name   string
	Holder string
	Token  int64
	Until  time.Time
}

// LeaseStore is the Worker's port to its lease (#2726). Acquire, Renew and
// Release run in their own administrative transaction. Assert runs inside the
// job transaction open in ctx and returns ErrLeaseNotHeld unless term still
// holds the lease for at least margin.
type LeaseStore interface {
	Acquire(ctx context.Context, name, holder string, ttl time.Duration) (LeaseTerm, bool, error)
	Renew(ctx context.Context, term LeaseTerm, ttl time.Duration) (LeaseTerm, bool, error)
	Release(ctx context.Context, term LeaseTerm) error
	Assert(ctx context.Context, term LeaseTerm, margin time.Duration) error
}

// LeaseEvidence receives the runtime evidence of the lease. The composition
// root binds it to metrics; nil functions are skipped.
type LeaseEvidence struct {
	// Term reports the term this process holds; held is false in standby.
	Term func(held bool, token int64, until time.Time)
	// Operation reports one acquire, renew or release with its outcome
	// (ok, busy, lost, error) and latency.
	Operation func(operation, outcome string, latency time.Duration)
	// Leadership reports a change: acquired, lost, expired, fenced, released.
	Leadership func(change string)
	// Standby reports how long this process waited before it led.
	Standby func(time.Duration)
	// Suppressed reports a job run or commit that did not happen because
	// this process did not hold the lease: standby or fenced.
	Suppressed func(reason string)
	// Ready reports readiness, which equals holding the lease.
	Ready func(bool)
	// Drain reports how long a graceful stop waited for running jobs.
	Drain func(time.Duration)
}

// WorkerLease configures the leadership every Worker process runs under.
// Only the holder of the lease named Name runs jobs.
//
// A crashed holder is replaced after at most TTL + RetryEvery, a released
// one after at most RetryEvery. The holder stops on its own TTL-FenceMargin
// after its last successful acquire or renew, measured on its monotonic
// clock from before the statement was sent, so it stops before the database
// lets a standby in, whatever either clock says. Commits additionally assert
// the term with FenceMargin of database time to spare.
type WorkerLease struct {
	Store       LeaseStore
	Name        string
	Holder      string
	TTL         time.Duration
	RenewEvery  time.Duration
	RetryEvery  time.Duration
	FenceMargin time.Duration
	Evidence    LeaseEvidence
}

func (lease WorkerLease) validate() error {
	switch {
	case isNilDependency(lease.Store):
		return errors.New("worker dependency lease store is required")
	case lease.Name == "" || lease.Holder == "":
		return errors.New("worker lease name and holder are required")
	case lease.TTL <= 0 || lease.RenewEvery <= 0 || lease.RetryEvery <= 0 || lease.FenceMargin < 0:
		return errors.New("worker lease durations must be positive")
	case lease.RenewEvery >= lease.TTL-lease.FenceMargin:
		return fmt.Errorf("worker lease renewal every %s cannot keep a %s term with %s fence margin", lease.RenewEvery, lease.TTL, lease.FenceMargin)
	}
	return nil
}

const leaseReleaseTimeout = 5 * time.Second

// leadership elects this process as the single Worker leader. Job runs ask
// current() and run under the term's context, which ends when the term ends.
type leadership struct {
	config WorkerLease
	logger *slog.Logger
	parent context.Context

	// running holds the current term; nil in standby. Each term is an
	// immutable value, replaced whole under mu.
	running atomic.Pointer[leaseRun]
	mu      sync.Mutex
	since   atomic.Int64 // start of the current standby, Unix nanoseconds

	stopRun       context.CancelFunc
	runCtx        context.Context
	stopped       chan struct{}
	loopStarted   atomic.Bool
	deadlineTimer *time.Timer   // guarded by mu
	acquired      chan struct{} // closed on each new term, then replaced; guarded by mu
}

// leaseRun is one term as this process holds it. validUntil is local
// monotonic time; ctx ends when the term ends.
type leaseRun struct {
	term       LeaseTerm
	validUntil time.Time
	ctx        context.Context
	cancel     context.CancelFunc
}

func newLeadership(config WorkerLease, logger *slog.Logger, parent context.Context) *leadership {
	runCtx, stopRun := context.WithCancel(context.Background())
	l := &leadership{config: config, logger: logger, parent: parent, runCtx: runCtx, stopRun: stopRun, stopped: make(chan struct{}), acquired: make(chan struct{})}
	l.since.Store(time.Now().UnixNano())
	return l
}

// start tries to lead once, so a sole Worker leads before its first job
// check, then keeps the term alive in the background.
func (l *leadership) start() {
	wait := l.step(l.runCtx)
	l.loopStarted.Store(true)
	go func() {
		defer close(l.stopped)
		l.run(l.runCtx, wait)
	}()
}

// stop releases the lease and waits until the background loop ended.
func (l *leadership) stop() {
	l.stopRun()
	if l.loopStarted.Load() {
		<-l.stopped
	}
}

// run keeps the term alive until ctx ends, then releases it. The caller has
// already taken the first step; wait is the delay it returned.
func (l *leadership) run(ctx context.Context, wait time.Duration) {
	defer l.release(ctx)
	for {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		wait = l.step(ctx)
	}
}

// step acquires the lease in standby and renews it while leading. It returns
// how long to wait before the next step.
func (l *leadership) step(ctx context.Context) time.Duration {
	term, held, validUntil := l.snapshot()
	if !held {
		if l.acquire(ctx) {
			return l.config.RenewEvery
		}
		return l.config.RetryEvery
	}
	if !time.Now().Before(validUntil) {
		l.stepDown(term, "expired")
		return 0
	}
	if l.renew(ctx, term, validUntil) {
		return l.config.RenewEvery
	}
	return max(0, min(l.config.RetryEvery, time.Until(validUntil)))
}

func (l *leadership) snapshot() (LeaseTerm, bool, time.Time) {
	run := l.running.Load()
	if run == nil {
		return LeaseTerm{}, false, time.Time{}
	}
	return run.term, true, run.validUntil
}

func (l *leadership) acquire(ctx context.Context) bool {
	started := time.Now()
	callCtx, cancel := context.WithTimeout(ctx, l.config.TTL-l.config.FenceMargin)
	defer cancel()
	term, acquired, err := l.config.Store.Acquire(callCtx, l.config.Name, l.config.Holder, l.config.TTL)
	latency := time.Since(started)
	switch {
	case err != nil:
		l.operation("acquire", "error", latency)
		if ctx.Err() == nil {
			l.logger.Warn("worker lease acquire failed",
				slog.String("lease", l.config.Name),
				slog.String("error", err.Error()),
			)
		}
		return false
	case !acquired:
		l.operation("acquire", "busy", latency)
		return false
	}
	l.operation("acquire", "ok", latency)
	validUntil := started.Add(l.config.TTL - l.config.FenceMargin)
	if !time.Now().Before(validUntil) {
		// The answer came too late to lead safely. The next attempt by this
		// holder starts a newer term, which fences this one.
		return false
	}
	l.lead(term, validUntil)
	return true
}

func (l *leadership) renew(ctx context.Context, term LeaseTerm, validUntil time.Time) bool {
	started := time.Now()
	callCtx, cancel := context.WithDeadline(ctx, validUntil)
	defer cancel()
	renewed, ok, err := l.config.Store.Renew(callCtx, term, l.config.TTL)
	latency := time.Since(started)
	switch {
	case err != nil:
		l.operation("renew", "error", latency)
		if ctx.Err() == nil {
			l.logger.Warn("worker lease renewal failed",
				slog.String("lease", term.Name),
				slog.Int64("fencing_token", term.Token),
				slog.Duration("remaining", time.Until(validUntil)),
				slog.String("error", err.Error()),
			)
		}
		return false
	case !ok:
		l.operation("renew", "lost", latency)
		l.stepDown(term, "lost")
		return false
	}
	l.operation("renew", "ok", latency)
	nextValidUntil := started.Add(l.config.TTL - l.config.FenceMargin)
	l.mu.Lock()
	run := l.running.Load()
	if run != nil && run.term.Token == term.Token && time.Now().Before(run.validUntil) && time.Now().Before(nextValidUntil) {
		l.running.Store(&leaseRun{
			term: renewed, validUntil: nextValidUntil,
			ctx: run.ctx, cancel: run.cancel,
		})
		l.deadlineTimer.Stop()
		l.deadlineTimer = time.AfterFunc(time.Until(nextValidUntil), func() { l.expire(term) })
		l.mu.Unlock()
		l.evidenceTerm(true, renewed.Token, renewed.Until)
		return true
	}
	l.mu.Unlock()
	l.stepDown(term, "expired")
	return false
}

func (l *leadership) lead(term LeaseTerm, validUntil time.Time) {
	termCtx, cancel := context.WithCancel(l.parent)
	standby := time.Since(time.Unix(0, l.since.Load()))
	l.mu.Lock()
	l.running.Store(&leaseRun{term: term, validUntil: validUntil, ctx: termCtx, cancel: cancel})
	l.deadlineTimer = time.AfterFunc(time.Until(validUntil), func() { l.expire(term) })
	close(l.acquired)
	l.acquired = make(chan struct{})
	l.mu.Unlock()

	l.logger.Info("worker lease acquired",
		slog.String("lease", term.Name),
		slog.String("holder_id", term.Holder),
		slog.Int64("fencing_token", term.Token),
		slog.Time("lease_until", term.Until),
		slog.Duration("standby_duration", standby),
	)
	l.waited(standby)
	l.change("acquired")
	l.evidenceTerm(true, term.Token, term.Until)
	l.ready(true)
}

func (l *leadership) expire(term LeaseTerm) {
	if _, held, validUntil := l.snapshot(); held && !time.Now().Before(validUntil) {
		l.stepDown(term, "expired")
	}
}

func (l *leadership) nextAcquisition() <-chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.acquired
}

// stepDown ends term at once: its context is cancelled, so its running jobs
// stop and their transactions roll back. A later term is left alone.
func (l *leadership) stepDown(term LeaseTerm, reason string) {
	l.mu.Lock()
	run := l.running.Load()
	if run == nil || run.term.Token != term.Token {
		l.mu.Unlock()
		return
	}
	run.cancel()
	l.deadlineTimer.Stop()
	l.running.Store(nil)
	l.since.Store(time.Now().UnixNano())
	l.mu.Unlock()

	level := slog.LevelWarn
	if reason == "released" {
		level = slog.LevelInfo
	}
	l.logger.Log(context.Background(), level, "worker lease ended",
		slog.String("lease", term.Name),
		slog.Int64("fencing_token", term.Token),
		slog.String("reason", reason),
	)
	l.change(reason)
	l.evidenceTerm(false, term.Token, time.Time{})
	l.ready(false)
}

// release hands the lease over at shutdown, so a standby takes over without
// waiting for the term to expire.
func (l *leadership) release(ctx context.Context) {
	term, held, _ := l.snapshot()
	if !held {
		return
	}
	l.stepDown(term, "released")
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), leaseReleaseTimeout)
	defer cancel()
	started := time.Now()
	if err := l.config.Store.Release(releaseCtx, term); err != nil {
		l.operation("release", "error", time.Since(started))
		l.logger.Warn("worker lease release failed; the standby takes over when the term expires",
			slog.String("lease", term.Name),
			slog.Int64("fencing_token", term.Token),
			slog.String("error", err.Error()),
		)
		return
	}
	l.operation("release", "ok", time.Since(started))
}

// current returns the running term and its context. ok is false in standby
// and once the local validity of the term has passed.
func (l *leadership) current() (LeaseTerm, context.Context, bool) {
	run := l.running.Load()
	if run == nil || !time.Now().Before(run.validUntil) {
		return LeaseTerm{}, nil, false
	}
	return run.term, run.ctx, true
}

// fence returns the commit guard of term: every job transaction asserts the
// term before it commits, and a stale term ends at once.
func (l *leadership) fence(term LeaseTerm) func(context.Context) error {
	return func(ctx context.Context) error {
		err := l.config.Store.Assert(ctx, term, l.config.FenceMargin)
		if errors.Is(err, ErrLeaseNotHeld) {
			l.suppress("fenced")
			l.stepDown(term, "fenced")
		}
		return err
	}
}

func (l *leadership) suppress(reason string) {
	if l.config.Evidence.Suppressed != nil {
		l.config.Evidence.Suppressed(reason)
	}
}

func (l *leadership) operation(operation, outcome string, latency time.Duration) {
	if l.config.Evidence.Operation != nil {
		l.config.Evidence.Operation(operation, outcome, latency)
	}
}

func (l *leadership) change(change string) {
	if l.config.Evidence.Leadership != nil {
		l.config.Evidence.Leadership(change)
	}
}

func (l *leadership) evidenceTerm(held bool, token int64, until time.Time) {
	if l.config.Evidence.Term != nil {
		l.config.Evidence.Term(held, token, until)
	}
}

func (l *leadership) waited(standby time.Duration) {
	if l.config.Evidence.Standby != nil {
		l.config.Evidence.Standby(standby)
	}
}

func (l *leadership) drained(drain time.Duration) {
	if l.config.Evidence.Drain != nil {
		l.config.Evidence.Drain(drain)
	}
}

func (l *leadership) ready(ready bool) {
	if l.config.Evidence.Ready != nil {
		l.config.Evidence.Ready(ready)
	}
}
