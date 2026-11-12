package contracttest_test

// Staff exits, staff creation and assignment writes against each other
// (#6702), in the order Konto → Schule → Profil. The staff person merge
// against offboarding is api/staff_person_merge_lock_order_test.go. Invariant
// K1: the staff row lock is FOR NO KEY UPDATE, so the foreign-key key share
// of a row referencing the staff member never waits for it.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/moto-ogs/project-phoenix/workflows/staffoffboarding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// K1: a transaction holds S's staff row through School Membership's
// FindStaffForMutation, as offboarding and every staff writer do. An
// assignment insert that references S (its foreign key takes a key share on
// S's row) must not wait for it. With FOR UPDATE it waited, so a writer that
// held a block or another leaf while it inserted could close a cycle with
// offboarding.
func TestStaffRowLockLeavesForeignKeySharesFree(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	sc := newOffboardingScenario(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Key", "Share")
	room := testpkg.CreateTestRoom(t, db, "Key share room")
	block := testpkg.CreateTestActivityInstance(t, db, dayWriterRaceDate, room.ID, testpkg.ActivityInstanceOpts{Title: "Key share block"})

	locked, release := make(chan struct{}), make(chan struct{})
	holder := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		if _, err := sc.deps.Membership.FindStaffForMutation(ctx, staff.ID); err != nil {
			return err
		}
		close(locked)
		<-release
		return nil
	})
	defer close(release)
	select {
	case <-locked:
	case err := <-holder.done:
		require.FailNow(t, "the staff row holder failed", "%v", err)
	}
	insert := startRaceTx(t, db, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewRaw(`INSERT INTO schedule.instance_staff (tenant_id, instance_id, staff_id) VALUES (?, ?, ?)`,
			testpkg.Tenant(t), block.ID, staff.ID).Exec(ctx)
		return err
	})
	requireFinishedWithoutWaitingFor(t, db, insert, holder.pid, "the assignment insert")
}

// requireFinishedWithoutWaitingFor requires the transaction to finish
// without ever waiting for the holder, observed in pg_blocking_pids: a wait
// fails at once, so no deadline decides the result.
func requireFinishedWithoutWaitingFor(t *testing.T, poller *bun.DB, run raceTx, holder int64, what string) {
	t.Helper()
	// The deadline only bounds a hang without a wait on the holder.
	deadline := time.After(30 * time.Second)
	for {
		select {
		case err := <-run.done:
			require.NoError(t, err, "%s must succeed", what)
			return
		case <-deadline:
			require.FailNow(t, what+" never finished")
		case <-time.After(5 * time.Millisecond):
		}
		var blocked bool
		require.NoError(t, poller.NewRaw(`SELECT ?::int = ANY(pg_blocking_pids(?::int))`, holder, run.pid).Scan(context.Background(), &blocked))
		require.False(t, blocked, "%s waited for the staff row lock", what)
	}
}

// A1 (Konto → Schule → Profil): the scheduled run of a due planned exit of X
// and an immediate offboarding of X. A third transaction holds X's staff row,
// so the run stops at it, and the immediate offboarding starts meanwhile.
// Before #6702 the run held X's work-time and shift gates and locked X's
// account only in the exit's snapshot after the staff row, while the
// immediate offboarding held the account and waited for the gates: 40P01.
// Now the run locks the account before its gates, so the immediate
// offboarding waits at the account, and the run completes first.
func TestPlannedExitRunAndImmediateOffboardingDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	sc := newOffboardingScenario(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestStaff(t, db, "Race", "Planner")
	subject, account := testpkg.CreateTestStaffWithAccount(t, db, "Race", "Leaver")
	_, err := db.NewRaw(`UPDATE users.staff_school_memberships SET planned_exit_day = '2026-11-01', planned_exit_by_account_id = ?,
		planned_exit_by_staff_id = ?, planned_exit_by_name = 'Leitung', planned_exit_at = NOW() WHERE tenant_id = ? AND id = ?`,
		account.ID, actor.ID, testpkg.Tenant(t), subject.ID).Exec(ctx)
	require.NoError(t, err)
	offboarding, err := api.NewStaffOffboardingWorkflow(sc.deps)
	require.NoError(t, err)
	planner, err := staffoffboarding.NewPlanner(offboarding, staffoffboarding.PlannerDependencies{
		AppendAudit: func(context.Context, staffoffboarding.Actor, staffoffboarding.ExitPlanChange) error { return nil },
		Now:         time.Now,
	})
	require.NoError(t, err)
	unitPID := make(chan int64, 1)
	due, err := staffoffboarding.NewDueExits(planner, staffoffboarding.DueExitDependencies{
		Notify: func(context.Context, staffoffboarding.ExitNotice) (bool, error) { return true, nil },
		// The exit runs in a transaction of its own (#6703); its backend is
		// the one that waits.
		Within: func(ctx context.Context, fn func(context.Context) error) (error, error) {
			return api.PlannedExitTransaction(ctx, func(txCtx context.Context) error {
				tx, _ := tenant.TransactionFromContext(txCtx)
				var backend int64
				if err := tx.(bun.Tx).NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &backend); err != nil {
					return err
				}
				unitPID <- backend
				return fn(txCtx)
			})
		},
		BoundLockWaits: api.BoundPlannedExitLockWaits(30 * time.Second),
		Isolate: func(ctx context.Context, fn func(context.Context) error) (error, error) {
			failed := tenant.WithSavepoint(ctx, fn)
			if errors.Is(failed, tenant.ErrSavepointControl) {
				return nil, failed
			}
			return failed, nil
		},
		Now: time.Now,
	})
	require.NoError(t, err)

	locked, release := make(chan struct{}), make(chan struct{})
	barrier := startRaceTx(t, db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE`, subject.ID).Exec(ctx); err != nil {
			return err
		}
		close(locked)
		<-release
		return nil
	})
	select {
	case <-locked:
	case err := <-barrier.done:
		require.FailNow(t, "the barrier failed", "%v", err)
	}
	var run staffoffboarding.DueRun
	scheduled := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		var err error
		run, err = due.RunDue(ctx)
		return err
	})
	var exit raceTx
	select {
	case pid := <-unitPID:
		exit = raceTx{pid: pid, done: scheduled.done}
	case err := <-scheduled.done:
		require.FailNow(t, "the planned exit run finished before its exit started", "%v", err)
	}
	awaitBlockedBy(t, db, exit, barrier.pid, "the planned exit run")
	immediate := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		actorCtx := context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actor.ID, Username: "test-admin"})
		_, err := offboarding.Offboard(actorCtx, subject.ID)
		return err
	})
	awaitBlockedBy(t, db, immediate, exit.pid, "the immediate offboarding")
	close(release)
	requireFinishedWithoutDeadlock(t, barrier, "the barrier")
	requireFinishedWithoutDeadlock(t, scheduled, "the planned exit run")
	for _, exit := range run.Exits {
		assert.NotEqual(t, staffoffboarding.ExitFailed, exit.Outcome, "the due exit ran")
	}
	select {
	case err := <-immediate.done:
		var state interface{ Field(byte) string }
		require.False(t, errors.As(err, &state) && state.Field('C') == "40P01", "the immediate offboarding was the deadlock victim: %v", err)
	case <-time.After(30 * time.Second):
		require.FailNow(t, "the immediate offboarding never finished")
	}
	var retired bool
	require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.staff_school_memberships WHERE id = ?`, subject.ID).Scan(ctx, &retired))
	assert.True(t, retired, "the planned exit retired X")
}

// M2: offboarding of S has locked S's staff row and waits at S's caregiver
// profile, which a third transaction holds. A staff creation for S's person
// starts meanwhile and would adopt S. Before #6702 it locked the person and
// then waited for S's staff row, so offboarding, released, waited for the
// person while the creation waited for S: 40P01. Now the creation locks the
// live staff row it adopts before the person and waits there holding no
// person; offboarding finishes, and the creation finds S retired.
func TestStaffAdoptionAndOffboardingDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	sc := newOffboardingScenario(t, db)
	ctx := testpkg.Ctx(t)
	actor := testpkg.CreateTestStaff(t, db, "Race", "Actor")
	subject := testpkg.CreateTestTeacher(t, db, "Race", "Adopted")
	people := setupServiceFactory(t, db).Users
	offboarding, err := api.NewStaffOffboardingWorkflow(sc.deps)
	require.NoError(t, err)

	locked, release := make(chan struct{}), make(chan struct{})
	barrier := startRaceTx(t, db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`SELECT id FROM users.teachers WHERE id = ? FOR UPDATE`, subject.ID).Exec(ctx); err != nil {
			return err
		}
		close(locked)
		<-release
		return nil
	})
	select {
	case <-locked:
	case err := <-barrier.done:
		require.FailNow(t, "the barrier failed", "%v", err)
	}
	exit := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		actorCtx := context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actor.ID, Username: "test-admin"})
		_, err := offboarding.Offboard(actorCtx, subject.StaffID)
		return err
	})
	awaitBlockedBy(t, db, exit, barrier.pid, "offboarding")
	adoption := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		_, _, _, err := people.CreateStaffWithTeacher(ctx, api.StaffDirectoryCreateInput(api.StaffCreateInput{
			PersonID: subject.Staff.PersonID, StaffNotes: "Rennen",
			ActorPermissions: []string{permissions.UsersCreate, permissions.StaffManage},
		}))
		return err
	})
	awaitBlockedBy(t, db, adoption, exit.pid, "the staff creation")
	close(release)
	requireFinishedWithoutDeadlock(t, barrier, "the barrier")
	requireFinishedWithoutDeadlock(t, exit, "offboarding")
	select {
	case err := <-adoption.done:
		var state interface{ Field(byte) string }
		require.False(t, errors.As(err, &state) && state.Field('C') == "40P01", "the staff creation was the deadlock victim: %v", err)
	case <-time.After(30 * time.Second):
		require.FailNow(t, "the staff creation never finished")
	}
	var retired bool
	require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.staff_school_memberships WHERE id = ?`, subject.StaffID).Scan(ctx, &retired))
	assert.True(t, retired, "offboarding retired S")
}
