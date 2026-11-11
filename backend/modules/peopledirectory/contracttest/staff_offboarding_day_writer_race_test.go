package contracttest_test

// Offboarding against a Timetable day writer that adds a staffing row of the
// offboarded person (#6583). Invariant O1: users.staff row locks, and the
// foreign-key key shares an assignment row takes on them, are one class of
// tenant.OrderLock (staff rows, ascending staff id) after the day locks and
// before every block row. A day writer that rewrites a block's staffing takes
// the staff rows of its roster before it locks the block, so it never holds
// the block while it waits for a staff row offboarding holds.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// dayWriterRaceDate is a fixed future planning day, far ahead of the
// production clock offboarding reads its operational boundary from.
var dayWriterRaceDate = calendar.NewDate(2099, time.June, 1)

// replanRaceDate is a fixed future school day without a holiday, which the
// materializer plans.
var replanRaceDate = calendar.NewDate(2099, time.June, 9)

// raceTx is one tenant transaction of the race: it reports its backend
// first, then its outcome.
type raceTx struct {
	pid  int64
	done chan error
}

func startRaceTx(t *testing.T, db *bun.DB, write func(ctx context.Context, tx bun.Tx) error) raceTx {
	t.Helper()
	pid, done := make(chan int64, 1), make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			var backend int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &backend); err != nil {
				return err
			}
			pid <- backend
			return write(ctx, tx)
		})
	}()
	select {
	case backend := <-pid:
		return raceTx{pid: backend, done: done}
	case err := <-done:
		require.FailNow(t, "race transaction failed before it started", "%v", err)
		return raceTx{}
	}
}

// awaitBlockedBy requires the transaction to wait for a lock the blocker
// holds, observed in pg_blocking_pids, without sleeping for the interleaving.
func awaitBlockedBy(t *testing.T, run raceTx, blocker int64, what string) {
	t.Helper()
	poller := testpkg.SetupClosableTestDB(t)
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-run.done:
			require.FailNow(t, what+" finished instead of waiting", "%v", err)
		default:
		}
		var blocked bool
		require.NoError(t, poller.NewRaw(`SELECT ?::int = ANY(pg_blocking_pids(?::int))`, blocker, run.pid).Scan(context.Background(), &blocked))
		if blocked {
			return
		}
	}
	require.FailNow(t, what+" never waited for the expected transaction")
}

func requireFinishedWithoutDeadlock(t *testing.T, run raceTx, what string) {
	t.Helper()
	select {
	case err := <-run.done:
		var state interface{ Field(byte) string }
		if errors.As(err, &state) && state.Field('C') == "40P01" {
			require.FailNow(t, what+" was the deadlock victim (40P01)", "%v", err)
		}
		require.NoError(t, err, "%s must succeed", what)
	case <-time.After(30 * time.Second):
		require.FailNow(t, what+" never finished")
	}
}

// O1: S is planned on the series occurrence T1, not yet on T2. A series
// roster change (the real AlignPlannedInstanceStaff day writer) adds S2 to
// both occurrences and S to T2. A third transaction holds T2, so the change
// stops after it added S2 on T1 (holding T1) and before it adds S on T2.
// Offboarding of S runs meanwhile. Before #6583 the change then waited for
// S's staff row while offboarding held that row and waited for T1: 40P01.
// Now the change takes the staff rows of S and S2 before any block row, and
// offboarding waits for the change's commit, then removes both of S's rows.
func TestOffboardingAndDayWriterAddingTheSameStaffDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	sc := newOffboardingScenario(t, db)
	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := testpkg.CreateTestStaff(t, db, "Race", "Actor")
	subject := testpkg.CreateTestStaff(t, db, "Race", "Subject")
	colleague := testpkg.CreateTestStaff(t, db, "Race", "Colleague")
	room := testpkg.CreateTestRoom(t, db, "Offboarding day writer race")
	template := testpkg.CreateTestActivityGroup(t, db, "Offboarding race series")
	secondDay := dayWriterRaceDate.AddDays(1)
	period := testpkg.CreateTestCalendarPeriod(t, db, "Offboarding race period", dayWriterRaceDate, secondDay)
	occurrence := testpkg.ActivityInstanceOpts{ActivityGroupID: &template.ID, CalendarPeriodID: &period.ID}
	first := testpkg.CreateTestActivityInstance(t, db, dayWriterRaceDate, room.ID, occurrence)
	testpkg.CreateTestInstanceStaff(t, db, first.ID, subject.ID, testpkg.InstanceStaffOpts{})
	second := testpkg.CreateTestActivityInstance(t, db, secondDay, room.ID, occurrence)
	ctx := testpkg.Ctx(t)
	for _, staffID := range []int64{subject.ID, colleague.ID} {
		_, err := db.NewRaw(`INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from) VALUES (?, ?, ?, FALSE, ?)`,
			testpkg.Tenant(t), staffID, template.ID, dayWriterRaceDate).Exec(ctx)
		require.NoError(t, err)
	}
	workflow, err := api.NewStaffOffboardingWorkflow(sc.deps)
	require.NoError(t, err)

	locked, release := make(chan struct{}), make(chan struct{})
	barrier := startRaceTx(t, db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`SELECT id FROM schedule.activity_instances WHERE id = ? FOR UPDATE`, second.ID).Exec(ctx); err != nil {
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
	change := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		return api.AlignSeriesStaffForTest(ctx, module, template.ID, []int64{colleague.ID, subject.ID}, dayWriterRaceDate)
	})
	awaitBlockedBy(t, change, barrier.pid, "the series roster change")
	offboarding := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		actorCtx := context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actor.ID, Username: "test-admin"})
		_, err := workflow.Offboard(actorCtx, subject.ID)
		return err
	})
	awaitBlockedBy(t, offboarding, change.pid, "offboarding")
	close(release)
	requireFinishedWithoutDeadlock(t, barrier, "the barrier")
	requireFinishedWithoutDeadlock(t, change, "the series roster change")
	requireFinishedWithoutDeadlock(t, offboarding, "offboarding")

	type staffRow struct {
		InstanceID int64 `bun:"instance_id"`
		StaffID    int64 `bun:"staff_id"`
	}
	var rows []staffRow
	require.NoError(t, db.NewRaw(`SELECT instance_id, staff_id FROM schedule.instance_staff WHERE tenant_id = ? AND instance_id IN (?, ?) ORDER BY instance_id, staff_id`,
		testpkg.Tenant(t), first.ID, second.ID).Scan(ctx, &rows))
	assert.Equal(t, []staffRow{{first.ID, colleague.ID}, {second.ID, colleague.ID}}, rows,
		"the change added S2 to both occurrences; offboarding removed S's existing and new rows")
	var retired bool
	require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.staff_school_memberships WHERE id = ?`, subject.ID).Scan(ctx, &retired))
	assert.True(t, retired, "the subject is retired")
}

// O1 for the Vertretungsplan save (the real ApplyDeviations day writer): Y
// and S are both staffed on T, and one save marks both absent. A third
// transaction holds Y's staffing row, so the save stops after it locked T
// for Y's row and before it reaches S. Offboarding of S runs meanwhile.
// Before #6583 the save took S's staff row only for S's own row, after T:
// 40P01 against offboarding, which holds S and waits for T. Now the save
// takes the staff rows of every person it names before its first block row.
func TestOffboardingAndDeviationSaveOfTheSameStaffDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	sc := newOffboardingScenario(t, db)
	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := testpkg.CreateTestStaff(t, db, "Race", "Actor")
	absent := testpkg.CreateTestStaff(t, db, "Race", "Absent")
	subject := testpkg.CreateTestStaff(t, db, "Race", "Subject")
	room := testpkg.CreateTestRoom(t, db, "Offboarding deviation race")
	block := testpkg.CreateTestActivityInstance(t, db, dayWriterRaceDate, room.ID, testpkg.ActivityInstanceOpts{Title: "Race deviation block"})
	absentRow := testpkg.CreateTestInstanceStaff(t, db, block.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, block.ID, subject.ID, testpkg.InstanceStaffOpts{})
	workflow, err := api.NewStaffOffboardingWorkflow(sc.deps)
	require.NoError(t, err)

	locked, release := make(chan struct{}), make(chan struct{})
	barrier := startRaceTx(t, db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`SELECT id FROM schedule.instance_staff WHERE id = ? FOR UPDATE`, absentRow.ID).Exec(ctx); err != nil {
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
	save := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		return api.MarkStaffAbsentForTest(ctx, module, block.ID, absent.ID, subject.ID)
	})
	awaitBlockedBy(t, save, barrier.pid, "the deviation save")
	offboarding := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		actorCtx := context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actor.ID, Username: "test-admin"})
		_, err := workflow.Offboard(actorCtx, subject.ID)
		return err
	})
	awaitBlockedBy(t, offboarding, save.pid, "offboarding")
	close(release)
	requireFinishedWithoutDeadlock(t, barrier, "the barrier")
	requireFinishedWithoutDeadlock(t, save, "the deviation save")
	requireFinishedWithoutDeadlock(t, offboarding, "offboarding")

	ctx := testpkg.Ctx(t)
	type staffRow struct {
		StaffID  int64 `bun:"staff_id"`
		IsAbsent bool  `bun:"is_absent"`
	}
	var rows []staffRow
	require.NoError(t, db.NewRaw(`SELECT staff_id, is_absent FROM schedule.instance_staff WHERE tenant_id = ? AND instance_id = ? ORDER BY staff_id`,
		testpkg.Tenant(t), block.ID).Scan(ctx, &rows))
	assert.Equal(t, []staffRow{{absent.ID, true}}, rows, "the save marked Y absent; offboarding removed S's row")
}

// O1 for the Sammel-Vertretung (the real ApplyBulkSubstitution day writer):
// S covers the absent Y on T, where S already co-supervises. The save marks
// Y's row absent (locking T) and records the substitution in the
// Änderungsprotokoll, whose foreign key takes a key share on S's staff row.
// A third transaction holds Y's staffing row, so the save stops between the
// two. Before #6583 the key share came after T: 40P01 against offboarding of
// S. Now the save takes the staff rows of Y and S before its first block row.
func TestOffboardingAndBulkSubstitutionOfTheSameStaffDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	sc := newOffboardingScenario(t, db)
	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := testpkg.CreateTestStaff(t, db, "Race", "Actor")
	absent := testpkg.CreateTestStaff(t, db, "Race", "Absent")
	subject := testpkg.CreateTestStaff(t, db, "Race", "Subject")
	room := testpkg.CreateTestRoom(t, db, "Offboarding bulk race")
	block := testpkg.CreateTestActivityInstance(t, db, dayWriterRaceDate, room.ID, testpkg.ActivityInstanceOpts{Title: "Race bulk block"})
	absentRow := testpkg.CreateTestInstanceStaff(t, db, block.ID, absent.ID, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, db, block.ID, subject.ID, testpkg.InstanceStaffOpts{})
	workflow, err := api.NewStaffOffboardingWorkflow(sc.deps)
	require.NoError(t, err)

	locked, release := make(chan struct{}), make(chan struct{})
	barrier := startRaceTx(t, db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`SELECT id FROM schedule.instance_staff WHERE id = ? FOR UPDATE`, absentRow.ID).Exec(ctx); err != nil {
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
	save := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		return api.SubstituteStaffForTest(ctx, module, absent.ID, subject.ID, dayWriterRaceDate)
	})
	awaitBlockedBy(t, save, barrier.pid, "the bulk substitution")
	offboarding := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		actorCtx := context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actor.ID, Username: "test-admin"})
		_, err := workflow.Offboard(actorCtx, subject.ID)
		return err
	})
	awaitBlockedBy(t, offboarding, save.pid, "offboarding")
	close(release)
	requireFinishedWithoutDeadlock(t, barrier, "the barrier")
	requireFinishedWithoutDeadlock(t, save, "the bulk substitution")
	requireFinishedWithoutDeadlock(t, offboarding, "offboarding")

	ctx := testpkg.Ctx(t)
	type staffRow struct {
		StaffID  int64 `bun:"staff_id"`
		IsAbsent bool  `bun:"is_absent"`
	}
	var rows []staffRow
	require.NoError(t, db.NewRaw(`SELECT staff_id, is_absent FROM schedule.instance_staff WHERE tenant_id = ? AND instance_id = ? ORDER BY staff_id`,
		testpkg.Tenant(t), block.ID).Scan(ctx, &rows))
	assert.Equal(t, []staffRow{{absent.ID, true}}, rows, "the save marked Y absent; offboarding removed S's row")
}

// O1 for the re-plan (the real ReplanWeek day writer): S and S0 supervise
// the series, S0 as the lead, and both are planned on its occurrence T. The
// re-plan deletes T and materializes it again; the materializer copies the
// lead first. A third transaction holds S0's staff row. Before #6583 the
// re-plan held T's deleted rows while it waited for S0 and then for S, and
// offboarding of S held S and waited for T's rows: 40P01. Now the re-plan
// takes the staff rows of the occurrences it deletes before the delete.
func TestOffboardingAndReplanOfTheSameStaffDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	sc := newOffboardingScenario(t, db)
	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	actor := testpkg.CreateTestStaff(t, db, "Race", "Actor")
	subject := testpkg.CreateTestStaff(t, db, "Race", "Subject")
	lead := testpkg.CreateTestStaff(t, db, "Race", "Lead")
	require.Less(t, subject.ID, lead.ID, "the re-plan takes the subject first and then waits for the lead")
	room := testpkg.CreateTestRoom(t, db, "Offboarding replan race")
	period := testpkg.CreateTestCalendarPeriod(t, db, "Offboarding replan period", replanRaceDate, replanRaceDate.AddDays(27))
	testpkg.SetCalendarPeriodActive(t, db, period, true)
	template := testpkg.CreateTestActivityGroup(t, db, "Offboarding replan series")
	frame := testpkg.CreateTestTimeframeForTenant(t, db, tenantID, "Offboarding replan frame")
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`UPDATE schedule.timeframes SET start_time = '14:00', end_time = '15:00' WHERE id = ? AND tenant_id = ?`, []any{frame.ID, tenantID}},
		{`UPDATE activities.groups SET is_template = TRUE, planned_room_id = ?, calendar_period_id = ? WHERE id = ? AND tenant_id = ?`, []any{room.ID, period.ID, template.ID, tenantID}},
		{`INSERT INTO activities.schedules (tenant_id, weekday, timeframe_id, activity_group_id, week_pattern) VALUES (?, ?, ?, ?, 0)`, []any{tenantID, isoWeekday(replanRaceDate), frame.ID, template.ID}},
		{`INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from) VALUES (?, ?, ?, TRUE, ?)`, []any{tenantID, lead.ID, template.ID, replanRaceDate}},
		{`INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from) VALUES (?, ?, ?, FALSE, ?)`, []any{tenantID, subject.ID, template.ID, replanRaceDate}},
	} {
		_, err := db.NewRaw(statement.query, statement.args...).Exec(ctx)
		require.NoError(t, err)
	}
	require.NoError(t, api.MaterializeDayForTest(ctx, module, replanRaceDate))
	require.ElementsMatch(t, []int64{lead.ID, subject.ID}, occurrenceStaff(t, db, template.ID), "the occurrence plans both supervisors")
	workflow, err := api.NewStaffOffboardingWorkflow(sc.deps)
	require.NoError(t, err)

	locked, release := make(chan struct{}), make(chan struct{})
	barrier := startRaceTx(t, db, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE`, lead.ID).Exec(ctx); err != nil {
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
	replan := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		return api.ReplanTemplateDayForTest(ctx, module, template.ID, replanRaceDate)
	})
	awaitBlockedBy(t, replan, barrier.pid, "the re-plan")
	offboarding := startRaceTx(t, db, func(ctx context.Context, _ bun.Tx) error {
		actorCtx := context.WithValue(ctx, offboardingTestActorKey{}, staffoffboarding.Actor{StaffID: actor.ID, Username: "test-admin"})
		_, err := workflow.Offboard(actorCtx, subject.ID)
		return err
	})
	awaitBlockedBy(t, offboarding, replan.pid, "offboarding")
	close(release)
	requireFinishedWithoutDeadlock(t, barrier, "the barrier")
	requireFinishedWithoutDeadlock(t, replan, "the re-plan")
	requireFinishedWithoutDeadlock(t, offboarding, "offboarding")

	assert.Equal(t, []int64{lead.ID}, occurrenceStaff(t, db, template.ID), "the re-plan planned both again; offboarding removed the subject")
}

// occurrenceStaff lists the staff planned on the template's occurrence of
// the re-plan's race day.
func occurrenceStaff(t *testing.T, db *bun.DB, templateID int64) []int64 {
	t.Helper()
	var staff []int64
	require.NoError(t, db.NewRaw(`
		SELECT s.staff_id FROM schedule.instance_staff s
		JOIN schedule.activity_instances i ON i.id = s.instance_id AND i.tenant_id = s.tenant_id
		WHERE s.tenant_id = ? AND i.activity_group_id = ? AND i.date = ? ORDER BY s.staff_id`,
		testpkg.Tenant(t), templateID, replanRaceDate).Scan(testpkg.Ctx(t), &staff))
	return staff
}

// isoWeekday is the date's ISO weekday (1 = Monday … 7 = Sunday).
func isoWeekday(date calendar.Date) int {
	if weekday := int(date.Weekday()); weekday != 0 {
		return weekday
	}
	return 7
}

// O1, the admission: School Membership's staff row lock goes through the
// unit of work's lock order. After it, the transaction holds the staff row
// class, and a day lock (or any earlier class) is refused before it waits.
func TestStaffRowLockIsAdmittedThroughTheLockOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	sc := newOffboardingScenario(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Lock", "Order")
	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		_, err := sc.deps.Membership.FindStaffForMutation(ctx, staff.ID)
		require.NoError(t, err)
		require.True(t, tenant.HoldsLock(ctx, tenant.LockStaffRows, tenant.LockIDKey(staff.ID)), "the staff row lock is in the ledger")
		return tenant.OrderLock(ctx, tenant.LockScheduleDay, dayWriterRaceDate.String())
	})
	require.ErrorIs(t, err, tenant.ErrLockOrder, "a day lock after a staff row is refused")
}
