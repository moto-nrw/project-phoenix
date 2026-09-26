package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// This identical local HTTP fixture runs on the pre-cutover reader too.
// It measures the real handler/middleware graph, not production throughput.
func TestOGSGroupLiveRuntime(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	ctx := testpkg.Ctx(t)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "LiveRuntime", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "LiveRuntimeGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	room := testpkg.CreateTestRoom(t, tc.db, "LiveRuntimeRoom")
	_, err := tc.db.NewUpdate().Table("education.groups").Set("room_id = ?", room.ID).Where("id = ?", group.ID).Exec(ctx)
	require.NoError(t, err)
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "LiveRuntimeActivity")
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	device := testpkg.CreateTestDevice(t, tc.db, "LiveRuntimeDevice")
	var size int
	addStudents := func(target int) {
		for size < target {
			student := testpkg.CreateTestStudent(t, tc.db, "LiveRuntime", fmt.Sprintf("Child%02d", size), "LR1")
			testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
			now := fixedCalendarClock()
			testpkg.CreateTestAttendanceForDate(t, tc.db, student.ID, teacher.Staff.ID, device.ID, timezone.DateFromTime(now), now.Add(-time.Hour), nil)
			testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, now.Add(-time.Hour), nil)
			size++
		}
	}
	counter := testpkg.CaptureQueries(t, tc.db)
	var version string
	require.NoError(t, tc.db.NewRaw("SHOW server_version").Scan(ctx, &version))
	t.Logf("live-runtime environment: go=%s postgres=%s concurrency=1 warmup=5 samples=30", runtime.Version(), version)
	samples := make(map[string][]testpkg.RuntimeCheckpointSample)
	plans := make(map[string][]json.RawMessage)
	wire := make(map[string]json.RawMessage)
	percentiles := make(map[string]map[string]float64)
	for _, target := range []int{3, 10} {
		addStudents(target)
		name := fmt.Sprintf("live_%d", target)
		for iteration := range 35 {
			counter.Reset()
			before, started := tc.db.Stats(), time.Now()
			req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
			rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
			elapsed, after := time.Since(started), tc.db.Stats()
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			var result ogsLiveEnvelope
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &result))
			require.Len(t, result.Data.Students, target)
			for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
				require.Empty(t, counter.Operation(operation), "live projection must not issue DML")
			}
			require.Zero(t, counter.WriteRows())
			if iteration >= 5 {
				rows, statements := counter.Rows()
				samples[name] = append(samples[name], testpkg.RuntimeCheckpointSample{DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(), RowsAffected: rows, StatementsWithRows: statements, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
			}
			wire[name] = append(json.RawMessage(nil), rr.Body.Bytes()...)
		}
		queries := counter.Operation("SELECT")
		require.NotEmpty(t, queries)
		require.NoError(t, testpkg.WithTenantTx(t, ctx, tc.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var bypass bool
			require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
			require.False(t, bypass)
			for _, query := range queries {
				var plan string
				require.NoError(t, tx.NewRaw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query).Scan(txCtx, &plan))
				plans[name] = append(plans[name], json.RawMessage(plan))
			}
			return nil
		}))
		durations := make([]float64, 0, 30)
		for _, sample := range samples[name] {
			durations = append(durations, sample.DurationMS)
		}
		slices.Sort(durations)
		percentiles[name] = map[string]float64{"p50_ms": durations[14], "p95_ms": durations[28]}
	}
	report, err := json.Marshal(map[string]any{"samples": samples, "percentiles": percentiles, "plans": plans, "wire": wire, "unexpected_errors": 0})
	require.NoError(t, err)
	t.Logf("live-runtime: %s", report)
}
