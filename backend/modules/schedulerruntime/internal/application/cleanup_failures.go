package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// A nightly cleanup job that fails for a school is retried with a growing
// delay per school and job (#6327): cleanupRetryAfter after the first
// failure, twice as long after every further one, at most
// cleanupRetryCeiling. The failure streak lives next to the last success in
// platform.cleanup_runs: a restart keeps it, a new day does not reset it, and
// only a success ends it. Schools and jobs keep separate streaks.
//
// The streak is written in the school's transaction, as a whole rather than
// one more, so a later write makes good a write that rolled back. A run that
// commits its successful steps (errCleanupIncomplete) or that could not read
// the cleanup switch writes it at once. A run that rolls its transaction back
// cannot: the process keeps the streak and writes it in the school's
// transaction of the job's next minute tick, before the due check. A restart
// before that write loses that one failure from the count.
//
// From cleanupAlertThreshold failures in a row on, every failed run logs
// exactly one cleanupFailingMessage line at ERROR. The first success after
// any failure logs cleanupRecoveredMessage with zero failures, so an alert
// ends even when the failure that raised it was never stored. The Grafana
// rule nightly-cleanup-failing alerts on the latest consecutive_failures of
// these two lines per school and job (monitoring/runbooks/nightly-cleanup.md).
const (
	cleanupRetryAfter       = time.Hour
	cleanupRetryCeiling     = 24 * time.Hour
	cleanupAlertThreshold   = 3
	cleanupFailingMessage   = "nightly cleanup failing repeatedly"
	cleanupRecoveredMessage = "nightly cleanup recovered"
	// cleanupRunFailedStep names a failure of the whole run rather than of
	// one of its steps: settings, transaction or ledger.
	cleanupRunFailedStep = "run"
)

// errCleanupNotCommitted reports a run whose steps all succeeded but whose
// tenant transaction did not commit.
var errCleanupNotCommitted = errors.New("data cleanup did not commit")

// cleanupRetryDelay is the wait after the given number of failed runs in a
// row: 1 h, 2 h, 4 h, 8 h, 16 h, then 24 h.
func cleanupRetryDelay(failures int) time.Duration {
	delay := cleanupRetryAfter
	for range max(failures-1, 0) {
		if delay >= cleanupRetryCeiling {
			break
		}
		delay *= 2
	}
	return min(delay, cleanupRetryCeiling)
}

// cleanupFailureStreak is the failure streak this process last saw for a
// school and job. written is false while the ledger may lack it.
type cleanupFailureStreak struct {
	failures int
	last     time.Time
	written  bool
}

// cleanupIncompleteError reports the steps of the nightly data cleanup that
// failed for a school while the others committed (#6288).
type cleanupIncompleteError struct {
	tenantID int64
	steps    []string
}

func (e *cleanupIncompleteError) Error() string {
	return fmt.Sprintf("%s for tenant %d: failed steps %s", errCleanupIncomplete, e.tenantID, strings.Join(e.steps, ","))
}

func (e *cleanupIncompleteError) Unwrap() error { return errCleanupIncomplete }

// failedCleanupSteps names the failed steps of a run for the alert.
func failedCleanupSteps(err error) string {
	var incomplete *cleanupIncompleteError
	if errors.As(err, &incomplete) && len(incomplete.steps) > 0 {
		return strings.Join(incomplete.steps, ",")
	}
	return cleanupRunFailedStep
}

// cleanupAttempt is one due run of a cleanup job for a school.
type cleanupAttempt struct {
	at time.Time
	// previous counts the failed runs in a row before this one.
	previous int
	// failure is the reported failure; nil while the run reported none.
	failure error
	// written reports that the run's transaction holds the streak.
	written bool
}

// cleanupAttempts collects the due runs of one job run per school. Tenant
// batches may report from several goroutines.
type cleanupAttempts struct {
	mu       sync.Mutex
	byTenant map[int64]*cleanupAttempt
}

func newCleanupAttempts() *cleanupAttempts {
	return &cleanupAttempts{byTenant: map[int64]*cleanupAttempt{}}
}

func (a *cleanupAttempts) start(tenantID int64, at time.Time, previous int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.byTenant[tenantID] = &cleanupAttempt{at: at, previous: previous}
}

func (a *cleanupAttempts) fail(tenantID int64, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if attempt, ok := a.byTenant[tenantID]; ok {
		attempt.failure = err
	}
}

func (a *cleanupAttempts) markWritten(tenantID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if attempt, ok := a.byTenant[tenantID]; ok {
		attempt.written = true
	}
}

func (a *cleanupAttempts) snapshot() map[int64]cleanupAttempt {
	a.mu.Lock()
	defer a.mu.Unlock()
	attempts := make(map[int64]cleanupAttempt, len(a.byTenant))
	for tenantID, attempt := range a.byTenant {
		attempts[tenantID] = *attempt
	}
	return attempts
}

// storeCleanupFailure writes the failed run of a transaction that commits
// anyway. It runs in a savepoint, so a failed write keeps the committed
// steps; the process then keeps the streak until the next tick writes it.
// It returns an error only when the transaction itself is lost.
func (s *Scheduler) storeCleanupFailure(ctx context.Context, tenantID int64, jobID string, attempts *cleanupAttempts) error {
	attempt, ok := attempts.snapshot()[tenantID]
	if s.cleanupRuns == nil || !ok {
		return nil
	}
	usable, err := s.inCleanupSavepoint(ctx, func(recordCtx context.Context) error {
		err := s.cleanupRuns.RecordCleanupFailed(recordCtx, jobID, attempt.previous+1, attempt.at)
		if err == nil {
			attempts.markWritten(tenantID)
		}
		return err
	})
	if err != nil {
		s.getLogger().Error("record failed cleanup run failed",
			slog.String("job_id", jobID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
	}
	if !usable {
		return err
	}
	return nil
}

// storePendingCleanupFailure writes the last failure the process kept for
// the school and job into the transaction in ctx, before the due check reads
// the ledger. The process marks it written once that transaction committed.
// The write has a savepoint of its own: a failed write leaves the streak with
// the process and does not keep the job from running.
func (s *Scheduler) storePendingCleanupFailure(ctx context.Context, jobID string, tenantID int64) error {
	key := cleanupAttemptKey{jobID: jobID, tenantID: tenantID}
	value, ok := s.cleanupFailures.Load(key)
	if !ok {
		return nil
	}
	pending, ok := value.(cleanupFailureStreak)
	if !ok || pending.written {
		return nil
	}
	usable, err := s.inCleanupSavepoint(ctx, func(recordCtx context.Context) error {
		return s.cleanupRuns.RecordCleanupFailed(recordCtx, jobID, pending.failures, pending.last)
	})
	if !usable {
		return fmt.Errorf("record failed cleanup run of %s: %w", jobID, err)
	}
	if err != nil {
		// The process keeps the streak and tries again next tick; the run
		// itself goes on with it.
		s.getLogger().Error("record failed cleanup run failed",
			slog.String("job_id", jobID),
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	written := pending
	written.written = true
	s.afterCommit(ctx, func() {
		s.cleanupFailures.CompareAndSwap(key, pending, written)
	})
	return nil
}

// settleCleanupAttempts reports every due run once its school's transaction
// ended. A run succeeded only when it reported no failure and its
// transaction committed. Every other run extends the streak the process
// keeps; it is still to be written unless the run's committed transaction
// wrote it.
func (s *Scheduler) settleCleanupAttempts(jobID string, attempts *cleanupAttempts, completed []int64) {
	committed := make(map[int64]bool, len(completed))
	for _, tenantID := range completed {
		committed[tenantID] = true
	}
	for tenantID, attempt := range attempts.snapshot() {
		if attempt.failure == nil && committed[tenantID] {
			s.noteCleanupRecovered(jobID, tenantID, attempt)
			continue
		}
		if attempt.failure == nil {
			attempt.failure = errCleanupNotCommitted
		}
		streak := cleanupFailureStreak{failures: attempt.previous + 1, last: attempt.at, written: attempt.written && committed[tenantID]}
		s.cleanupFailures.Store(cleanupAttemptKey{jobID: jobID, tenantID: tenantID}, streak)
		s.alertCleanupFailure(jobID, tenantID, attempt, streak.failures)
	}
}

func (s *Scheduler) alertCleanupFailure(jobID string, tenantID int64, attempt cleanupAttempt, failures int) {
	if failures < cleanupAlertThreshold {
		return
	}
	s.getLogger().Error(cleanupFailingMessage,
		slog.String("job_id", jobID),
		slog.Int64("tenant_id", tenantID),
		slog.Int("consecutive_failures", failures),
		slog.String("failed_steps", failedCleanupSteps(attempt.failure)),
		slog.Time("next_attempt_at", attempt.at.Add(cleanupRetryDelay(failures))),
		slog.String("error", attempt.failure.Error()),
	)
}

func (s *Scheduler) noteCleanupRecovered(jobID string, tenantID int64, attempt cleanupAttempt) {
	if attempt.previous == 0 {
		return
	}
	s.getLogger().Info(cleanupRecoveredMessage,
		slog.String("job_id", jobID),
		slog.Int64("tenant_id", tenantID),
		slog.Int("consecutive_failures", 0),
		slog.Int("previous_failures", attempt.previous),
	)
}
