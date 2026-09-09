package schedule_test

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Local #2697 capability evidence, including the real retained Timetable
// completion provider. Fixture work and verification are outside the timer.
// HTTP contracts have separate kiosk route tests; this is not TCP/staging.
func TestSessionEndRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := services.NewActiveTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	var postgres string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &postgres))
	counter := testpkg.CaptureQueriesForContext(t, db)
	measuredCtx := counter.Context(ctx)
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	for _, operation := range []string{"end-mirrored-session", "reject-already-ended"} {
		t.Run(operation, func(t *testing.T) {
			var samples []testpkg.RuntimeCheckpointSample
			var durations []float64
			var stopSampling func() testpkg.RuntimeCheckpointLockSamples
			var deadlocksBefore int64
			for iteration := range 35 {
				label := fmt.Sprintf("%s-%d", operation, iteration)
				activity := testpkg.CreateTestActivityGroup(t, db, label)
				room := testpkg.CreateTestRoom(t, db, label)
				group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
				staff := testpkg.CreateTestStaff(t, db, "Runtime", label)
				testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
				present := testpkg.CreateTestStudent(t, db, "Present", label, "1a")
				expected := testpkg.CreateTestStudent(t, db, "Expected", label, "1a")
				checkedIn := time.Now().Add(-time.Hour)
				visit := testpkg.CreateTestVisit(t, db, present.ID, group.ID, checkedIn, nil)
				instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
					Status: "active", ActivityGroupID: &activity.ID, ActiveGroupID: &group.ID, IsSpontaneous: true,
				})
				assignment := testpkg.CreateTestInstanceStudent(t, db, instance.ID, present.ID, "present", testpkg.InstanceStudentOpts{CheckedInAt: &checkedIn})
				testpkg.CreateTestInstanceStudent(t, db, instance.ID, expected.ID, "expected")
				if operation == "reject-already-ended" {
					_, err := module.SessionEnd.EndSession(ctx, group.ID)
					require.NoError(t, err)
				}
				if iteration == 5 {
					deadlocksBefore = deadlocks()
					stopSampling = testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
						var waiting int
						err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
						return waiting, err
					})
					t.Cleanup(func() { _ = stopSampling() })
				}
				counter.Reset()
				before, started := db.Stats(), time.Now()
				result, err := module.SessionEnd.EndSession(measuredCtx, group.ID)
				elapsed, after := time.Since(started), db.Stats()
				if operation == "reject-already-ended" {
					require.EqualError(t, err, "active group session already ended")
					require.Zero(t, counter.WriteRows())
				} else {
					require.NoError(t, err)
					require.Equal(t, 1, result.StudentsCheckedOut)
					require.Equal(t, 1, result.SupervisorsEnded)
					require.NotNil(t, result.MirroredInstanceID)
					require.Equal(t, instance.ID, *result.MirroredInstanceID)
				}
				require.Equal(t, "completed", testpkg.InstanceStatus(t, db, instance.ID))
				require.NotNil(t, testpkg.VisitExitTime(t, db, visit.ID))
				require.NotNil(t, testpkg.InstanceStudentByID(t, db, assignment.ID).CheckedOutAt)
				if iteration >= 5 {
					duration := float64(elapsed) / float64(time.Millisecond)
					writes := counter.WriteRows()
					rows, statements := counter.Rows()
					samples = append(samples, testpkg.RuntimeCheckpointSample{
						DurationMS: duration, Queries: counter.Total(), WriteRowsAffected: &writes,
						RowsAffected: rows, StatementsWithRows: statements,
						PoolWaitCount: after.WaitCount - before.WaitCount,
						PoolWaitMS:    float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
					})
					durations = append(durations, duration)
				}
			}
			locks := stopSampling()
			require.Empty(t, locks.Error)
			slices.Sort(durations)
			report, err := json.Marshal(map[string]any{
				"operation": operation, "go": runtime.Version(), "postgres": postgres,
				"workload":    "session-end-v1: one mirrored group, one supervisor, one present and one expected student",
				"concurrency": 1, "warmup": 5, "samples": samples,
				"p50_ms": durations[14], "p95_ms": durations[28], "unexpected_errors": 0,
				"lock_samples": locks, "deadlocks": deadlocks() - deadlocksBefore,
			})
			require.NoError(t, err)
			t.Logf("session-end-runtime: %s", report)
		})
	}
}
