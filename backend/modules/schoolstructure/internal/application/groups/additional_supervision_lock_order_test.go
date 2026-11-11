package groups_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// #6702: an additional supervisor's balance gate comes before the session
// and the staff row. The supervisor write itself takes the gate and then the
// row, like offboarding. Before, the assignment locked the session and the
// staff row first and the gate only inside the supervisor write, so it held
// the row while it waited for a gate that a second supervisor write of the
// same person held while it waited for the row: 40P01. The ledger now
// refuses that order; this test runs the real writers against each other.
//
// A third transaction holds the target's staff row, so the assignment waits
// there first and a second supervisor write of the target queues behind it.
// When the holder releases, both must commit.
func TestAdditionalSupervisionAndSupervisorWriteOfTheSameStaffDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPresence := api.NewPresenceSuiteRepositories(testpkg.NewDatabase(t, db))
	presence := api.NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)).Active
	module := api.NewSubstitutionSuiteModule(repos, testpkg.NewDatabase(t, db), SubstitutionDependencies{
		ActiveGroups: reposPresence.ActiveGroup, ActiveSupervisors: reposPresence.GroupSupervisor,
		ActiveSupervisorCreator: presence,
		StaffBalance:            testpkg.SettingsRuntime(t, db).LockStaffBalance,
	})
	activity := testpkg.CreateTestActivityGroup(t, db, "Zusatzaufsicht")
	room := testpkg.CreateTestRoom(t, db, "Zusatzraum")
	first := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	second := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	_, adminAccountID := activeTeacher(t, db, "Ada", "Admin")
	target, _ := activeTeacher(t, db, "Toni", "Target")
	ctx := testpkg.Ctx(t)

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	holderPID := backendPID(t, holder)
	_, err = holder.ExecContext(ctx, `SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE`, target.StaffID)
	require.NoError(t, err)

	assign := startRacingWrite(t, db, func(ctx context.Context) error {
		_, err := module.Assign(ctx, substitutionCaller(t, adminAccountID, true), additionalSupervisionAssignment(first.ID, target.StaffID))
		return err
	})
	awaitWaiting(t, db, assign, holderPID, "the assignment")
	supervise := startRacingWrite(t, db, func(ctx context.Context) error {
		return presence.CreateGroupSupervisor(ctx, &studentpresence.GroupSupervision{
			StaffID: target.StaffID, GroupID: second.ID, Role: "supervisor", StartDate: calendar.TodayDate().String(),
		})
	})
	awaitWaiting(t, db, supervise, 0, "the supervisor write")
	require.NoError(t, holder.Rollback())

	requireCommitted(t, assign, "the additional supervision")
	requireCommitted(t, supervise, "the supervisor write")
	var supervisions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.group_supervisors WHERE staff_id = ? AND group_id IN (?, ?)`,
		target.StaffID, first.ID, second.ID).Scan(ctx, &supervisions))
	require.Equal(t, 2, supervisions, "both supervisions of the target are stored")
}

type racingWrite struct {
	pid  int64
	done chan error
}

// startRacingWrite runs write in its own tenant transaction and returns once
// the transaction reported its backend.
func startRacingWrite(t *testing.T, db *bun.DB, write func(context.Context) error) racingWrite {
	t.Helper()
	pid, done := make(chan int64, 1), make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			var backend int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &backend); err != nil {
				return err
			}
			pid <- backend
			return write(ctx)
		})
	}()
	select {
	case backend := <-pid:
		return racingWrite{pid: backend, done: done}
	case err := <-done:
		require.FailNow(t, "racing write failed before it started", "%v", err)
		return racingWrite{}
	}
}

// awaitWaiting requires the write to wait for blocker (any transaction when
// blocker is 0), observed in pg_blocking_pids.
func awaitWaiting(t *testing.T, db *bun.DB, run racingWrite, blocker int64, what string) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-run.done:
			require.FailNow(t, what+" finished instead of waiting", "%v", err)
		default:
		}
		var blocked bool
		require.NoError(t, db.NewRaw(`SELECT CASE WHEN ?::int = 0 THEN cardinality(pg_blocking_pids(?::int)) > 0
			ELSE ?::int = ANY(pg_blocking_pids(?::int)) END`, blocker, run.pid, blocker, run.pid).Scan(context.Background(), &blocked))
		if blocked {
			return
		}
	}
	require.FailNow(t, what+" never waited")
}

func requireCommitted(t *testing.T, run racingWrite, what string) {
	t.Helper()
	select {
	case err := <-run.done:
		var state interface{ Field(byte) string }
		if errors.As(err, &state) && state.Field('C') == "40P01" {
			require.FailNow(t, what+" was the deadlock victim (40P01)", "%v", err)
		}
		require.NoError(t, err, "%s must commit", what)
	case <-time.After(30 * time.Second):
		require.FailNow(t, what+" never finished")
	}
}

func backendPID(t *testing.T, tx bun.Tx) int64 {
	t.Helper()
	var pid int64
	require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(context.Background(), &pid))
	return pid
}
