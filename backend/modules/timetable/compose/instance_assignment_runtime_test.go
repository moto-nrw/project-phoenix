package compose

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// This is flow-specific evidence for #2685, not a replacement for the accepted
// cross-module runtime checkpoint. Each sample starts with one planned row.
func TestInstanceAssignmentMigrationRuntime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, ctx := buildModule(t, db), testpkg.Ctx(t)
	fixture := newOwnedActivityInstanceFixture(t, db, "assignment-runtime")
	instance := createOwnedActivityInstance(t, module, ctx, fixture, "2027-11-02", "08:00:00", "Runtime")
	student := testpkg.CreateTestStudent(t, db, "Runtime", "Assignment", "3a")
	row := createOwnedInstanceStudent(t, module, ctx, instance.ID, student.ID, timetable.InstanceAttendanceExpected)
	ids, snapshot := []int64{student.ID}, []timetable.InstanceStudent{row}
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx = counter.Context(ctx)
	var commits, rollbacks, retries int
	ctx = tenant.WithUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) {
		if event.Kind != tenant.UnitOfWorkTransaction {
			return
		}
		retries += event.Retries
		if event.Result == tenant.UnitOfWorkCommitted {
			commits++
		}
		if event.Result == tenant.UnitOfWorkRolledBack {
			rollbacks++
		}
	})
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(context.Background(), &count))
		return count
	}
	abort := errors.New("injected failure after assignment deletion")
	for _, name := range []string{"care-exit-remove-restore-duplicate", "delete-rollback-retry"} {
		t.Run(name, func(t *testing.T) {
			var samples []testpkg.RuntimeCheckpointSample
			var duplicateRowsSkipped, injectedFailures int
			var stopSampling func() testpkg.RuntimeCheckpointLockSamples
			var deadlocksBefore int64
			for iteration := range 35 {
				if iteration == 5 {
					commits, rollbacks, retries = 0, 0, 0
					deadlocksBefore = deadlocks()
					stopSampling = testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
						var waiting int
						err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
						return waiting, err
					})
					t.Cleanup(func() { _ = stopSampling() })
				}
				counter.Reset()
				before := db.Stats()
				started := time.Now()
				if name == "care-exit-remove-restore-duplicate" {
					err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
						require.NoError(t, module.LockPlannedStudentAssignmentsAfter(txCtx, ids, "2027-11-01"))
						removed, err := module.RemovePlannedStudentAssignmentsAfter(txCtx, ids, "2027-11-01")
						if err != nil {
							return err
						}
						require.Len(t, removed, 1)
						count, err := module.RestoreCareExitStudentAssignments(txCtx, ids, nil, nil, nil, removed)
						if err != nil {
							return err
						}
						require.EqualValues(t, 1, count)
						count, err = module.RestoreCareExitStudentAssignments(txCtx, ids, nil, nil, nil, removed)
						require.Zero(t, count)
						return err
					})
					require.NoError(t, err)
					if iteration >= 5 {
						duplicateRowsSkipped++
					}
				} else {
					err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
						count, err := module.DeleteStudentAssignments(txCtx, student.ID)
						if err != nil {
							return err
						}
						require.EqualValues(t, 1, count)
						return abort
					})
					require.ErrorIs(t, err, abort)
					count, err := module.CountStudentAssignments(ctx, student.ID)
					require.NoError(t, err)
					require.Equal(t, 1, count)
					deleted, err := module.DeleteStudentAssignments(ctx, student.ID)
					require.NoError(t, err)
					require.EqualValues(t, 1, deleted)
					if iteration >= 5 {
						injectedFailures++
					}
				}
				elapsed, after := time.Since(started), db.Stats()
				if iteration >= 5 {
					rows, statements := counter.Rows()
					samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(), RowsAffected: rows, StatementsWithRows: statements, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
				}
				// Reset outside the measured region and transaction counters.
				if name == "delete-rollback-retry" {
					_, err := module.RestoreCareExitStudentAssignments(testpkg.Ctx(t), ids, nil, nil, nil, snapshot)
					require.NoError(t, err)
				}
			}
			locks := stopSampling()
			require.Empty(t, locks.Error)
			report, err := json.Marshal(map[string]any{"flow": name, "samples": samples, "commits": commits, "rollbacks": rollbacks, "transaction_retries": retries, "injected_failures": injectedFailures, "unexpected_errors": 0, "duplicate_rows_skipped": duplicateRowsSkipped, "lock_samples": locks, "deadlocks": deadlocks() - deadlocksBefore})
			require.NoError(t, err)
			t.Logf("assignment-runtime: %s", report)
		})
	}
}
