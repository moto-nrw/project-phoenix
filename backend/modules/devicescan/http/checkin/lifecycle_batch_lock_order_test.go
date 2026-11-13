package checkin_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
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
