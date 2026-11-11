package httpintegration_test

// Staff rows are one ascending class of the unit of work's lock order
// (#6702): a writer that plans several people determines all of them first
// and locks their staff rows in one ascending batch before its first row.
// Before, multi-person writers took the staff rows one by one in input or
// template order, so two writers naming the same two people in another
// order could each hold one and wait for the other: PostgreSQL aborted one
// of them with 40P01, or the ledger now refuses the lower person after the
// higher one. These tests run the real writers against each other with a
// third transaction as barrier and pg_blocking_pids as proof of waiting.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// orderTx is one tenant transaction of a race: its backend, then its outcome.
type orderTx struct {
	pid  int64
	done chan error
}

func startOrderTx(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64, write func(ctx context.Context, tx bun.Tx) error) orderTx {
	t.Helper()
	pid, done := make(chan int64, 1), make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, ctx, db, tenantID, func(ctx context.Context, tx bun.Tx) error {
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
		return orderTx{pid: backend, done: done}
	case err := <-done:
		require.FailNow(t, "race transaction failed before it started", "%v", err)
		return orderTx{}
	}
}

// awaitOrderTxBlockedBy requires run to wait for a lock blocker holds, as
// PostgreSQL reports it; the ticker only paces the observation.
func awaitOrderTxBlockedBy(t *testing.T, db *bun.DB, run orderTx, blocker int64, what string) {
	t.Helper()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		var blocked bool
		require.NoError(t, db.NewRaw(`SELECT ?::int = ANY(pg_blocking_pids(?::int))`, blocker, run.pid).Scan(context.Background(), &blocked))
		if blocked {
			return
		}
		select {
		case err := <-run.done:
			require.FailNow(t, what+" finished instead of waiting", "%v", err)
		case <-poll.C:
		}
	}
	require.FailNow(t, what+" never waited for the expected transaction")
}

func requireOrderTxCommitted(t *testing.T, run orderTx, what string) {
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

// holdGroupRow holds an activity row FOR UPDATE until release: a supervisor
// or occurrence insert for that activity key-shares the row and waits there.
func holdGroupRow(t *testing.T, db *bun.DB, ctx context.Context, tenantID, groupID int64) (orderTx, func()) {
	t.Helper()
	locked, release := make(chan struct{}), make(chan struct{})
	holder := startOrderTx(t, db, ctx, tenantID, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`SELECT id FROM activities.groups WHERE id = ? FOR UPDATE`, groupID).Exec(ctx); err != nil {
			return err
		}
		close(locked)
		<-release
		return nil
	})
	select {
	case <-locked:
	case err := <-holder.done:
		require.FailNow(t, "the barrier transaction failed", "%v", err)
	}
	return holder, func() { close(release) }
}

// Two activity supervisor writes name the same two people in reversed
// order. The first stops after its first person (the barrier holds its
// activity row, which the supervisor insert key-shares); the second then
// locks its first person and waits for the other. Before #6702 the first
// write, released, waited for the second's person: 40P01, or now the
// ledger's refusal of the lower person after the higher one.
func TestActivitySupervisorWritesWithReversedPeopleDoNotDeadlock(t *testing.T) {
	t.Parallel()
	s := makeScenario(t, activityContract.WeekdayMonday, calendar.NewDate(2026, time.April, 20))
	_, activities, _, _, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, s.db))
	require.NoError(t, err)
	low := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Order", "Low")
	high := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Order", "High")
	require.Less(t, low.ID, high.ID)
	first := testpkg.CreateTestActivityGroupForTenant(t, s.db, s.tenantID, "Reversed order first")
	second := testpkg.CreateTestActivityGroupForTenant(t, s.db, s.tenantID, "Reversed order second")

	barrier, release := holdGroupRow(t, s.db, s.ctx, s.tenantID, first.ID)
	ascending := startOrderTx(t, s.db, s.ctx, s.tenantID, func(ctx context.Context, _ bun.Tx) error {
		return activities.UpdateGroupSupervisors(ctx, first.ID, []int64{low.ID, high.ID})
	})
	awaitOrderTxBlockedBy(t, s.db, ascending, barrier.pid, "the write with [low, high]")
	descending := startOrderTx(t, s.db, s.ctx, s.tenantID, func(ctx context.Context, _ bun.Tx) error {
		return activities.UpdateGroupSupervisors(ctx, second.ID, []int64{high.ID, low.ID})
	})
	awaitOrderTxBlockedBy(t, s.db, descending, ascending.pid, "the write with [high, low]")
	release()
	requireOrderTxCommitted(t, barrier, "the barrier")
	requireOrderTxCommitted(t, ascending, "the write with [low, high]")
	requireOrderTxCommitted(t, descending, "the write with [high, low]")
	for _, groupID := range []int64{first.ID, second.ID} {
		var staff []int64
		require.NoError(t, s.db.NewRaw(`SELECT staff_id FROM activities.supervisors WHERE tenant_id = ? AND group_id = ? ORDER BY staff_id`, s.tenantID, groupID).Scan(context.Background(), &staff))
		require.Equal(t, []int64{low.ID, high.ID}, staff, "activity %d has both supervisors", groupID)
	}
}

// The materialization copies the supervisors of its templates in name
// order: "Malen-AG" plans high, "Zeichnen-AG" plans low. The barrier holds
// "Zeichnen-AG", so the run stops at that occurrence's insert. An activity
// supervisor write then locks [low, high] ascending and waits. Before
// #6702 the released run copied low after high: 40P01 against the write,
// or now the ledger's refusal. The run now locks every person it copies,
// ascending, before its first occurrence.
func TestMaterializationAndSupervisorWriteDoNotDeadlock(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, time.April, 20)
	s := makeScenario(t, activityContract.WeekdayMonday, day)
	defer s.runCleanup(t)
	_, activities, _, _, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, s.db))
	require.NoError(t, err)
	low := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Order", "Low")
	high := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Order", "High")
	require.Less(t, low.ID, high.ID)
	addPlannedSupervisor(t, s, s.template.ID, high.ID, day)
	later := addScenarioTemplate(t, s, "Zeichnen-AG")
	addPlannedSupervisor(t, s, later.ID, low.ID, day)
	activity := testpkg.CreateTestActivityGroupForTenant(t, s.db, s.tenantID, "Order activity")

	barrier, release := holdGroupRow(t, s.db, s.ctx, s.tenantID, later.ID)
	run := startOrderTx(t, s.db, s.ctx, s.tenantID, func(ctx context.Context, _ bun.Tx) error {
		_, err := s.svc.MaterializeForTenant(ctx, day, day, timetable.MaterializationSourceManual)
		return err
	})
	awaitOrderTxBlockedBy(t, s.db, run, barrier.pid, "the materialization")
	write := startOrderTx(t, s.db, s.ctx, s.tenantID, func(ctx context.Context, _ bun.Tx) error {
		return activities.UpdateGroupSupervisors(ctx, activity.ID, []int64{low.ID, high.ID})
	})
	awaitOrderTxBlockedBy(t, s.db, write, run.pid, "the supervisor write")
	release()
	requireOrderTxCommitted(t, barrier, "the barrier")
	requireOrderTxCommitted(t, run, "the materialization")
	requireOrderTxCommitted(t, write, "the supervisor write")
	for _, templateID := range []int64{s.template.ID, later.ID} {
		require.Len(t, listInstancesForDate(t, s.db, templateID, day), 1, "template %d is materialized", templateID)
	}
}

func addPlannedSupervisor(t *testing.T, s *scenarioSetup, templateID, staffID int64, day calendar.Date) {
	t.Helper()
	row := &activityContract.SupervisorPlanned{StaffID: staffID, GroupID: templateID, ValidFrom: activityContract.Date(day.AddDays(-30))}
	row.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(activitySupervisorPlannedFixture(row)).ModelTableExpr(`activities.supervisors`).Exec(s.ctx)
	require.NoError(t, err)
}

// addScenarioTemplate adds a template with the scenario's category, room,
// timeframe and weekday; its name sets its place in the run's order.
func addScenarioTemplate(t *testing.T, s *scenarioSetup, name string) *activityContract.Group {
	t.Helper()
	template := &activityContract.Group{
		Name: fmt.Sprintf("%s-%d", name, time.Now().UnixNano()), CategoryID: s.categoryID,
		MaxParticipants: 20, IsOpen: true, PlannedRoomID: &s.roomID, IsTemplate: true,
	}
	template.SetTenantID(s.tenantID)
	_, err := s.db.NewInsert().Model(activityGroupFixture(template)).ModelTableExpr(`activities.groups AS "group"`).Exec(s.ctx)
	require.NoError(t, err)
	schedule := &activityContract.Schedule{ActivityGroupID: template.ID, TimeframeID: &s.timeframe.ID, Weekday: s.schedule.Weekday}
	schedule.SetTenantID(s.tenantID)
	_, err = s.db.NewInsert().Model(activityScheduleFixture(schedule)).ModelTableExpr(`activities.schedules`).Exec(s.ctx)
	require.NoError(t, err)
	return template
}

// A template update writes its template row after its roster's staff rows;
// an activity supervisor write against the template's id locks the row in
// its modification check and its people before it refuses the template.
// Before #6702 the update could hold a person and wait for the row while the
// supervisor write held the row and waited for that person: 40P01 for one of
// them instead of the refusal and the update. Now the update locks the
// template row before its days and people (tenant.LockReference), so it
// waits for the supervisor write holding no person.
func TestTemplateUpdateAndActivitySupervisorWriteDoNotDeadlock(t *testing.T) {
	t.Parallel()
	s := makeScenario(t, activityContract.WeekdayMonday, calendar.NewDate(2026, time.April, 20))
	_, activities, _, _, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, s.db))
	require.NoError(t, err)
	high := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Order", "Template")
	require.Less(t, s.staffID, high.ID)
	template := s.template

	locked, releaseStaff := make(chan struct{}), make(chan struct{})
	barrier := startOrderTx(t, s.db, s.ctx, s.tenantID, func(ctx context.Context, tx bun.Tx) error {
		// The strength School Membership's staff row lock takes.
		if _, err := tx.NewRaw(`SELECT id FROM users.staff_school_memberships WHERE id = ? FOR NO KEY UPDATE`, s.staffID).Exec(ctx); err != nil {
			return err
		}
		close(locked)
		<-releaseStaff
		return nil
	})
	<-locked
	supervisors := startOrderTx(t, s.db, s.ctx, s.tenantID, func(ctx context.Context, _ bun.Tx) error {
		allowed, err := activities.CanModifyActivity(ctx, template.ID, s.staffID, false)
		if err != nil || !allowed {
			return fmt.Errorf("modification check: allowed=%t: %w", allowed, err)
		}
		return activities.UpdateGroupSupervisors(ctx, template.ID, []int64{s.staffID, high.ID})
	})
	awaitOrderTxBlockedBy(t, s.db, supervisors, barrier.pid, "the supervisor write")
	update := startOrderTx(t, s.db, s.ctx, s.tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.timetableData.Templates.UpdateTemplate(ctx, timetable.UpdateTemplateCommand{
			TemplateID: template.ID,
			Fields: timetable.TemplateFields{
				Name: template.Name + " geändert", Type: template.Type, CategoryID: template.CategoryID, RoomID: s.roomID,
				MaxParticipants: template.MaxParticipants, CalendarPeriodID: template.CalendarPeriodID,
			},
			Weekdays:        []int{activityContract.WeekdayMonday},
			TimeframeID:     s.timeframe.ID,
			WeekPattern:     s.schedule.WeekPattern,
			RosterValidFrom: calendar.NewDate(2026, time.March, 21),
			StudentIDs:      s.students[:2],
			StaffIDs:        []int64{high.ID},
			PrimaryStaffID:  &high.ID,
			GradeLevelMax:   testpkg.MaxSchoolGradeLevel,
		})
	})
	awaitOrderTxBlockedBy(t, s.db, update, supervisors.pid, "the template update")
	close(releaseStaff)
	requireOrderTxCommitted(t, barrier, "the barrier")
	select {
	case err := <-supervisors.done:
		require.ErrorIs(t, err, activityContract.ErrTimetableTemplateProtected, "the supervisor write ends with its refusal, not a deadlock")
	case <-time.After(30 * time.Second):
		require.FailNow(t, "the supervisor write never finished")
	}
	requireOrderTxCommitted(t, update, "the template update")
}
