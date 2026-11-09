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
//   - I6: an edit that adds a person or moves a block takes the sick state
//     of the block's date; a stamp never lands outside its report's days.
//   - I8: a series roster change that adds a person to already planned
//     occurrences plans them absent on their full sick days.
//
// #6300 adds no lock: concurrent sick-report writes are #6232's.
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
			for _, event := range e.eventsByType(t, stampWeekOne, stampWeekOne, "deviation_dropped_by_edit") {
				assert.Failf(t, "a kept sick absence is no dropped deviation", "event %+v", event)
			}
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
