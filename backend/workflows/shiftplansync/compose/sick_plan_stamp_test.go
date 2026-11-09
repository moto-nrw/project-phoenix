// Hermetic integration tests for the sick stamp on block assignments that
// are rebuilt or created after a sick report (#6292).
//
// Invariants:
//   - I1: an assignment a sick report made absent names the report
//     (sick_absence_id), also after an instance edit and a re-plan.
//   - I2: a block planned after the report comes in is planned absent on the
//     report's full sick days, with the stamp; other days stay present.
//   - I3: shortening or deleting the report restores every assignment of
//     I1 and I2 on the removed days.
//   - I4: a re-plan keeps a presence a planner restored on a sick day.
//   - I5: a materialization and a concurrent sick-report write serialize on
//     the day locks (recurrence → days ascending): no new block keeps the
//     sick person present, none keeps the stamp of a deleted report.
//   - I6: an edit that adds a person or moves a block takes the sick state
//     of the block's date; a stamp never lands outside its report's days.
//   - I7: deleting a report takes all its day locks once, ascending, so it
//     cannot deadlock against an ascending day-lock writer.
//   - I8: a series roster change that adds a person to already planned
//     occurrences plans them absent on their full sick days.
//   - I9: merging overlapping reports moves every stamp of a merged report to
//     the survivor, also the stamps a concurrent materialization writes.
//
// The sick reports run through the real Workforce absence writes and their
// cascade; the blocks through the real materializer and instance lifecycle.
package compose_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Mondays after the suite's fixed clock (2026-08-24).
var (
	stampWeekOne   = calendar.NewDate(2026, time.August, 31)
	stampWeekTwo   = calendar.NewDate(2026, time.September, 7)
	stampWeekThree = calendar.NewDate(2026, time.September, 14)
)

// createSupervisedTemplate plans the subject as the lead of a Monday
// template (14:00-15:00) over an active period and returns the template.
func (e *sickCascadeEnv) createSupervisedTemplate(t *testing.T) int64 {
	t.Helper()
	suffix := time.Now().UnixNano()
	period := testpkg.CreateTestCalendarPeriod(t, e.db, fmt.Sprintf("Stamp-Period-%d", suffix),
		calendar.NewDate(2026, time.August, 1), calendar.NewDate(2026, time.December, 31))
	testpkg.SetCalendarPeriodActive(t, e.db, period, true)
	group := testpkg.CreateTestActivityGroup(t, e.db, fmt.Sprintf("Stamp-Template-%d", suffix))
	frame := testpkg.CreateTestTimeframeForTenant(t, e.db, e.tenantID, fmt.Sprintf("Stamp-Frame-%d", suffix))
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`UPDATE schedule.timeframes SET start_time = '14:00', end_time = '15:00' WHERE id = ? AND tenant_id = ?`, []any{frame.ID, e.tenantID}},
		{`UPDATE activities.groups SET is_template = TRUE, planned_room_id = ?, calendar_period_id = ? WHERE id = ? AND tenant_id = ?`, []any{e.roomID, period.ID, group.ID, e.tenantID}},
		{`INSERT INTO activities.schedules (tenant_id, weekday, timeframe_id, activity_group_id, week_pattern) VALUES (?, 1, ?, ?, 0)`, []any{e.tenantID, frame.ID, group.ID}},
		{`INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from) VALUES (?, ?, ?, TRUE, ?)`, []any{e.tenantID, e.subject.ID, group.ID, calendar.NewDate(2026, time.August, 1)}},
	} {
		_, err := e.db.NewRaw(statement.query, statement.args...).Exec(e.ctx)
		require.NoError(t, err)
	}
	return group.ID
}

func (e *sickCascadeEnv) materialize(t *testing.T, from, to calendar.Date) {
	t.Helper()
	_, err := e.timetable.Materialization.MaterializeForTenant(e.ctx, from, to, timetable.MaterializationSourceManual)
	require.NoError(t, err)
}

// templateRow loads the staff member's assignment on the template's
// occurrence of date.
func (e *sickCascadeEnv) templateRow(t *testing.T, templateID, staffID int64, date calendar.Date) *timetable.InstanceStaff {
	t.Helper()
	var id int64
	require.NoError(t, e.db.NewRaw(`
		SELECT s.id FROM schedule.instance_staff s
		JOIN schedule.activity_instances i ON i.id = s.instance_id AND i.tenant_id = s.tenant_id
		WHERE s.tenant_id = ? AND s.staff_id = ? AND i.activity_group_id = ? AND i.date = ?`,
		e.tenantID, staffID, templateID, date).Scan(e.ctx, &id))
	return e.reloadRow(t, id)
}

func (e *sickCascadeEnv) shortenSickAbsence(t *testing.T, absenceID int64, start, end calendar.Date) {
	t.Helper()
	startText, endText := start.String(), end.String()
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.UpdateAbsence(ctx, e.subject.ID, nil, absenceID, workforce.UpdateAbsenceRequest{
			DateStart: &startText,
			DateEnd:   &endText,
		})
		return err
	})
}

func assertSickStamp(t *testing.T, row *timetable.InstanceStaff, absenceID int64, msg string) {
	t.Helper()
	assert.True(t, row.IsAbsent, msg+": absent")
	if assert.NotNil(t, row.SickAbsenceID, msg+": stamp") {
		assert.Equal(t, absenceID, *row.SickAbsenceID, msg+": stamp names the report")
	}
	if assert.NotNil(t, row.AbsenceReason, msg+": reason") {
		assert.Equal(t, timetable.SickBlockAbsenceReason, *row.AbsenceReason, msg+": neutral reason")
	}
}

func assertPresent(t *testing.T, row *timetable.InstanceStaff, msg string) {
	t.Helper()
	assert.False(t, row.IsAbsent, msg+": present")
	assert.Nil(t, row.SickAbsenceID, msg+": no stamp")
}

// I2 + I3: a report filed before its weeks are materialized.
func TestSickReport_BlocksMaterializedLaterArePlannedAbsent(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekThree)

	e.materialize(t, stampWeekOne, stampWeekThree)

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "day before the report")
	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), absenceID, "first sick day")
	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekThree), absenceID, "last sick day")

	e.shortenSickAbsence(t, absenceID, stampWeekThree, stampWeekThree)
	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), "removed sick day")
	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekThree), absenceID, "kept sick day")
}

// I2: a half-day report never makes a block absent; a full-day report of
// the same person does.
func TestSickReport_MaterializationSkipsHalfDayReports(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: "sick",
			DateStart:   stampWeekOne.String(),
			DateEnd:     stampWeekOne.String(),
			HalfDay:     true,
		})
		return err
	})
	fullDayID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)

	e.materialize(t, stampWeekOne, stampWeekTwo)

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "half sick day")
	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), fullDayID, "full sick day")
}

// I1 + I3 for an instance edit, with and without a roster change.
func TestSickReport_EditedBlockKeepsStampSoShortenedReportRestoresPresence(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		addStaff bool
	}{
		{name: "roster unchanged"},
		{name: "staff added", addStaff: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := buildSickCascadeEnv(t)
			templateID := e.createSupervisedTemplate(t)
			e.materialize(t, stampWeekOne, stampWeekTwo)
			absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekTwo)
			row := e.templateRow(t, templateID, e.subject.ID, stampWeekOne)
			assertSickStamp(t, row, absenceID, "cascade marked the existing block")

			staffIDs := []int64{e.subject.ID}
			if tc.addStaff {
				staffIDs = append(staffIDs, e.sub.ID)
			}
			e.updateInstance(t, row.InstanceID, templateID, staffIDs)

			assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), absenceID, "edited block")
			e.shortenSickAbsence(t, absenceID, stampWeekTwo, stampWeekTwo)
			assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "edited block after the report was shortened")
		})
	}
}

func (e *sickCascadeEnv) updateInstance(t *testing.T, instanceID, templateID int64, staffIDs []int64) {
	t.Helper()
	e.updateInstanceOn(t, instanceID, templateID, staffIDs, "")
}

// updateInstanceOn edits a planned block; a non-empty date moves it.
func (e *sickCascadeEnv) updateInstanceOn(t *testing.T, instanceID, templateID int64, staffIDs []int64, date calendar.Date) {
	t.Helper()
	instances, err := testpkg.TenantQuery1(t, e.ctx, e.db, e.repos.InstanceReads().GetActivityInstancesByID, []int64{instanceID})
	require.NoError(t, err)
	instance := instances[instanceID]
	require.NotNil(t, instance)
	e.inTx(t, func(ctx context.Context) error {
		if date == "" {
			date = instance.Date
		}
		_, err := e.timetable.Instance.UpdatePlanned(ctx, instanceID, timetable.UpdateInstanceInput{
			Date:            date,
			StartTime:       instance.StartTime,
			EndTime:         instance.EndTime,
			Title:           instance.Title + " (bearbeitet)",
			RoomID:          instance.RoomID,
			ActivityGroupID: &templateID,
			StaffIDs:        staffIDs,
		}, nil)
		return err
	})
}

// I6: the edit adds a sick person, and moves blocks onto and off a sick day.
func TestSickReport_EditTakesSickStateOfTheBlocksDate(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekThree)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	var substituteSickID int64
	e.inTx(t, func(ctx context.Context) error {
		created, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.sub.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: "sick", DateStart: stampWeekOne.String(), DateEnd: stampWeekOne.String(),
		})
		if err == nil {
			substituteSickID = created.ID
		}
		return err
	})

	weekOne := e.templateRow(t, templateID, e.subject.ID, stampWeekOne)
	e.updateInstance(t, weekOne.InstanceID, templateID, []int64{e.subject.ID, e.sub.ID})
	assertSickStamp(t, e.templateRow(t, templateID, e.sub.ID, stampWeekOne), substituteSickID, "sick person added by the edit")
	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "healthy person kept by the edit")

	sickDay := e.templateRow(t, templateID, e.subject.ID, stampWeekTwo)
	assertSickStamp(t, sickDay, absenceID, "cascade marked the sick day")
	movedOff := stampWeekTwo.AddDays(1)
	e.updateInstanceOn(t, sickDay.InstanceID, templateID, []int64{e.subject.ID}, movedOff)
	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, movedOff), "block moved off the sick day")

	weekThree := e.templateRow(t, templateID, e.subject.ID, stampWeekThree)
	e.updateInstanceOn(t, weekThree.InstanceID, templateID, []int64{e.subject.ID}, stampWeekTwo)
	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), absenceID, "block moved onto the sick day")
}

// I1 + I3 for a re-plan.
func TestSickReport_ReplannedBlockKeepsStampSoShortenedReportRestoresPresence(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekTwo)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekTwo)

	e.replan(t, templateID, stampWeekOne)

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), absenceID, "re-planned block")
	e.shortenSickAbsence(t, absenceID, stampWeekTwo, stampWeekTwo)
	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "re-planned block after the report was shortened")
}

// I4: the re-plan regenerates the template; it does not undo a planner's
// decision that the sick person works this block after all.
func TestSickReport_ReplanKeepsRestoredPresence(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekOne)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
	row := e.templateRow(t, templateID, e.subject.ID, stampWeekOne)
	assertSickStamp(t, row, absenceID, "cascade marked the block")
	// The persisted state of the planner's "wieder anwesend" (applyPresence).
	_, err := e.db.NewRaw(`UPDATE schedule.instance_staff SET is_absent = FALSE, absence_reason = NULL, sick_absence_id = NULL WHERE id = ? AND tenant_id = ?`,
		row.ID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)

	e.replan(t, templateID, stampWeekOne)

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "restored presence survives the re-plan")
}

func (e *sickCascadeEnv) replan(t *testing.T, templateID int64, week calendar.Date) {
	t.Helper()
	_, err := e.timetable.Instance.ReplanWeek(e.ctx, week, week.AddDays(6), &templateID, nil)
	require.NoError(t, err)
}

// I5: a sick report holding its day while a materialization runs. The run
// waits for the report's commit and then plans the person absent.
func TestSickReport_MaterializationWaitsForConcurrentReport(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	created := make(chan int64, 1)
	release := make(chan struct{})
	reportDone := make(chan error, 1)
	go func() {
		reportDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			absence, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
				AbsenceType: "sick",
				DateStart:   stampWeekOne.String(),
				DateEnd:     stampWeekOne.String(),
			})
			if err != nil {
				return err
			}
			created <- absence.ID
			<-release
			return nil
		})
	}()
	var absenceID int64
	select {
	case absenceID = <-created:
	case err := <-reportDone:
		require.FailNow(t, "sick report failed before it held its day", "%v", err)
	}

	e.expectBlockedUntilRelease(t, release, reportDone)

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), absenceID, "block materialized beside the open report")
}

// I5 for the deletion: the run waits for the deleting transaction and then
// stamps nothing, instead of stamping the report being deleted.
func TestSickReport_MaterializationWaitsForConcurrentDeletion(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
	deleted := make(chan struct{})
	release := make(chan struct{})
	deletionDone := make(chan error, 1)
	go func() {
		deletionDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			if err := e.absences.SickReports.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID); err != nil {
				return err
			}
			close(deleted)
			<-release
			return nil
		})
	}()
	select {
	case <-deleted:
	case err := <-deletionDone:
		require.FailNow(t, "deletion failed before it held its day", "%v", err)
	}

	e.expectBlockedUntilRelease(t, release, deletionDone)

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "block materialized beside the deletion")
}

// expectBlockedUntilRelease starts the week's materialization while a sick
// report write holds its day, requires the run to wait, then commits the
// write and requires both to succeed.
func (e *sickCascadeEnv) expectBlockedUntilRelease(t *testing.T, release chan struct{}, writeDone chan error) {
	t.Helper()
	materialized := make(chan error, 1)
	go func() {
		_, err := e.timetable.Materialization.MaterializeForTenant(e.ctx, stampWeekOne, stampWeekOne, timetable.MaterializationSourceManual)
		materialized <- err
	}()
	if returned, err := e.awaitDayLockWaiter(t, stampWeekOne, materialized); returned {
		close(release)
		require.NoError(t, <-writeDone)
		require.FailNow(t, "materialization did not wait for the sick report's day lock", "returned early: %v", err)
	}
	close(release)
	require.NoError(t, <-writeDone)
	require.NoError(t, <-materialized)
}

// awaitDayLockWaiter blocks until a transaction waits for the day lock of
// date in this test's tenant, or until done delivers first. It reports
// whether done delivered, with its value. A run that blocks on another lock
// instead is released after a bounded wait: its outcome then shows whether it
// read the reports before or after the write committed.
func (e *sickCascadeEnv) awaitDayLockWaiter(t *testing.T, date calendar.Date, done <-chan error) (bool, error) {
	t.Helper()
	return e.awaitLockWaiter(t, timetable.SubstituteDayLockKey(e.tenantID, date), "advisory", done)
}

// awaitLockWaiter blocks until a transaction whose statement contains needle
// waits for a lock of waitEvent ("advisory", "transactionid"), or until done
// delivers first, at most three seconds.
func (e *sickCascadeEnv) awaitLockWaiter(t *testing.T, needle, waitEvent string, done <-chan error) (bool, error) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-done:
			return true, err
		default:
		}
		var waiting int
		require.NoError(t, e.db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND wait_event = ?
			  AND query LIKE '%' || ? || '%'`, waitEvent, needle).Scan(context.Background(), &waiting))
		if waiting > 0 {
			return false, nil
		}
	}
	return false, nil
}

// lockDay takes the day lock the Timetable owner's adapter takes.
func lockDay(ctx context.Context, tx bun.Tx, tenantID int64, date calendar.Date) error {
	_, err := tx.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", timetable.SubstituteDayLockKey(tenantID, date)).Exec(ctx)
	return err
}

// I7: a stamp outside the report's range (left by an older edit) is cleared
// under its day lock taken in the same ascending pass as the range days. A
// writer that holds the earlier day and then takes the later one, the
// documented ascending order, must not deadlock against the deletion.
func TestSickReport_DeletionTakesItsDayLocksAscending(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekTwo)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	outside := e.templateRow(t, templateID, e.subject.ID, stampWeekOne)
	_, err := e.db.NewRaw(`UPDATE schedule.instance_staff SET is_absent = TRUE, absence_reason = ?, sick_absence_id = ? WHERE id = ? AND tenant_id = ?`,
		timetable.SickBlockAbsenceReason, absenceID, outside.ID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)

	holding := make(chan struct{})
	proceed := make(chan struct{})
	writerDone := make(chan error, 1)
	go func() {
		writerDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, tx bun.Tx) error {
			if err := lockDay(ctx, tx, e.tenantID, stampWeekOne); err != nil {
				return err
			}
			close(holding)
			<-proceed
			return lockDay(ctx, tx, e.tenantID, stampWeekTwo)
		})
	}()
	<-holding
	deletionDone := make(chan error, 1)
	go func() {
		deletionDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			return e.absences.SickReports.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
		})
	}()
	if returned, err := e.awaitDayLockWaiter(t, stampWeekOne, deletionDone); returned {
		close(proceed)
		require.NoError(t, <-writerDone)
		require.FailNow(t, "deletion did not wait for the stamped day", "returned early: %v", err)
	}
	close(proceed)
	require.NoError(t, <-writerDone, "ascending writer must not deadlock")
	require.NoError(t, <-deletionDone, "deletion must not deadlock")

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "stamp outside the range")
	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), "sick day of the deleted report")
}

// I8: the series roster gains a supervisor who is sick in week two; aligning
// the already planned occurrences adds them absent there, present elsewhere.
func TestSickReport_SeriesRosterChangeAddsSickPersonAbsent(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekTwo)
	var absenceID int64
	e.inTx(t, func(ctx context.Context) error {
		created, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.sub.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: "sick", DateStart: stampWeekTwo.String(), DateEnd: stampWeekTwo.String(),
		})
		if err == nil {
			absenceID = created.ID
		}
		return err
	})
	_, err := e.db.NewRaw(`INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from) VALUES (?, ?, ?, FALSE, ?)`,
		e.tenantID, e.sub.ID, templateID, calendar.NewDate(2026, time.August, 1)).Exec(e.ctx)
	require.NoError(t, err)

	e.inTx(t, func(ctx context.Context) error {
		return e.timetable.TimetableData.Templates.AlignPlannedInstanceStaff(ctx, templateID, []int64{e.sub.ID}, stampWeekOne)
	})

	assertPresent(t, e.templateRow(t, templateID, e.sub.ID, stampWeekOne), "added on a healthy day")
	assertSickStamp(t, e.templateRow(t, templateID, e.sub.ID, stampWeekTwo), absenceID, "added on a sick day")
}

// I8 + I5: the series change waits for a sick report that holds its day and
// then adds the person absent.
func TestSickReport_SeriesRosterChangeWaitsForConcurrentReport(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekOne)
	_, err := e.db.NewRaw(`INSERT INTO activities.supervisors (tenant_id, staff_id, group_id, is_primary, valid_from) VALUES (?, ?, ?, FALSE, ?)`,
		e.tenantID, e.sub.ID, templateID, calendar.NewDate(2026, time.August, 1)).Exec(e.ctx)
	require.NoError(t, err)
	created := make(chan int64, 1)
	release := make(chan struct{})
	reportDone := make(chan error, 1)
	go func() {
		reportDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			absence, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.sub.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
				AbsenceType: "sick", DateStart: stampWeekOne.String(), DateEnd: stampWeekOne.String(),
			})
			if err != nil {
				return err
			}
			created <- absence.ID
			<-release
			return nil
		})
	}()
	var absenceID int64
	select {
	case absenceID = <-created:
	case err := <-reportDone:
		require.FailNow(t, "sick report failed before it held its day", "%v", err)
	}

	aligned := make(chan error, 1)
	go func() {
		aligned <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			return e.timetable.TimetableData.Templates.AlignPlannedInstanceStaff(ctx, templateID, []int64{e.sub.ID}, stampWeekOne)
		})
	}()
	if returned, err := e.awaitDayLockWaiter(t, stampWeekOne, aligned); returned {
		close(release)
		require.NoError(t, <-reportDone)
		require.FailNow(t, "series change did not wait for the sick report's day lock", "returned early: %v", err)
	}
	close(release)
	require.NoError(t, <-reportDone)
	require.NoError(t, <-aligned)

	assertSickStamp(t, e.templateRow(t, templateID, e.sub.ID, stampWeekOne), absenceID, "added beside the open report")
}

// I9: a materialization stamps week two for report S and holds its day locks
// while a bridging report merges S into P. The merge waits for the
// materialization (the staff row lock the absence write takes conflicts with
// the key share the materialized rows hold), then moves the materialized
// stamp to P instead of leaving it on the deleted S.
func TestSickReport_MergeMovesStampsOfConcurrentMaterialization(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	survivorID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
	mergedID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)

	materialized := make(chan struct{})
	release := make(chan struct{})
	materializationDone := make(chan error, 1)
	go func() {
		materializationDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			if _, err := e.timetable.Materialization.MaterializeForTenant(ctx, stampWeekTwo, stampWeekTwo, timetable.MaterializationSourceManual); err != nil {
				return err
			}
			close(materialized)
			<-release
			return nil
		})
	}()
	select {
	case <-materialized:
	case err := <-materializationDone:
		require.FailNow(t, "materialization failed before it held its days", "%v", err)
	}

	mergeDone := make(chan error, 1)
	go func() {
		mergeDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			_, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
				AbsenceType: "sick", DateStart: stampWeekOne.String(), DateEnd: stampWeekTwo.String(),
			})
			return err
		})
	}()
	if returned, err := e.awaitLockWaiter(t, "users.staff", "transactionid", mergeDone); returned {
		close(release)
		require.NoError(t, <-materializationDone)
		require.FailNow(t, "merge did not wait for the materialization", "returned early: %v", err)
	}
	close(release)
	require.NoError(t, <-materializationDone)
	require.NoError(t, <-mergeDone)

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), survivorID, "materialized row of the merged report")
	_, err := e.shifts.FindStaffAbsence(e.ctx, mergedID)
	require.Error(t, err, "the merged report is gone")
}
