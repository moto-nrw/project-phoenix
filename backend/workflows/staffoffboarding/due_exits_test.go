package staffoffboarding_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/stretchr/testify/require"
)

// lockedBuffer collects the run's JSON log lines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// exitLines are the result lines of the planned exits by staff ID; a second
// line for one exit fails the test.
func (b *lockedBuffer) exitLines(t *testing.T) map[int64]map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	lines := map[int64]map[string]any{}
	for _, raw := range strings.Split(strings.TrimSpace(b.buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(raw), &line))
		if line["msg"] != "planned staff exit" {
			continue
		}
		staffID := int64(line["staff_id"].(float64))
		require.NotContains(t, lines, staffID, "one result line per planned exit and run")
		lines[staffID] = line
	}
	b.buf.Reset()
	return lines
}

// threeDueExits plans the exits of the fixture's member and two more for the
// same day and returns their staff IDs, the fixture's member first.
func threeDueExits(t *testing.T, f *plannerFixture, planner *testPlanner, day string) [3]int64 {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	ids := [3]int64{f.staffID, testpkg.CreateTestStaff(t, db, "Unit", "Second").ID, testpkg.CreateTestStaff(t, db, "Unit", "Third").ID}
	for _, id := range ids {
		_, err := planner.Plan(ctx, id, plannedExitDate(day))
		require.NoError(t, err)
	}
	return ids
}

func (f *plannerFixture) staffRetired(t *testing.T, staffID int64) bool {
	t.Helper()
	_, err := f.membership.FindStaff(testpkg.Ctx(t), staffID)
	if err == nil {
		return false
	}
	require.ErrorIs(t, err, schoolmembership.ErrStaffNotFound)
	return true
}

// A database error while the middle of three due exits takes its locks rolls
// back that exit alone (#6703). Before, the run took every exit's locks up
// front in the school's transaction, so the error ended the whole run and
// none of the three retired.
func TestDueExitLockFailureRollsBackOnlyThatExit(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	ids := threeDueExits(t, f, planner, "2027-10-10")
	f.today, f.failLockOf = "2027-10-11", ids[1]

	require.NoError(t, planner.RunDue(ctx), "one failing exit does not end the run")
	require.Equal(t, 3, f.units)
	failed := planner.lastRun.Failed()
	require.Len(t, failed, 1)
	require.ErrorContains(t, failed[ids[1]], "division by zero")
	require.True(t, f.staffRetired(t, ids[0]), "the first exit committed")
	require.True(t, f.staffRetired(t, ids[2]), "the exit after the failing one ran and committed")
	require.False(t, f.staffRetired(t, ids[1]))
	plan, err := f.exits.FindPlannedExitForMutation(ctx, ids[1])
	require.NoError(t, err, "the failed exit keeps its plan")
	require.Nil(t, plan.AccessLockedAt, "nothing of the failed exit was kept")
	require.Nil(t, plan.NextAttemptAt, "the failed exit is due again on the next run")
	lines := f.logs.exitLines(t)
	require.Len(t, lines, 3)
	require.Equal(t, staffoffboarding.ExitFailed, lines[ids[1]]["outcome"])
	require.Contains(t, lines[ids[1]]["error"], "division by zero")
	for _, id := range []int64{ids[0], ids[2]} {
		require.Equal(t, staffoffboarding.ExitSucceeded, lines[id]["outcome"])
		require.Equal(t, float64(testpkg.Tenant(t)), lines[id]["tenant_id"])
	}

	f.failLockOf = 0
	require.NoError(t, planner.RunDue(ctx))
	require.Empty(t, planner.lastRun.Failed())
	require.True(t, f.staffRetired(t, ids[1]), "the next run completes the failed exit")
	require.Len(t, f.deletion, 3, "every exit ran its offboarding once")
}

// A second connection holds the middle exit's membership row. The run waits
// for it, gives way after the bound and finishes the other reminders; the
// next run sends the held one once (#6703).
func TestDueExitLockedByAnotherTransactionIsDeferred(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	ids := threeDueExits(t, f, planner, "2027-10-10")
	f.today, f.lockWait = "2027-10-03", time.Second

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int64
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &holderPID))
	_, err = holder.ExecContext(ctx, `SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE`, ids[1])
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- planner.RunDue(ctx) }()
	waited := false
	deadline := time.After(30 * time.Second)
	for finished := false; !finished; {
		select {
		case err := <-done:
			require.NoError(t, err)
			finished = true
		case <-deadline:
			require.FailNow(t, "the run never finished while the exit was locked")
		case <-time.After(5 * time.Millisecond):
			if !waited {
				require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ?::int = ANY(pg_blocking_pids(pid)))`, holderPID).Scan(ctx, &waited))
			}
		}
	}
	require.True(t, waited, "the run waited for the held row before it gave way")
	require.Empty(t, planner.lastRun.Failed(), "a locked exit is no failure")
	require.Len(t, f.notices, 2, "the other reminders went out")
	for _, notice := range f.notices {
		require.NotEqual(t, ids[1], notice.StaffID)
	}
	lines := f.logs.exitLines(t)
	require.Len(t, lines, 3)
	require.Equal(t, staffoffboarding.ExitDeferred, lines[ids[1]]["outcome"])
	require.Contains(t, lines[ids[1]]["error"], staffoffboarding.ErrExitLocked.Error())
	require.Equal(t, staffoffboarding.ExitSucceeded, lines[ids[0]]["outcome"])
	require.Equal(t, staffoffboarding.ExitSucceeded, lines[ids[2]]["outcome"])
	require.NoError(t, holder.Rollback())

	for _, id := range []int64{ids[0], ids[2]} {
		plan, err := f.exits.FindPlannedExitForMutation(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, plan.WeekNoticeSentAt, "the sent reminder is recorded")
	}
	plan, err := f.exits.FindPlannedExitForMutation(ctx, ids[1])
	require.NoError(t, err)
	require.Nil(t, plan.WeekNoticeSentAt, "the deferred reminder is still due")

	require.NoError(t, planner.RunDue(ctx))
	require.NoError(t, planner.RunDue(ctx))
	require.Len(t, f.notices, 3, "the deferred reminder went out once, the others not again")
	require.Equal(t, ids[1], f.notices[2].StaffID)
	require.Equal(t, staffoffboarding.NoticeWeekBefore, f.notices[2].Kind)
	lines = f.logs.exitLines(t)
	require.Len(t, lines, 1, "after the reminders went out, no exit has work due")
	require.Equal(t, staffoffboarding.ExitSucceeded, lines[ids[1]]["outcome"])
}

// The execution takes the person lock after the preliminary locks. A held
// person must defer that exit without recording an hourly retry or stopping
// the other exits, even though the execution rolls back to a savepoint.
func TestDueExitPersonLockedDuringExecutionIsDeferred(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	ids := threeDueExits(t, f, planner, "2027-10-10")
	f.today, f.lockWait = "2027-10-11", 100*time.Millisecond
	staff, err := f.membership.FindStaff(ctx, ids[1])
	require.NoError(t, err)

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.ExecContext(ctx, `SELECT id FROM users.persons WHERE id = ? FOR UPDATE`, staff.PersonID)
	require.NoError(t, err)

	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, planner.RunDue(runCtx), "the run finishes while the person lock is still held")
	require.Len(t, planner.lastRun.Exits, 3)
	require.Empty(t, planner.lastRun.Failed(), "a locked person defers the exit, it does not fail it")
	for _, result := range planner.lastRun.Exits {
		if result.StaffID == ids[1] {
			require.Equal(t, staffoffboarding.ExitDeferred, result.Outcome)
			require.ErrorIs(t, result.Err, staffoffboarding.ErrExitLocked)
		} else {
			require.Equal(t, staffoffboarding.ExitSucceeded, result.Outcome)
		}
	}
	for _, id := range []int64{ids[0], ids[2]} {
		require.True(t, f.staffRetired(t, id), "the other exits committed")
	}
	require.False(t, f.staffRetired(t, ids[1]))
	plan, err := f.exits.FindPlannedExitForMutation(ctx, ids[1])
	require.NoError(t, err)
	require.Nil(t, plan.AccessLockedAt, "the deferred exit keeps no access change")
	require.Nil(t, plan.NextAttemptAt, "the deferred exit is due on the next run, not in an hour")
	lines := f.logs.exitLines(t)
	require.Len(t, lines, 3)
	require.Equal(t, staffoffboarding.ExitDeferred, lines[ids[1]]["outcome"])
	require.Contains(t, lines[ids[1]]["error"], staffoffboarding.ErrExitLocked.Error())
	require.NoError(t, holder.Rollback())

	require.NoError(t, planner.RunDue(ctx))
	require.True(t, f.staffRetired(t, ids[1]), "the next run completes the deferred exit")
	require.Len(t, f.deletion, 3, "each exit completed once")
	require.NoError(t, planner.RunDue(ctx))
	require.Len(t, f.deletion, 3)
}

// A unit whose transaction can no longer be trusted ends the school's run:
// it is reported as aborted, never as an isolated failure, and the exits
// after it wait for the next run.
func TestDueExitUntrustedTransactionAbortsTheRun(t *testing.T) {
	t.Parallel()
	f, planner := newPlannerFixture(t)
	ctx := testpkg.Ctx(t)
	ids := threeDueExits(t, f, planner, "2027-10-10")
	f.today = "2027-10-11"
	f.breakUnit = 2

	err := planner.RunDue(ctx)
	require.ErrorIs(t, err, tenant.ErrSavepointControl)
	require.Len(t, planner.lastRun.Exits, 2, "the run stopped at the broken unit")
	require.Equal(t, staffoffboarding.ExitSucceeded, planner.lastRun.Exits[0].Outcome)
	aborted := planner.lastRun.Exits[1]
	require.Equal(t, staffoffboarding.ExitAborted, aborted.Outcome)
	require.Empty(t, planner.lastRun.Failed(), "an aborted run reports no isolated failure")
	lines := f.logs.exitLines(t)
	require.Len(t, lines, 2)
	require.Equal(t, staffoffboarding.ExitAborted, lines[aborted.StaffID]["outcome"])
	retired := 0
	for _, id := range ids {
		if f.staffRetired(t, id) {
			retired++
		}
	}
	require.Equal(t, 1, retired, "only the unit before the broken one committed")

	f.breakUnit = 0
	require.NoError(t, planner.RunDue(ctx))
	for _, id := range ids {
		require.True(t, f.staffRetired(t, id), fmt.Sprintf("staff %d retired on the next run", id))
	}
}

// newTestLogger logs JSON lines into buf.
func newTestLogger(buf *lockedBuffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// breakSavepoint returns a savepoint-control failure, as a broken savepoint
// of the exit's transaction would.
func breakSavepoint(context.Context) error {
	return fmt.Errorf("%w: injected", tenant.ErrSavepointControl)
}
