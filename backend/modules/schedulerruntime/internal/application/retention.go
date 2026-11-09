package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// cleanupDue decides whether a nightly cleanup runs now, on the Berlin
// calendar: at the configured minute unless it already succeeded that day,
// and at any minute once a scheduled run was missed, that is when the last
// success lies on a day before the latest scheduled time that has passed. A
// school that never succeeded waits for its configured minute, so a release
// does not start every first run at once. Moving the time later in the day
// therefore runs at the new time from the next day on instead of catching up
// at the old one.
func cleanupDue(now time.Time, cleanupTime string, last time.Time, hasLast bool) (due, catchUp bool) {
	now = now.In(calendar.Berlin)
	if hasLast {
		last = last.In(calendar.Berlin)
	}
	if timeMatchesAt(cleanupTime, now) {
		return !hasLast || !sameLocalDay(last, now), false
	}
	slot, ok := latestScheduledRun(cleanupTime, now)
	if hasLast && ok && last.Before(slot) && !sameLocalDay(last, slot) {
		return true, true
	}
	return false, false
}

// latestScheduledRun is the most recent configured cleanup time at or before
// now, today or yesterday.
func latestScheduledRun(cleanupTime string, now time.Time) (time.Time, bool) {
	clock, err := time.Parse("15:04", cleanupTime)
	if err != nil {
		return time.Time{}, false
	}
	slot := time.Date(now.Year(), now.Month(), now.Day(), clock.Hour(), clock.Minute(), 0, 0, now.Location())
	if slot.After(now) {
		slot = slot.AddDate(0, 0, -1)
	}
	return slot, true
}

func sameLocalDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func (s *Scheduler) cleanupNow() time.Time {
	if s.cleanupClock != nil {
		return s.cleanupClock()
	}
	return time.Now()
}

// cleanupRunState is the stored state of one cleanup job of a school: its
// last success, zero while it never succeeded, and the failed runs in a row
// since with the last of them.
type cleanupRunState struct {
	LastSuccess time.Time
	Failures    int
	LastFailure time.Time
}

type cleanupAttemptKey struct {
	jobID    string
	tenantID int64
}

// dailyCleanupDue reads the stored state of the job for the school and
// decides whether it runs now. After failed runs it waits the backoff of the
// failure streak (#6327), also at the configured minute and across days.
func (s *Scheduler) dailyCleanupDue(ctx context.Context, dayCache *sync.Map, jobID string, tenantID int64, cleanupTime string, now time.Time) (cleanupRunState, bool, error) {
	state, err := s.cleanupRunState(ctx, dayCache, jobID, tenantID)
	if err != nil {
		return state, false, err
	}
	hasLast := !state.LastSuccess.IsZero()
	due, catchUp := cleanupDue(now, cleanupTime, state.LastSuccess, hasLast)
	// A school without a success normally waits for its first configured
	// minute. After that first run failed, the failure streak distinguishes
	// it from an untouched school and enables the retry.
	if !due && !hasLast && state.Failures > 0 {
		due, catchUp = true, true
	}
	if !due {
		return state, false, nil
	}
	if state.Failures > 0 && now.Sub(state.LastFailure) < cleanupRetryDelay(state.Failures) {
		return state, false, nil
	}
	if catchUp {
		s.getLogger().Info("catching up missed data cleanup",
			slog.String("job_id", jobID),
			slog.Int64("tenant_id", tenantID),
			slog.Time("last_success", state.LastSuccess),
			slog.Int("consecutive_failures", state.Failures),
		)
	}
	return state, true, nil
}

// cleanupRunState reads the last success and the failure streak of the job
// for the school, after writing a failure the process still keeps. Without a
// ledger the day cache holds the last success and the process the streak. A
// streak the process saw later than the ledger wins, so a failure whose
// write is still pending or was lost spaces the next retry all the same.
func (s *Scheduler) cleanupRunState(ctx context.Context, dayCache *sync.Map, jobID string, tenantID int64) (cleanupRunState, error) {
	var state cleanupRunState
	if s.cleanupRuns == nil {
		if value, ok := dayCache.Load(tenantID); ok {
			state.LastSuccess, _ = value.(time.Time)
		}
	} else {
		if err := s.storePendingCleanupFailure(ctx, jobID, tenantID); err != nil {
			return cleanupRunState{}, err
		}
		var err error
		state.LastSuccess, state.Failures, state.LastFailure, err = s.cleanupRuns.CleanupRun(ctx, jobID)
		if err != nil {
			return cleanupRunState{}, fmt.Errorf("read cleanup run of %s: %w", jobID, err)
		}
	}
	if value, ok := s.cleanupFailures.Load(cleanupAttemptKey{jobID: jobID, tenantID: tenantID}); ok {
		if streak, ok := value.(cleanupFailureStreak); ok && streak.last.After(state.LastFailure) {
			state.Failures, state.LastFailure = streak.failures, streak.last
		}
	}
	return state, nil
}

// recordCleanupSuccess stores the success in the tenant transaction; the
// process cache follows only after commit.
func (s *Scheduler) recordCleanupSuccess(ctx context.Context, dayCache *sync.Map, jobID string, tenantID int64, now time.Time) error {
	if s.cleanupRuns != nil {
		if err := s.cleanupRuns.RecordCleanupSucceeded(ctx, jobID, now); err != nil {
			return fmt.Errorf("record cleanup of %s: %w", jobID, err)
		}
	}
	s.afterCommit(ctx, func() {
		s.cleanupFailures.Delete(cleanupAttemptKey{jobID: jobID, tenantID: tenantID})
	})
	s.markRunAtAfterCommit(ctx, dayCache, tenantID, now)
	return nil
}

// RetentionRun runs every owner's deletion for the school of the tenant
// transaction in ctx: the shared retention capability of the nightly cleanup
// (#3950). The Worker runs it inside the school's cleanup transaction.
type RetentionRun func(ctx context.Context, tenantID int64) error

// newRetentionRun builds the retention run over the owners' steps. Each step
// that deletes something writes one deletion-log entry, unless it logs each
// deleted subject itself (LogsSubjects); a failing step or log
// entry aborts the run. The nightly cleanup runs it in its own savepoint, so
// its deletions roll back together with their log entries and the run is
// caught up later (#6288).
func newRetentionRun(steps []ports.RetentionStep, log ports.RetentionLog, logger *slog.Logger) (RetentionRun, error) {
	if len(steps) > 0 && log == nil {
		return nil, errors.New("retention run: a deletion log is required for retention steps")
	}
	for _, step := range steps {
		if step.Kind == "" || step.Run == nil {
			return nil, errors.New("retention run: every step needs a kind and a run")
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	steps = append([]ports.RetentionStep(nil), steps...)
	return func(ctx context.Context, tenantID int64) error {
		return runRetention(ctx, tenantID, steps, log, logger)
	}, nil
}

func runRetention(ctx context.Context, tenantID int64, steps []ports.RetentionStep, log ports.RetentionLog, logger *slog.Logger) error {
	for _, step := range steps {
		outcome, err := step.Run(ctx)
		if err != nil {
			logger.Error("retention step failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("data_kind", step.Kind),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("retention step %s: %w", step.Kind, err)
		}
		if outcome.Deleted == 0 {
			continue
		}
		if !step.LogsSubjects {
			if err := log(ctx, step.Kind, outcome); err != nil {
				return fmt.Errorf("log retention step %s: %w", step.Kind, err)
			}
		}
		logger.Info("retention step completed",
			slog.Int64("tenant_id", tenantID),
			slog.String("data_kind", step.Kind),
			slog.Int("records_deleted", outcome.Deleted),
			slog.Int("retention_days", outcome.RetentionDays),
		)
	}
	return nil
}
