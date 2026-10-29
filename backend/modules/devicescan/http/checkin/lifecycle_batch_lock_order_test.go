package checkin_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOpenRoomWorkflowAndAdminAmendmentDoNotDeadlock(t *testing.T) {
	t.Parallel()
	// Initialize the package clone; each actor fixture uses its own pool.
	testpkg.SetupTestDB(t)
	for _, first := range []string{"open_room_first", "admin_first"} {
		t.Run(first, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f := newBulkMoveScanFixture(t)
			ctx := testpkg.Ctx(t)
			_, err := f.k.db.NewUpdate().Table("facilities.rooms").Set("is_open_room = TRUE").Where("tenant_id = ?", testpkg.Tenant(t)).Where("id = ?", f.scanRoom).Exec(ctx)
			require.NoError(t, err)
			move, err := api.NewOpenRoomMoveTestRunner(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db))
			require.NoError(t, err)
			operations := api.NewActiveTestPresence(testpkg.NewDatabase(t, f.k.db), testpkg.TenantRuntime(t, f.k.db)).PresenceOperations()
			visit, err := f.k.presence(t).FindVisit(ctx, f.visit)
			require.NoError(t, err)
			visit.ActiveGroupID = f.scanStay
			if first == "open_room_first" {
				at := time.Now()
				visit.ExitTime = &at
			}
			holder := holdRow(t, f.k.db, "SELECT id FROM facilities.rooms WHERE id = ? FOR UPDATE", f.scanRoom)
			var target int64
			moveCommand := func(ctx context.Context) error {
				var err error
				target, err = move(ctx, []int64{f.student}, f.scanRoom)
				return err
			}
			amendCommand := func(ctx context.Context) error { return operations.AmendVisit(ctx, *visit) }
			var moved, amended <-chan error
			if first == "open_room_first" {
				moved = startLockOrderCommand(t, moveCommand)
				pid := awaitWaiterOf(t, f.k.db, holder.pid)
				amended = startLockOrderCommand(t, amendCommand)
				awaitWaiterOf(t, f.k.db, pid)
			} else {
				amended = startLockOrderCommand(t, amendCommand)
				pid := awaitWaiterOf(t, f.k.db, holder.pid)
				moved = startLockOrderCommand(t, moveCommand)
				awaitWaiterOf(t, f.k.db, pid)
			}
			holder.commit(t)
			require.NoError(t, awaitReattach(t, moved))
			require.NoError(t, awaitReattach(t, amended))
			require.NotZero(t, target)
			f.assertOnlyOpenVisitIn(t, target)
		})
	}
}

func TestTimeoutCohortAndScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, first := range []string{"cleanup_first", "scan_first"} {
		t.Run(first, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f, _ := newSessionEndScanFixture(t)
			// A is the scanner's destination and sorts before B. A single ambient
			// cleanup transaction ends both. The scan moves one child from B to A.
			_, _, source := f.k.roomWithSession(t, "Timeout Source B")
			ctx := testpkg.Ctx(t)
			_, err := f.k.db.NewUpdate().Table("active.groups").Set("last_activity = NOW() - INTERVAL '2 hours'").Where("id = ?", source).Exec(ctx)
			require.NoError(t, err)
			blocker := f.visit
			_, err = f.k.db.NewUpdate().Table("active.visits").Set("active_group_id = ?", source).Where("id = ?", blocker).Exec(ctx)
			require.NoError(t, err)
			f.tag, f.student, _ = f.k.studentCard(t, "Batch", "Scanned", "3a")
			attendanceDevice := testpkg.CreateTestDevice(t, f.k.db, "batch-attendance-device")
			testpkg.LinkDeviceToActiveGroup(t, f.k.db, source, attendanceDevice.ID)
			f.attendance = testpkg.CreateTestAttendance(t, f.k.db, f.student, 0, attendanceDevice.ID, time.Now().Add(-2*time.Hour), nil).ID
			f.visit = testpkg.CreateTestVisit(t, f.k.db, f.student, source, time.Now().Add(-time.Hour), nil).ID
			queries := testpkg.CaptureQueries(t, f.k.db)
			var count int
			cleanup := func(ctx context.Context) error {
				return testpkg.WithTenantTx(t, ctx, f.k.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
					var err error
					count, err = f.active.CleanupAbandonedSessions(txCtx, time.Minute)
					return err
				})
			}
			var ended <-chan error
			var scanned <-chan *httptest.ResponseRecorder
			if first == "cleanup_first" {
				holder := holdRow(t, f.k.db, "SELECT id FROM active.visits WHERE id = ? FOR UPDATE", blocker)
				ended = startLockOrderCommand(t, cleanup)
				pid := awaitWaiterOf(t, f.k.db, holder.pid)
				scan := startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
				result := make(chan *httptest.ResponseRecorder, 1)
				done := make(chan error, 1)
				go func() { result <- <-scan; done <- nil }()
				_, err := awaitDoneOrWaiting(t, f.k.db, done, pid)
				require.NoError(t, err)
				scanned = result
				holder.commit(t)
			} else {
				holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", f.scanStay)
				scanned = startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
				pid := awaitWaiterOf(t, f.k.db, holder.pid)
				ended = startLockOrderCommand(t, cleanup)
				awaitWaiterOf(t, f.k.db, pid)
				holder.commit(t)
			}
			testutil.AssertSuccessResponse(t, awaitScan(t, scanned), 200)
			require.NoError(t, awaitReattach(t, ended))
			require.Equal(t, 2, count)
			assertNoLockFailureRollback(t, queries)
			groups, err := f.k.presence(t).QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{IDs: []int64{source, f.scanStay}})
			require.NoError(t, err)
			require.Len(t, groups, 2)
			for _, group := range groups {
				require.NotNil(t, group.EndTime)
			}
			visits, err := f.k.presence(t).ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{source, f.scanStay}, OpenOnly: true})
			require.NoError(t, err)
			require.Empty(t, visits)
		})
	}
}
