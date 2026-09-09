package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Bounded owner-capability evidence for #2690, not an HTTP or kiosk benchmark.
func TestWorkforceOwnerRuntime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	config := testpkg.ConfigRuntime(db)
	var measuring, observing bool
	var operations []map[string]any
	capability, err := New(Dependencies{DB: db, AssignedStaffIDs: config.AssignedStaffIDs, RebaseStaffAnchor: config.RebaseAssignedStaffAnchor, Observe: func(event Observation) {
		if observing {
			operations = append(operations, map[string]any{"operation": event.Operation, "error_code": workforce.ErrorCode(event.Err), "queries": event.Stats.Queries, "rows": event.Stats.Rows, "duration_ms": float64(event.Duration) / float64(time.Millisecond)})
		}
	}})
	require.NoError(t, err)
	counter := testpkg.CaptureQueriesForContext(t, db)
	measuredCtx := counter.Context(ctx)
	var commits, rollbacks, retries int
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	t.Logf("workforce-runtime environment: go=%s postgres=%s concurrency=1 warmup=5 samples_per_scenario=30", runtime.Version(), version)
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	deadlocksBefore := deadlocks()
	stop := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
		var waiting int
		err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
		return waiting, err
	})
	t.Cleanup(func() { _ = stop() })
	samples := make(map[string][]testpkg.RuntimeCheckpointSample)
	measure := func(name string, expected error, run func(context.Context) error) {
		counter.Reset()
		observing = measuring
		before, started := db.Stats(), time.Now()
		sampleCtx, evidence := testpkg.CaptureUnitOfWorkEvidence(measuredCtx)
		err := run(sampleCtx)
		elapsed, after := time.Since(started), db.Stats()
		observing = false
		if expected == nil {
			require.NoError(t, err, name)
		} else {
			require.ErrorIs(t, err, expected, name)
		}
		if measuring {
			for _, event := range evidence() {
				if event.Kind != "transaction" {
					continue
				}
				retries += event.Retries
				if event.Result == "commit" {
					commits++
				}
				if event.Result == "rollback" {
					rollbacks++
				}
			}
			rows, statements := counter.Rows()
			samples[name] = append(samples[name], testpkg.RuntimeCheckpointSample{DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(), RowsAffected: rows, StatementsWithRows: statements, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
		}
	}
	abort := errors.New("injected failure after workforce writes")
	day := timezone.NewDate(2026, 5, 4)
	start := day.BerlinMidnight().Add(8 * time.Hour)
	for iteration := range 35 {
		measuring = iteration >= 5
		staff := testpkg.CreateTestStaff(t, db, "Runtime", "Workforce")
		var session workforce.WorkSession
		workday := func(ctx context.Context, fail bool) error {
			return testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				var err error
				session, err = capability.CreateWorkSession(txCtx, testWorkSession(staff.ID, day, start))
				if err != nil {
					return err
				}
				pause, err := capability.CreateWorkSessionBreak(txCtx, workforce.WorkSessionBreak{SessionID: session.ID, StartedAt: start.Add(4 * time.Hour)})
				if err != nil {
					return err
				}
				_, err = capability.EndWorkSessionBreak(txCtx, pause.ID, start.Add(4*time.Hour+30*time.Minute), 30)
				if err != nil {
					return err
				}
				_, err = capability.SetWorkSessionBreakMinutes(txCtx, session.ID, 30)
				if err != nil {
					return err
				}
				closed, err := capability.CloseWorkSession(txCtx, session.ID, start.Add(8*time.Hour), false)
				require.True(t, closed)
				if err != nil {
					return err
				}
				if fail {
					return abort
				}
				return nil
			})
		}
		measure("workday_rollback", abort, func(ctx context.Context) error { return workday(ctx, true) })
		_, err = capability.FindWorkSession(ctx, session.ID)
		require.ErrorIs(t, err, workforce.ErrWorkSessionNotFound)
		measure("workday_retry", nil, func(ctx context.Context) error { return workday(ctx, false) })
		measure("workday_read", nil, func(ctx context.Context) error {
			sessions, err := capability.ListWorkSessions(ctx, workforce.WorkSessionFilter{StaffIDs: []int64{staff.ID}})
			if err != nil {
				return err
			}
			require.Len(t, sessions, 1)
			require.Equal(t, 30, sessions[0].BreakMinutes)
			pauses, err := capability.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionID: session.ID})
			require.Len(t, pauses, 1)
			return err
		})
		var document workforce.StaffDocument
		records := func(ctx context.Context, fail bool) error {
			return testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				if _, err := capability.CreateStaffMasterData(txCtx, workforce.StaffMasterData{StaffID: staff.ID}); err != nil {
					return err
				}
				if _, err := capability.CreateStaffFinancialData(txCtx, workforce.StaffFinancialData{StaffID: staff.ID}); err != nil {
					return err
				}
				if _, err := capability.ReplaceStaffQualifications(txCtx, staff.ID, []workforce.StaffQualification{{Name: "Runtime qualification"}}); err != nil {
					return err
				}
				if _, err := capability.CreateStaffBalanceAdjustment(txCtx, workforce.StaffBalanceAdjustment{StaffID: staff.ID, Type: workforce.BalanceAdjustmentTypeOpening, EffectiveDate: day.String(), DecidedBy: staff.ID}); err != nil {
					return err
				}
				if _, err := capability.CreateStaffVacationOpening(txCtx, workforce.StaffVacationOpening{StaffID: staff.ID, Year: 2026, EffectiveDate: day.String(), EnteredRemainingDays: 30, DecidedBy: staff.ID}); err != nil {
					return err
				}
				if err := capability.UpsertStaffVacationQuota(txCtx, workforce.StaffVacationQuota{StaffID: staff.ID, Year: 2026, EntitledDays: 30}); err != nil {
					return err
				}
				var err error
				document, err = capability.CreateStaffDocument(txCtx, testDocument(staff.ID, workforce.StaffDocumentCategorySonstiges, fmt.Sprintf("runtime-document-%d", iteration)))
				if err != nil {
					return err
				}
				if fail {
					return abort
				}
				return nil
			})
		}
		measure("records_rollback", abort, func(ctx context.Context) error { return records(ctx, true) })
		measure("records_retry", nil, func(ctx context.Context) error { return records(ctx, false) })
		cleanup := func(ctx context.Context, fail bool) error {
			return testpkg.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				if _, err := capability.SoftDeleteStaffDocument(txCtx, document.ID, staff.ID, start); err != nil {
					return err
				}
				if err := capability.QueueStaffDocumentFileCleanup(txCtx, workforce.StaffDocumentFileCleanup{StaffID: staff.ID, FilenameStored: document.FilenameStored, RetryAfter: start}); err != nil {
					return err
				}
				if fail {
					return abort
				}
				return nil
			})
		}
		measure("document_cleanup_rollback", abort, func(ctx context.Context) error { return cleanup(ctx, true) })
		intents, err := capability.ListQueuedStaffDocumentFileCleanups(ctx, staff.ID)
		require.NoError(t, err)
		require.Empty(t, intents)
		measure("document_cleanup_retry", nil, func(ctx context.Context) error { return cleanup(ctx, false) })
		intents, err = capability.ListQueuedStaffDocumentFileCleanups(ctx, staff.ID)
		require.NoError(t, err)
		require.Len(t, intents, 1)
	}
	locks := stop()
	require.Empty(t, locks.Error)
	percentiles := make(map[string]map[string]float64)
	for name, values := range samples {
		durations := make([]float64, 0, len(values))
		for _, value := range values {
			durations = append(durations, value.DurationMS)
		}
		slices.Sort(durations)
		percentiles[name] = map[string]float64{"p50_ms": durations[14], "p95_ms": durations[28]}
	}
	report, err := json.Marshal(map[string]any{"samples": samples, "percentiles": percentiles, "operations": operations, "commits": commits, "rollbacks": rollbacks, "serialization_retries": retries, "injected_failures": 90, "unexpected_errors": 0, "deadlocks": deadlocks() - deadlocksBefore, "lock_samples": locks})
	require.NoError(t, err)
	t.Logf("workforce-runtime: %s", report)
}
