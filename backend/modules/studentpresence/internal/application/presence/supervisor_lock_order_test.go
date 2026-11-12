package presence_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// #6702: a supervisor write with several people takes every balance gate,
// ascending, before the first staff row. Before, it took each person's gate
// and row in turn, so it held the first person's staff row while it waited
// for a later person's gate, the order offboarding and every work-time
// writer reverse (gate, then row). The ledger refuses that order now.
//
// Another transaction holds the second supervisor's balance gate. The write
// must wait for it without holding the first supervisor's staff row, and
// commit once the gate is released.
func TestSupervisorWriteTakesEveryBalanceGateBeforeAnyStaffRow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildSessionEdgeCaseService(t, db)
	ctx := testpkg.Ctx(t)
	activity := testpkg.CreateTestActivityGroup(t, db, "lock-order-activity")
	room := testpkg.CreateTestRoom(t, db, "Lock Order Room")
	running := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	first := testpkg.CreateTestStaff(t, db, "LockOrder", "First")
	second := testpkg.CreateTestStaff(t, db, "LockOrder", "Second")
	require.Less(t, first.ID, second.ID)

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int64
	require.NoError(t, holder.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &holderPID))
	_, err = holder.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
		fmt.Sprintf("staff-balance:%d:%d", testpkg.Tenant(t), second.ID))
	require.NoError(t, err)

	pid, writing, done := make(chan int64, 1), make(chan context.Context, 1), make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var backend int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &backend); err != nil {
				return err
			}
			pid <- backend
			writing <- txCtx
			_, err := service.UpdateActiveGroupSupervisors(txCtx, running.ID, []int64{second.ID, first.ID})
			return err
		})
	}()
	writer, writeCtx := <-pid, <-writing
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-done:
			require.FailNow(t, "the supervisor write finished instead of waiting for the gate", "%v", err)
		default:
		}
		var waiting bool
		require.NoError(t, db.NewRaw(`SELECT ?::int = ANY(pg_blocking_pids(?::int))`, holderPID, writer).Scan(ctx, &waiting))
		if waiting {
			break
		}
		require.True(t, time.Now().Before(deadline), "the supervisor write never waited for the gate")
	}

	require.True(t, tenant.HoldsLock(writeCtx, tenant.LockStaffWorkTime, tenant.StaffWorkTimeKey(second.ID, false)),
		"the gate the write waits for is admitted in the lock order first")

	// While it waits for the second gate, the first staff row is free.
	probe, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = probe.ExecContext(ctx, `SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE NOWAIT`, first.ID)
	require.NoError(t, err, "the waiting supervisor write holds the first staff row")
	require.NoError(t, probe.Rollback())

	require.NoError(t, holder.Rollback())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		require.FailNow(t, "the supervisor write never finished")
	}
}
