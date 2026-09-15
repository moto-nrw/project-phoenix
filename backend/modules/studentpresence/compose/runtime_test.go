package compose_test

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Local flow evidence for #2691. This does not replace the accepted runtime
// checkpoint or claim a staging/production observation window.
func TestStudentPresenceMigrationRuntime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	var databaseVersion string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &databaseVersion))
	t.Logf("student-presence-runtime environment: go=%s postgres=%s concurrency=1 warmup=5 samples_per_flow=30", runtime.Version(), databaseVersion)
	student := testpkg.CreateTestStudent(t, db, "Runtime", "Presence", "3a")
	device := testpkg.CreateTestDevice(t, db, "presence-runtime")
	activity := testpkg.CreateTestActivityGroup(t, db, "Presence runtime")
	room := testpkg.CreateTestRoom(t, db, "Presence runtime")
	group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	day := testpkg.TodayDate()
	at := day.BerlinMidnight().Add(8 * time.Hour)
	var observations []compose.Observation
	var measuring bool
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(event compose.Observation) {
		if measuring {
			observations = append(observations, event)
		}
	}})
	require.NoError(t, err)
	counter := testpkg.CaptureQueriesForContext(t, db)
	measuredCtx := counter.Context(ctx)
	var commits, rollbacks, retries int
	measuredCtx = tenant.WithUnitOfWorkObserver(measuredCtx, func(event tenant.UnitOfWorkEvent) {
		if !measuring || event.Kind != tenant.UnitOfWorkTransaction {
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
	abort := errors.New("injected presence checkout failure")
	for _, flow := range []string{"checkin-checkout", "rollback-after-visit-close", "rollback-after-attendance-close"} {
		t.Run(flow, func(t *testing.T) {
			var samples []testpkg.RuntimeCheckpointSample
			var durations []float64
			var duplicateConflicts, injectedFailures int
			var stopSampling func() testpkg.RuntimeCheckpointLockSamples
			var deadlocksBefore int64
			for iteration := range 35 {
				if iteration == 5 {
					commits, rollbacks, retries = 0, 0, 0
					observations = nil
					deadlocksBefore = deadlocks()
					stopSampling = testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
						var waiting int
						err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
						return waiting, err
					})
					t.Cleanup(func() { _ = stopSampling() })
				}
				counter.Reset()
				measuring = iteration >= 5
				before, started := db.Stats(), time.Now()
				var attendance studentpresence.Attendance
				var visit studentpresence.Visit
				err := tenant.WithinCurrentTenant(measuredCtx, func(txCtx context.Context) error {
					var inserted bool
					var err error
					attendance, inserted, err = module.EnsureAttendance(txCtx, studentpresence.Attendance{StudentID: student.ID, Date: day.String(), CheckInTime: at, DeviceID: device.ID})
					if err != nil {
						return err
					}
					require.True(t, inserted)
					_, inserted, err = module.EnsureAttendance(txCtx, studentpresence.Attendance{StudentID: student.ID, Date: day.String(), CheckInTime: at, DeviceID: device.ID})
					if err != nil {
						return err
					}
					require.False(t, inserted)
					visit, err = module.RecordVisit(txCtx, studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: at})
					return err
				})
				require.NoError(t, err)
				closeStay := func(txCtx context.Context, fail bool) error {
					visits, err := module.CloseVisits(txCtx, []int64{visit.ID}, at.Add(time.Hour))
					if err != nil {
						return err
					}
					require.Len(t, visits, 1)
					if fail && flow == "rollback-after-visit-close" {
						return abort
					}
					rows, err := module.CloseAttendance(txCtx, studentpresence.AttendanceCheckout{StudentIDs: []int64{student.ID}, Date: day.String(), At: at.Add(time.Hour), DeviceID: device.ID})
					if err != nil {
						return err
					}
					require.Len(t, rows, 1)
					if fail {
						return abort
					}
					return nil
				}
				if flow != "checkin-checkout" {
					err = tenant.WithinCurrentTenant(measuredCtx, func(txCtx context.Context) error { return closeStay(txCtx, true) })
					require.ErrorIs(t, err, abort)
					storedVisit, err := module.FindVisit(measuredCtx, visit.ID)
					require.NoError(t, err)
					require.Nil(t, storedVisit.ExitTime)
					storedAttendance, err := module.FindAttendance(measuredCtx, attendance.ID)
					require.NoError(t, err)
					require.Nil(t, storedAttendance.CheckOutTime)
				}
				require.NoError(t, tenant.WithinCurrentTenant(measuredCtx, func(txCtx context.Context) error { return closeStay(txCtx, false) }))
				statuses, err := module.ListSchoolStatuses(measuredCtx, []int64{student.ID}, day.String())
				require.NoError(t, err)
				require.Len(t, statuses, 1)
				require.Equal(t, "checked_out", statuses[0].Status)
				elapsed, after := time.Since(started), db.Stats()
				measuring = false
				if iteration >= 5 {
					writes := counter.WriteRows()
					duration := float64(elapsed) / float64(time.Millisecond)
					samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: duration, Queries: counter.Total(), WriteRowsAffected: &writes, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
					durations = append(durations, duration)
					duplicateConflicts++
					if flow != "checkin-checkout" {
						injectedFailures++
					}
				}
				// Fixture cleanup is outside the measured query and transaction context.
				require.NoError(t, module.DeleteVisit(ctx, visit.ID))
				require.NoError(t, module.DeleteAttendance(ctx, attendance.ID))
			}
			locks := stopSampling()
			require.Empty(t, locks.Error)
			slices.Sort(durations)
			report, err := json.Marshal(map[string]any{
				"flow": flow, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28],
				"commits": commits, "rollbacks": rollbacks, "transaction_retries": retries,
				"injected_failures": injectedFailures, "unexpected_errors": 0,
				"duplicate_prevention_conflicts": duplicateConflicts, "lock_samples": locks,
				"deadlocks": deadlocks() - deadlocksBefore, "operations": observations,
			})
			require.NoError(t, err)
			t.Logf("student-presence-runtime: %s", report)
		})
	}
}
