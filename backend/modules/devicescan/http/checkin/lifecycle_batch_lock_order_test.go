package checkin_test

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

// One ambient sweep locks two idle sessions without children, A before B,
// while a scan checks a child into A. Neither order may deadlock. A child
// checked in before the sweep's lock defers the whole sweep: A is in use
// again, and the next run ends B (#6502, #6977).
func TestTimeoutCohortAndScanDoNotDeadlock(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, first := range []string{"cleanup_first", "scan_first"} {
		t.Run(first, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			f, _ := newSessionEndScanFixture(t)
			ctx := testpkg.Ctx(t)
			_, err := f.k.db.NewUpdate().Table("active.visits").Set("exit_time = NOW() - INTERVAL '30 minutes'").
				Where("active_group_id = ? AND exit_time IS NULL", f.scanStay).Exec(ctx)
			require.NoError(t, err)
			_, _, source := f.k.roomWithSession(t, "Timeout Cohort B")
			_, err = f.k.db.NewUpdate().Table("active.groups").Set("last_activity = NOW() - INTERVAL '2 hours'").Where("id = ?", source).Exec(ctx)
			require.NoError(t, err)
			testpkg.LinkDeviceToActiveGroup(t, f.k.db, source, testpkg.CreateTestDevice(t, f.k.db, "cohort-b-device").ID)
			require.Less(t, f.scanStay, source, "the sweep locks A before B")
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
				// The sweep holds A and waits for B; the scan queues on A.
				holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", source)
				ended = startLockOrderCommand(t, cleanup)
				sweepPID := awaitWaiterOf(t, f.k.db, holder.pid)
				scanned = startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
				awaitWaiterOf(t, f.k.db, sweepPID)
				holder.commit(t)
			} else {
				// The scan holds A's child check-in; the sweep queues on A.
				holder := holdRow(t, f.k.db, "SELECT id FROM active.groups WHERE id = ? FOR UPDATE", f.scanStay)
				scanned = startLockOrderScan(t, f.k, f.tag, f.scanRoom, f.scanDevice, f.scanStaff)
				scanPID := awaitWaiterOf(t, f.k.db, holder.pid)
				ended = startLockOrderCommand(t, cleanup)
				awaitWaiterOf(t, f.k.db, scanPID)
				holder.commit(t)
			}
			response := awaitScan(t, scanned)
			require.NoError(t, awaitReattach(t, ended))
			assertNoLockFailureRollback(t, queries)
			groups, err := f.k.presence(t).QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{IDs: []int64{source, f.scanStay}})
			require.NoError(t, err)
			require.Len(t, groups, 2)
			visits, err := f.k.presence(t).ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{source, f.scanStay}, OpenOnly: true})
			require.NoError(t, err)
			if first == "cleanup_first" {
				// The scan reaches A only after the sweep ended it; its answer
				// to that is #7024. Here it must just not deadlock.
				require.NotNil(t, response)
				require.Equal(t, 2, count)
				for _, group := range groups {
					require.NotNil(t, group.EndTime)
				}
				require.Empty(t, visits)
				return
			}
			testutil.AssertSuccessResponse(t, response, 200)
			require.Zero(t, count, "A is in use again, so the sweep waits for its next run")
			for _, group := range groups {
				require.Nil(t, group.EndTime)
			}
			require.Len(t, visits, 1)
			require.Equal(t, f.scanStay, visits[0].ActiveGroupID)
			require.Equal(t, f.student, visits[0].StudentID)
		})
	}
}
