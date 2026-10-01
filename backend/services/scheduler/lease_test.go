package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memoryLeaseStore mirrors platform.worker_leases: one row, database time,
// a token that grows with every new holder. skew shifts the clock the store
// reports, so tests can show the Worker never trusts it.
type memoryLeaseStore struct {
	mu          sync.Mutex
	holder      string
	token       int64
	until       time.Time
	skew        time.Duration
	down        bool
	unreachable map[string]bool
	drained     *atomic.Bool
	releasedAt  []bool
}

var errLeaseDatabaseDown = errors.New("database unreachable")

func newMemoryLeaseStore() *memoryLeaseStore {
	return &memoryLeaseStore{unreachable: make(map[string]bool)}
}

func (m *memoryLeaseStore) failing(holder string) bool {
	return m.down || m.unreachable[holder]
}

func (m *memoryLeaseStore) Acquire(ctx context.Context, name, holder string, ttl time.Duration) (LeaseTerm, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing(holder) {
		return LeaseTerm{}, false, errLeaseDatabaseDown
	}
	now := time.Now()
	if m.holder != "" && m.holder != holder && m.until.After(now) {
		return LeaseTerm{}, false, nil
	}
	m.holder, m.token, m.until = holder, m.token+1, now.Add(ttl)
	return LeaseTerm{Name: name, Holder: holder, Token: m.token, Until: m.until.Add(m.skew)}, true, ctx.Err()
}

func (m *memoryLeaseStore) Renew(_ context.Context, term LeaseTerm, ttl time.Duration) (LeaseTerm, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing(term.Holder) {
		return LeaseTerm{}, false, errLeaseDatabaseDown
	}
	now := time.Now()
	if m.holder != term.Holder || m.token != term.Token || !m.until.After(now) {
		return LeaseTerm{}, false, nil
	}
	m.until = now.Add(ttl)
	term.Until = m.until.Add(m.skew)
	return term, true, nil
}

func (m *memoryLeaseStore) Release(_ context.Context, term LeaseTerm) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing(term.Holder) {
		return errLeaseDatabaseDown
	}
	if m.holder == term.Holder && m.token == term.Token && m.until.After(time.Now()) {
		m.until = time.Now()
	}
	if m.drained != nil {
		m.releasedAt = append(m.releasedAt, m.drained.Load())
	}
	return nil
}

func (m *memoryLeaseStore) Assert(_ context.Context, term LeaseTerm, margin time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing(term.Holder) {
		return errLeaseDatabaseDown
	}
	if m.holder != term.Holder || m.token != term.Token || !m.until.After(time.Now().Add(margin)) {
		return fmt.Errorf("%w: token %d", ErrLeaseNotHeld, term.Token)
	}
	return nil
}

// takeOver simulates another process taking the lease after it expired.
func (m *memoryLeaseStore) takeOver(holder string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.holder, m.token, m.until = holder, m.token+1, time.Now().Add(time.Hour)
	return m.token
}

func (m *memoryLeaseStore) set(fn func(*memoryLeaseStore)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(m)
}

func (m *memoryLeaseStore) current() (string, int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.until.After(time.Now()) {
		return "", m.token
	}
	return m.holder, m.token
}

type delayedRenewStore struct {
	*memoryLeaseStore
	started chan struct{}
	release chan struct{}
}

func (m *delayedRenewStore) Renew(_ context.Context, term LeaseTerm, ttl time.Duration) (LeaseTerm, bool, error) {
	select {
	case m.started <- struct{}{}:
	default:
	}
	<-m.release // simulates a store that returns after its context deadline
	term.Until = time.Now().Add(ttl)
	return term, true, nil
}

type leaseRecorder struct {
	mu         sync.Mutex
	changes    []string
	suppressed map[string]int
	ready      []bool
	drains     []time.Duration
	operations map[string]int
}

func (r *leaseRecorder) evidence() LeaseEvidence {
	r.suppressed = make(map[string]int)
	r.operations = make(map[string]int)
	return LeaseEvidence{
		Operation: func(operation, outcome string, _ time.Duration) {
			r.mu.Lock()
			r.operations[operation+"/"+outcome]++
			r.mu.Unlock()
		},
		Leadership: func(change string) {
			r.mu.Lock()
			r.changes = append(r.changes, change)
			r.mu.Unlock()
		},
		Suppressed: func(reason string) {
			r.mu.Lock()
			r.suppressed[reason]++
			r.mu.Unlock()
		},
		Ready: func(ready bool) {
			r.mu.Lock()
			r.ready = append(r.ready, ready)
			r.mu.Unlock()
		},
		Drain: func(duration time.Duration) {
			r.mu.Lock()
			r.drains = append(r.drains, duration)
			r.mu.Unlock()
		},
	}
}

func (r *leaseRecorder) count(reason string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.suppressed[reason]
}

func (r *leaseRecorder) changed(change string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, recorded := range r.changes {
		if recorded == change {
			return true
		}
	}
	return false
}

const (
	testLeaseTTL    = 400 * time.Millisecond
	testLeaseRenew  = 80 * time.Millisecond
	testLeaseRetry  = 40 * time.Millisecond
	testLeaseMargin = 80 * time.Millisecond
)

func leaseWorker(t *testing.T, store LeaseStore, holder string, recorder *leaseRecorder) *Scheduler {
	t.Helper()
	deps := minimalWorkerDependencies(t)
	deps.Logger = slog.New(slog.DiscardHandler)
	deps.Lease = WorkerLease{
		Store: store, Name: "worker", Holder: holder,
		TTL: testLeaseTTL, RenewEvery: testLeaseRenew, RetryEvery: testLeaseRetry, FenceMargin: testLeaseMargin,
	}
	if recorder != nil {
		deps.Lease.Evidence = recorder.evidence()
	}
	require.NoError(t, deps.Lease.validate())
	worker := newScheduler(deps)
	registry, err := NewRegistry(nil)
	require.NoError(t, err)
	worker.registry = registry
	return worker
}

// stopOnce lets a test stop a worker itself and still clean up on failure.
func stopOnce(t *testing.T, worker *Scheduler) func() {
	t.Helper()
	var once sync.Once
	stop := func() { once.Do(worker.Stop) }
	t.Cleanup(stop)
	return stop
}

// probeRun runs one job check through the production gate and reports
// whether the check executed.
func probeRun(worker *Scheduler, check func(context.Context)) bool {
	ran := false
	worker.runJobCheck(&ScheduledTask{Name: "lease-probe"}, func(ctx context.Context, _ *ScheduledTask) {
		ran = true
		check(ctx)
	})
	return ran
}

func TestWorkerLeaseConfigurationIsRequiredAndConsistent(t *testing.T) {
	t.Parallel()
	valid := WorkerLease{
		Store: newMemoryLeaseStore(), Name: "worker", Holder: "a",
		TTL: 30 * time.Second, RenewEvery: 10 * time.Second, RetryEvery: 5 * time.Second, FenceMargin: 5 * time.Second,
	}
	require.NoError(t, valid.validate())

	tests := map[string]func(*WorkerLease){
		"store":           func(l *WorkerLease) { l.Store = nil },
		"holder":          func(l *WorkerLease) { l.Holder = "" },
		"positive":        func(l *WorkerLease) { l.RetryEvery = 0 },
		"cannot keep":     func(l *WorkerLease) { l.RenewEvery = 25 * time.Second },
		"negative margin": func(l *WorkerLease) { l.FenceMargin = -time.Second },
	}
	for name, mutate := range tests {
		lease := valid
		mutate(&lease)
		require.Error(t, lease.validate(), name)
	}

	deps := minimalWorkerDependencies(t)
	deps.Lease = WorkerLease{}
	require.ErrorContains(t, validateWorkerDependencies(deps), "lease store is required", "no Worker runs without the lease")
}

func TestStandbyWorkerIsNotReadyAndRunsNoJob(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	store.takeOver("other-worker")
	recorder := &leaseRecorder{}
	worker := leaseWorker(t, store, "standby", recorder)
	stopOnce(t, worker)

	worker.Start()

	assert.False(t, worker.Ready(), "readiness is gated on lease ownership")
	assert.False(t, probeRun(worker, func(context.Context) {}), "a standby suppresses the run")
	assert.Equal(t, 1, recorder.count("standby"))
}

func TestSoleWorkerLeadsAtStartAndStaysReadyAcrossRenewals(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	worker := leaseWorker(t, store, "leader", nil)
	stop := stopOnce(t, worker)

	worker.Start()
	require.True(t, worker.Ready(), "a sole Worker leads before its first job check")
	_, firstToken := store.current()

	time.Sleep(3 * testLeaseTTL)
	assert.True(t, worker.Ready(), "renewal keeps the term beyond its TTL")
	holder, token := store.current()
	assert.Equal(t, "leader", holder)
	assert.Equal(t, firstToken, token, "renewal keeps the fencing token")

	stop()
	holder, _ = store.current()
	assert.Empty(t, holder, "a graceful stop releases the lease")
	assert.False(t, worker.Ready())
}

func TestLeaseLossMidJobCancelsTheRunAndFencesItsCommit(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	recorder := &leaseRecorder{}
	worker := leaseWorker(t, store, "leader", recorder)
	stopOnce(t, worker)
	worker.Start()
	require.True(t, worker.Ready())

	var commitErr, cancelled error
	ran := probeRun(worker, func(ctx context.Context) {
		store.takeOver("usurper")

		commitErr = tenant.WithinAdmin(worker.withUnitOfWork(ctx), func(context.Context) error { return nil })

		select {
		case <-ctx.Done():
			cancelled = ctx.Err()
		case <-time.After(time.Second):
		}
	})

	require.True(t, ran)
	require.ErrorIs(t, commitErr, ErrLeaseNotHeld, "a stale token is rejected at finalize")
	require.ErrorIs(t, cancelled, context.Canceled, "the fenced term cancels its running job")
	assert.Equal(t, 1, recorder.count("fenced"))
	assert.True(t, recorder.changed("fenced"))
	assert.False(t, worker.Ready())
	assert.False(t, probeRun(worker, func(context.Context) {}), "the fenced Worker runs nothing until it leads again")
}

func TestLostRenewalEndsTheTermAndCancelsItsJob(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	recorder := &leaseRecorder{}
	worker := leaseWorker(t, store, "leader", recorder)
	stopOnce(t, worker)
	worker.Start()

	var cancelled error
	probeRun(worker, func(ctx context.Context) {
		store.takeOver("usurper")
		select {
		case <-ctx.Done():
			cancelled = ctx.Err()
		case <-time.After(2 * testLeaseRenew):
		}
	})

	require.ErrorIs(t, cancelled, context.Canceled, "the next renewal finds the term gone and stops the job")
	require.Eventually(t, func() bool { return recorder.changed("lost") }, time.Second, time.Millisecond)
	assert.False(t, worker.Ready())
}

func TestLocalDeadlineCancelsJobWhileRenewalIsInFlight(t *testing.T) {
	t.Parallel()
	store := &delayedRenewStore{memoryLeaseStore: newMemoryLeaseStore(), started: make(chan struct{}, 1), release: make(chan struct{})}
	worker := leaseWorker(t, store, "leader", nil)
	lease := worker.leadership
	require.True(t, lease.acquire(context.Background()))
	term, _, validUntil := lease.snapshot()
	renewed := make(chan bool, 1)
	go func() { renewed <- lease.renew(context.Background(), term, validUntil) }()
	<-store.started

	_, jobCtx, held := lease.current()
	require.True(t, held)
	select {
	case <-jobCtx.Done():
	case <-time.After(testLeaseTTL):
		t.Fatal("the local deadline did not cancel the running job")
	}
	_, _, held = lease.current()
	assert.False(t, held)
	close(store.release)
	assert.False(t, <-renewed, "a late successful response cannot restore the expired term")
	_, _, held = lease.current()
	assert.False(t, held)
}

func TestStandbyRunsIntervalStartupCheckOnTakeover(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	store.takeOver("other-worker")
	recorder := &leaseRecorder{}
	worker := leaseWorker(t, store, "standby", recorder)
	stopOnce(t, worker)
	worker.Start()

	checked := make(chan struct{}, 1)
	worker.registerTask("takeover-check", "1h", func(task *ScheduledTask) {
		worker.runIntervalPolling(task, "takeover check", "takeover check started", 0,
			func() time.Duration { return time.Hour }, func(context.Context, *ScheduledTask) {
				select {
				case checked <- struct{}{}:
				default:
				}
			})
	})
	require.Eventually(t, func() bool { return recorder.count("standby") > 0 }, time.Second, time.Millisecond)
	store.set(func(m *memoryLeaseStore) { m.until = time.Now() })
	require.Eventually(t, worker.Ready, time.Second, time.Millisecond)
	select {
	case <-checked:
	case <-time.After(time.Second):
		t.Fatal("the skipped startup check was not retried on takeover")
	}
}

func TestStandbyRunsMinuteStartupCheckOnTakeover(t *testing.T) {
	t.Parallel()
	testMinuteTakeoverCheck(t, 0)
}

func TestStandbyRunsMinuteCheckOnTakeoverAfterTick(t *testing.T) {
	t.Parallel()
	testMinuteTakeoverCheck(t, 2*time.Minute+time.Second)
}

func testMinuteTakeoverCheck(t *testing.T, standbyFor time.Duration) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		store := newMemoryLeaseStore()
		store.takeOver("other-worker")
		recorder := &leaseRecorder{}
		worker := leaseWorker(t, store, "standby", recorder)
		defer func() {
			worker.Stop()
			if term, held, _ := worker.leadership.snapshot(); held {
				worker.leadership.stepDown(term, "released")
			}
		}()

		checked := make(chan struct{}, 1)
		worker.registerTask("minute-takeover-check", "1m", func(task *ScheduledTask) {
			worker.runMinutePolling(task, "minute takeover check", "minute takeover check started",
				func(context.Context, *ScheduledTask) { checked <- struct{}{} })
		})
		synctest.Wait()
		require.Equal(t, 1, recorder.count("standby"), "the startup check was skipped")
		if standbyFor > 0 {
			time.Sleep(standbyFor)
			synctest.Wait()
			require.GreaterOrEqual(t, recorder.count("standby"), 2, "a scheduled minute check was skipped")
		}

		store.set(func(m *memoryLeaseStore) { m.until = time.Now() })
		require.True(t, worker.leadership.acquire(context.Background()))
		synctest.Wait()
		select {
		case <-checked:
		default:
			t.Fatal("the skipped check was not retried before the next minute")
		}
	})
}

func TestDatabaseOutageEndsTheTermLocallyBeforeTheLeaseExpires(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	recorder := &leaseRecorder{}
	worker := leaseWorker(t, store, "leader", recorder)
	stopOnce(t, worker)
	worker.Start()
	_, firstToken := store.current()

	var stoppedAt time.Time
	var dbExpiry time.Time
	probeRun(worker, func(ctx context.Context) {
		store.set(func(m *memoryLeaseStore) {
			m.down = true
			dbExpiry = m.until
		})
		select {
		case <-ctx.Done():
			stoppedAt = time.Now()
		case <-time.After(3 * testLeaseTTL):
		}
	})

	require.False(t, stoppedAt.IsZero(), "an unreachable database ends the term on the Worker's own clock")
	assert.True(t, stoppedAt.Before(dbExpiry), "the Worker stops before the database lets a standby in")
	require.Eventually(t, func() bool { return recorder.changed("expired") }, time.Second, time.Millisecond)
	assert.False(t, worker.Ready())

	store.set(func(m *memoryLeaseStore) { m.down = false })
	require.Eventually(t, worker.Ready, 3*testLeaseTTL, 10*time.Millisecond, "the Worker leads again once the database is back")
	_, token := store.current()
	assert.Greater(t, token, firstToken, "the recovered Worker leads under a newer token")
}

func TestWorkerTrustsItsOwnMonotonicClockNotTheReportedLeaseTime(t *testing.T) {
	t.Parallel()
	for name, skew := range map[string]time.Duration{"database behind": -time.Hour, "database ahead": time.Hour} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := newMemoryLeaseStore()
			store.skew = skew
			worker := leaseWorker(t, store, "leader", nil)
			stopOnce(t, worker)
			worker.Start()

			time.Sleep(2 * testLeaseTTL)
			require.True(t, worker.Ready(), "a skewed lease time neither ends nor extends a renewed term")

			store.set(func(m *memoryLeaseStore) { m.down = true })
			require.Eventually(t, func() bool { return !worker.Ready() }, testLeaseTTL, 5*time.Millisecond,
				"without renewal the term ends within its TTL however far the reported time lies ahead")
		})
	}
}

func TestStandbyTakesOverAReleasedLeaseWithinTheRetryInterval(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	leader := leaseWorker(t, store, "leader", nil)
	stopLeader := stopOnce(t, leader)
	leader.Start()
	_, leaderToken := store.current()
	standby := leaseWorker(t, store, "standby", nil)
	stopOnce(t, standby)
	standby.Start()
	require.True(t, leader.Ready())
	require.False(t, standby.Ready())

	stopLeader()
	released := time.Now()
	require.Eventually(t, standby.Ready, time.Second, time.Millisecond)

	assert.Less(t, time.Since(released), testLeaseRetry+250*time.Millisecond, "graceful takeover is bounded by the retry interval")
	holder, token := store.current()
	assert.Equal(t, "standby", holder)
	assert.Equal(t, leaderToken+1, token, "the new holder leads under the next token")
}

func TestStandbyTakesOverACrashedLeaderAfterItsTermExpires(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	leader := leaseWorker(t, store, "leader", nil)
	stopOnce(t, leader)
	leader.Start()
	standby := leaseWorker(t, store, "standby", nil)
	stopOnce(t, standby)
	standby.Start()

	var dbExpiry time.Time
	store.set(func(m *memoryLeaseStore) {
		m.unreachable["leader"] = true // the leader process is gone
		dbExpiry = m.until
	})
	crashed := time.Now()
	require.Eventually(t, standby.Ready, 3*testLeaseTTL, time.Millisecond)

	assert.False(t, time.Now().Before(dbExpiry), "the standby waits for the crashed term to expire")
	assert.Less(t, time.Since(crashed), testLeaseTTL+testLeaseRetry+250*time.Millisecond, "crash takeover is bounded by TTL plus the retry interval")
	assert.False(t, leader.Ready(), "the cut-off leader stepped down on its own")
}

func TestTwoWorkersNeverRunAJobAtTheSameTime(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	workers := []*Scheduler{
		leaseWorker(t, store, "worker-a", nil),
		leaseWorker(t, store, "worker-b", nil),
	}
	stops := make([]func(), len(workers))
	for i, worker := range workers {
		stops[i] = stopOnce(t, worker)
		worker.Start()
	}

	var active, overlaps atomic.Int32
	runs := make([]atomic.Int32, len(workers))
	tick := func(i int) {
		probeRun(workers[i], func(ctx context.Context) {
			if active.Add(1) > 1 {
				overlaps.Add(1)
			}
			defer active.Add(-1)
			runs[i].Add(1)
			select {
			case <-ctx.Done():
			case <-time.After(2 * time.Millisecond):
			}
		})
	}
	hammer := func(d time.Duration, live ...int) {
		var wg sync.WaitGroup
		deadline := time.Now().Add(d)
		for _, i := range live {
			wg.Go(func() {
				for time.Now().Before(deadline) {
					tick(i)
				}
			})
		}
		wg.Wait()
	}

	hammer(testLeaseTTL, 0, 1)
	first := 0
	if runs[1].Load() > 0 {
		first = 1
	}
	assert.Zero(t, runs[1-first].Load(), "the standby suppressed every run while the leader held the lease")
	stops[first]()
	hammer(testLeaseTTL, 1-first)

	assert.Zero(t, overlaps.Load(), "duplicate job execution is zero")
	assert.Positive(t, runs[1-first].Load(), "the standby took over the jobs")
}

func TestGracefulStopDrainsRunningJobsBeforeReleasingTheLease(t *testing.T) {
	t.Parallel()
	store := newMemoryLeaseStore()
	drained := &atomic.Bool{}
	store.drained = drained
	recorder := &leaseRecorder{}
	worker := leaseWorker(t, store, "leader", recorder)
	worker.Start()

	running := make(chan struct{})
	worker.wg.Add(1)
	go func() {
		defer worker.wg.Done()
		probeRun(worker, func(ctx context.Context) {
			close(running)
			<-ctx.Done()
			time.Sleep(2 * testLeaseRenew) // finishing work while the term stays renewed
			drained.Store(true)
		})
	}()
	<-running

	worker.Stop()

	assert.Equal(t, []bool{true}, store.releasedAt, "the lease is released only after the running job drained")
	holder, _ := store.current()
	assert.Empty(t, holder)
	require.Len(t, recorder.drains, 1)
	assert.GreaterOrEqual(t, recorder.drains[0], 2*testLeaseRenew)
}
