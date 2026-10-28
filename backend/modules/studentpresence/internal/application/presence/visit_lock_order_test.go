package presence_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Wait for an observed PostgreSQL lock wait, not an assumed goroutine order.
func awaitPresenceWait(t *testing.T, ctx context.Context, db *bun.DB, holder int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
   WHERE ?::int = ANY(pg_blocking_pids(pid)))`, holder).Scan(ctx, &waiting)
		require.NoError(t, err)
		return waiting
	}, 5*time.Second, 10*time.Millisecond)
}

func TestUpdateVisitLocksBothSessionsBeforeTarget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	presence := setupActiveService(t, db)
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()
	activity := testpkg.CreateTestActivityGroup(t, db, "Transfer activity")
	firstRoom := testpkg.CreateTestRoom(t, db, "Transfer first")
	secondRoom := testpkg.CreateTestRoom(t, db, "Transfer second")
	firstID := testpkg.CreateTestActiveGroup(t, db, activity.ID, firstRoom.ID).ID
	secondID := testpkg.CreateTestActiveGroup(t, db, activity.ID, secondRoom.ID).ID
	firstStudent := testpkg.CreateTestStudent(t, db, "First", "Transfer", "1a")
	secondStudent := testpkg.CreateTestStudent(t, db, "Second", "Transfer", "1a")
	first := testpkg.CreateTestVisit(t, db, firstStudent.ID, firstID, time.Now(), nil)
	second := testpkg.CreateTestVisit(t, db, secondStudent.ID, secondID, time.Now(), nil)
	forward := make(chan error, 1)
	err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		var pid int
		if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
			return err
		}
		var locked int64
		if err := tx.NewRaw("SELECT id FROM active.groups WHERE id = ? AND tenant_id = ? FOR UPDATE", firstID, testpkg.Tenant(t)).Scan(txCtx, &locked); err != nil {
			return err
		}
		go func() {
			forward <- presence.UpdateVisit(ctx, &studentpresence.Visit{ID: first.ID, StudentID: firstStudent.ID, ActiveGroupID: secondID, EntryTime: first.EntryTime})
		}()
		awaitPresenceWait(t, ctx, db, pid)
		// The forward move must wait on the lower source ID before locking its
		// destination. Otherwise this public reverse move completes the cycle.
		return presence.UpdateVisit(txCtx, &studentpresence.Visit{ID: second.ID, StudentID: secondStudent.ID, ActiveGroupID: firstID, EntryTime: second.EntryTime})
	})
	select {
	case forwardErr := <-forward:
		require.NoError(t, err)
		require.NoError(t, forwardErr)
	case <-ctx.Done():
		t.Fatal("opposite transfer did not finish")
	}
	for visitID, groupID := range map[int64]int64{first.ID: secondID, second.ID: firstID} {
		stored, err := testSchoolPresence(t, db).FindVisit(ctx, visitID)
		require.NoError(t, err)
		require.Equal(t, groupID, stored.ActiveGroupID)
	}
}
