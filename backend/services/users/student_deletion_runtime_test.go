package users_test

import (
	"context"
	"encoding/json"
	"runtime"
	"slices"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Local cutover evidence, not a production observation or a new checkpoint.
func TestScheduledCheckoutPreviewRuntime(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Scheduled", "Preview", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Scheduled", "Preview")
	service := newStudentDeletionTestService(db, nil, nil)
	counter := testpkg.CaptureQueriesForContext(t, db)
	measuredCtx := counter.Context(ctx)
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	t.Logf("scheduled-preview environment: go=%s postgres=%s concurrency=1 warmup=5 samples_per_flow=30", runtime.Version(), version)
	for _, count := range []int{0, 1} {
		if count == 1 {
			testpkg.CreateTestScheduledCheckout(t, db, student.ID, staff.ID, time.Now().Add(time.Hour))
		}
		var samples []testpkg.RuntimeCheckpointSample
		var durations []float64
		stop := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
			var waiting int
			err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
			return waiting, err
		})
		t.Cleanup(func() { _ = stop() })
		for iteration := range 35 {
			counter.Reset()
			before, started := db.Stats(), time.Now()
			preview, err := service.Preview(measuredCtx, student.ID)
			elapsed, after := time.Since(started), db.Stats()
			require.NoError(t, err)
			require.Equal(t, count, preview.Counts.AttendanceRecords)
			require.NotEmpty(t, preview.Fingerprint)
			if iteration >= 5 {
				duration := float64(elapsed) / float64(time.Millisecond)
				rows, statements := counter.Rows()
				samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: duration, Queries: counter.Total(), RowsAffected: rows, StatementsWithRows: statements, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
				durations = append(durations, duration)
			}
		}
		locks := stop()
		require.Empty(t, locks.Error)
		slices.Sort(durations)
		report, err := json.Marshal(map[string]any{"scheduled_rows": count, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28], "unexpected_errors": 0, "lock_samples": locks})
		require.NoError(t, err)
		t.Logf("scheduled-preview-runtime: %s", report)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err := service.Preview(cancelled, student.ID)
	require.ErrorIs(t, err, context.Canceled)
}
