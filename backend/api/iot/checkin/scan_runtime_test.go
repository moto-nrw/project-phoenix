package checkin_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Local capability measurements include the real tenant transaction and
// owners, excluding fixture creation and assertions. HTTP/authentication
// contracts are verified by the separate route and device-auth suites.
func TestDeviceScanRuntimeEvidence(t *testing.T) {
	t.Parallel()
	isolated := testpkg.SetupIsolatedTestDB(t)
	db, module := testutil.SetupCheckinModule(t)
	require.Same(t, isolated, db)
	ctx := testpkg.Ctx(t)
	var postgres string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &postgres))
	counter := testpkg.CaptureQueriesForContext(t, db)
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	for _, operation := range []string{"check-in", "room-transfer", "check-out", "duplicate-supervisor", "rollback-after-scan"} {
		t.Run(operation, func(t *testing.T) {
			var samples []testpkg.RuntimeCheckpointSample
			var durations []float64
			codes := map[string]int{}
			var stopSampling func() testpkg.RuntimeCheckpointLockSamples
			var deadlocksBefore int64
			for iteration := range 35 {
				label := fmt.Sprintf("%s-%d", operation, iteration)
				room := testpkg.CreateTestRoom(t, db, label)
				activity := testpkg.CreateTestActivityGroup(t, db, label)
				group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
				staff := testpkg.CreateTestStaff(t, db, "Scan", label)
				device := testpkg.CreateTestDevice(t, db, label)
				testpkg.LinkDeviceToActiveGroup(t, db, group.ID, device.ID)
				testpkg.CreateTestGroupSupervisor(t, db, staff.ID, group.ID, "supervisor")
				student := testpkg.CreateTestStudent(t, db, "Scan", label, "1a")
				card := testpkg.CreateTestRFIDCard(t, db, fmt.Sprintf("RT%d", student.ID))
				personID := student.PersonID
				if operation == "duplicate-supervisor" {
					personID = staff.PersonID
				}
				testpkg.LinkRFIDToStudent(t, db, personID, card.ID)
				command := devicescan.ScanCommand{RFIDTag: card.ID, RoomID: &room.ID}
				var sourceVisitID int64
				if operation == "room-transfer" || operation == "check-out" {
					sourceRoom := testpkg.CreateTestRoom(t, db, label+" source")
					sourceGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, sourceRoom.ID)
					visit := testpkg.CreateTestVisit(t, db, student.ID, sourceGroup.ID, time.Now().Add(-time.Hour), nil)
					sourceVisitID = visit.ID
					if operation == "check-out" {
						command.RoomID = nil
					}
				}
				req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", nil, testutil.WithDeviceContext(device), testutil.WithStaffContext(staff))
				measuredCtx := counter.Context(req.Context())
				if iteration == 5 {
					deadlocksBefore = deadlocks()
					stopSampling = testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
						var waiting int
						err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &waiting)
						return waiting, err
					})
					t.Cleanup(func() { _ = stopSampling() })
				}
				abort := errors.New("injected scan rollback")
				var result *devicescan.ScanResult
				counter.Reset()
				before, started := db.Stats(), time.Now()
				err := testpkg.WithTenantTx(t, measuredCtx, db, testpkg.Tenant(t), func(txCtx context.Context, _ testpkg.Tx) error {
					var scanErr error
					result, scanErr = module.DeviceScan.Scan(txCtx, command)
					if scanErr != nil {
						return scanErr
					}
					if operation == "rollback-after-scan" {
						return abort
					}
					return nil
				})
				elapsed, after := time.Since(started), db.Stats()
				if operation == "rollback-after-scan" {
					require.ErrorIs(t, err, abort)
					var visits int
					require.NoError(t, db.NewRaw("SELECT count(*) FROM active.visits WHERE student_id = ?", student.ID).Scan(ctx, &visits))
					require.Zero(t, visits)
				} else {
					require.NoError(t, err)
				}
				require.NotNil(t, result)
				expected := map[string]string{"check-in": "checked_in", "room-transfer": "transferred", "check-out": "checked_out", "duplicate-supervisor": "supervisor_authenticated", "rollback-after-scan": "checked_in"}
				require.Equal(t, expected[operation], result.Action)
				if sourceVisitID != 0 {
					require.NotNil(t, testpkg.VisitExitTime(t, db, sourceVisitID))
				}
				if iteration >= 5 {
					duration := float64(elapsed) / float64(time.Millisecond)
					writes := counter.WriteRows()
					rows, statements := counter.Rows()
					samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: duration, Queries: counter.Total(), WriteRowsAffected: &writes,
						RowsAffected: rows, StatementsWithRows: statements, PoolWaitCount: after.WaitCount - before.WaitCount,
						PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
					durations = append(durations, duration)
					code := result.Action
					if operation == "rollback-after-scan" {
						code = "injected_rollback"
					}
					codes[code]++
				}
			}
			locks := stopSampling()
			require.Empty(t, locks.Error)
			slices.Sort(durations)
			report, err := json.Marshal(map[string]any{"operation": operation, "go": runtime.Version(), "postgres": postgres,
				"concurrency": 1, "warmup": 5, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28], "max_ms": durations[29],
				"result_codes": codes, "unexpected_errors": 0, "lock_samples": locks, "deadlocks": deadlocks() - deadlocksBefore})
			require.NoError(t, err)
			t.Logf("device-scan-runtime: %s", report)
		})
	}
}
