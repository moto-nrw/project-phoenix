// Hermetic integration tests for the PR #6300 design: every writer of block
// assignments goes through one stamp helper (I-1), including the
// materialization of past days, and a sick person is no substitute. Each test
// is red without the guard it names.
package compose_test

import (
	"context"
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

// A Monday before the suite's fixed cascade clock (2026-08-24).
var pastWeek = calendar.NewDate(2026, time.August, 17)

// createSickAbsenceFor files a full-day sick report for staffID.
func (e *sickCascadeEnv) createSickAbsenceFor(t *testing.T, staffID int64, start, end calendar.Date) int64 {
	t.Helper()
	var absenceID int64
	e.inTx(t, func(ctx context.Context) error {
		created, err := e.absences.SickReports.CreateAbsenceFor(ctx, staffID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: "sick", DateStart: start.String(), DateEnd: end.String(),
		})
		if err == nil {
			absenceID = created.ID
		}
		return err
	})
	return absenceID
}

// The half boundary days of a range are no sick days for the plan, in the
// materializer as in the cascade: the root maps start_half_day/end_half_day
// onto the report and CoversFullDay skips them.
func TestSickReport_MaterializationSkipsHalfBoundaryDays(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekThree)
	_, err := e.db.NewRaw(`UPDATE active.staff_absences SET start_half_day = TRUE, end_half_day = TRUE WHERE id = ? AND tenant_id = ?`,
		absenceID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)

	e.materialize(t, stampWeekOne, stampWeekThree)

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), "half start day")
	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), absenceID, "full day inside the range")
	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekThree), "half end day")
}

// The re-plan recreates a substitute row with its sick stamp, so the
// substitute's shortened report releases it.
func TestSickReport_ReplanKeepsSubstituteStamp(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekOne)
	planned := e.templateRow(t, templateID, e.subject.ID, stampWeekOne)
	_, err := e.db.NewRaw(`UPDATE schedule.instance_staff SET is_absent = TRUE WHERE id = ? AND tenant_id = ?`, planned.ID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)
	_, err = e.db.NewRaw(`INSERT INTO schedule.instance_staff (tenant_id, instance_id, staff_id, is_substitute) VALUES (?, ?, ?, TRUE)`,
		e.tenantID, planned.InstanceID, e.sub.ID).Exec(e.ctx)
	require.NoError(t, err)
	absenceID := e.createSickAbsenceFor(t, e.sub.ID, stampWeekOne, stampWeekTwo)
	assertSickStamp(t, e.templateRow(t, templateID, e.sub.ID, stampWeekOne), absenceID, "cascade marked the substitute")

	e.replan(t, templateID, stampWeekOne)

	substitute := e.templateRow(t, templateID, e.sub.ID, stampWeekOne)
	assert.True(t, substitute.IsSubstitute, "the substitute is recreated")
	assertSickStamp(t, substitute, absenceID, "recreated substitute")
	startText, endText := stampWeekTwo.String(), stampWeekTwo.String()
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.UpdateAbsence(ctx, e.sub.ID, nil, absenceID, workforce.UpdateAbsenceRequest{
			DateStart: &startText, DateEnd: &endText,
		})
		return err
	})
	assertPresent(t, e.templateRow(t, templateID, e.sub.ID, stampWeekOne), "recreated substitute after the report was shortened")
}

// A single block created for a sick person plans them absent.
func TestSickReport_CreatedBlockPlansSickPersonAbsent(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	spontaneous := true
	created := map[calendar.Date]int64{}
	for _, day := range []calendar.Date{stampWeekOne, stampWeekTwo} {
		e.inTx(t, func(ctx context.Context) error {
			instance, err := e.timetable.Instance.CreateInstance(ctx, timetable.CreateInstanceInput{
				Date:          day,
				StartTime:     time.Date(2000, 1, 1, 9, 0, 0, 0, time.UTC),
				EndTime:       time.Date(2000, 1, 1, 10, 0, 0, 0, time.UTC),
				Title:         "Einzeltermin",
				RoomID:        e.roomID,
				IsSpontaneous: &spontaneous,
				StaffIDs:      []int64{e.subject.ID},
			})
			if err == nil {
				created[day] = instance.ID
			}
			return err
		})
	}

	assertPresent(t, e.instanceRow(t, created[stampWeekOne], e.subject.ID), "healthy day")
	assertSickStamp(t, e.instanceRow(t, created[stampWeekTwo], e.subject.ID), absenceID, "sick day")
}

// instanceRow loads the staff member's assignment on one instance.
func (e *sickCascadeEnv) instanceRow(t *testing.T, instanceID, staffID int64) *timetable.InstanceStaff {
	t.Helper()
	var id int64
	require.NoError(t, e.db.NewRaw(`SELECT id FROM schedule.instance_staff WHERE tenant_id = ? AND instance_id = ? AND staff_id = ?`,
		e.tenantID, instanceID, staffID).Scan(e.ctx, &id))
	return e.reloadRow(t, id)
}

// A sick report makes the person absent the whole day: like an absent
// person, they are refused as a substitute, in the single save and in the
// Sammel-Vertretung, with the existing "absent" codes.
func TestSickReport_SickSubstituteIsRefused(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	// The Vertretungsplan refuses blocks before the real calendar day.
	day := calendar.NewDate(2026, time.December, 7)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, day, day)
	planned := e.templateRow(t, templateID, e.subject.ID, day)
	e.createSickAbsenceFor(t, e.sub.ID, day, day)
	scope := []int64{planned.InstanceID}

	err := testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
		_, err := e.timetable.TimetableData.Deviations.ApplyDeviations(ctx, planned.InstanceID, timetable.ApplyDeviationsInput{
			Absences:      []timetable.DeviationAbsenceInput{{StaffID: e.subject.ID, InstanceIDs: &scope}},
			Substitutions: []timetable.DeviationSubstitutionInput{{AbsentStaffID: e.subject.ID, SubstituteStaffID: e.sub.ID, InstanceIDs: &scope}},
		})
		return err
	})
	assertDeviationCode(t, err, timetable.CodeSubstituteAbsent)

	err = testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
		_, err := e.timetable.TimetableData.Deviations.ApplyBulkSubstitution(ctx, timetable.BulkSubstitutionInput{
			AbsentStaffID: e.subject.ID, SubstituteStaffID: &e.sub.ID, Dates: []calendar.Date{day},
		})
		return err
	})
	assertDeviationCode(t, err, timetable.CodeSubstituteAbsentOnDate)

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, day), "refused save changed nothing")
}

func assertDeviationCode(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *timetable.DeviationError
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, code, refusal.Code)
}

// Moving a sick person onto another block of their sick day keeps them
// absent there.
func TestSickReport_MovedStaffKeepsSickState(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	// MoveStaffBetweenBlocks refuses blocks before the real calendar day.
	day := calendar.NewDate(2026, time.December, 7)
	absenceID := e.createSickAbsenceFor(t, e.sub.ID, day, day)
	target := testpkg.CreateTestActivityInstance(t, e.db, day, e.roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &e.tmplID, Status: timetable.InstanceStatusPlanned, StartHHMM: "10:00",
	})

	e.inTx(t, func(ctx context.Context) error {
		_, err := e.timetable.Instance.MoveStaffBetweenBlocks(ctx, target.ID, timetable.MoveStaffInput{StaffID: e.sub.ID})
		return err
	})

	assertSickStamp(t, e.instanceRow(t, target.ID, e.sub.ID), absenceID, "sick person placed on the block")
}

// The materialization of a past day applies the sick report as well: the
// cascade leaves existing past blocks alone, but a block created now for a
// sick day is planned absent (I-1).
func TestSickReport_PastDayMaterializationStampsSickPerson(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	absenceID := e.createSickAbsence(t, pastWeek, pastWeek)

	e.materialize(t, pastWeek, pastWeek)

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, pastWeek), absenceID, "past block materialized after the report")
}

// A declined or cancelled sick report takes no plan effect.
func TestSickReport_DeclinedReportIsNotApplied(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	_, err := e.db.NewRaw(`UPDATE active.staff_absences SET status = 'declined' WHERE id = ? AND tenant_id = ?`, absenceID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)

	e.materialize(t, stampWeekTwo, stampWeekTwo)

	assertPresent(t, e.templateRow(t, templateID, e.subject.ID, stampWeekTwo), "declined report")
}

// A block moved onto a sick day keeps a planner's manual absence, as the
// cascade leaves an already absent row alone.
func TestSickReport_MovedBlockKeepsManualAbsence(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekThree, stampWeekThree)
	e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	row := e.templateRow(t, templateID, e.subject.ID, stampWeekThree)
	_, err := e.db.NewRaw(`UPDATE schedule.instance_staff SET is_absent = TRUE, absence_reason = 'Fortbildung' WHERE id = ? AND tenant_id = ?`, row.ID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)

	e.updateInstanceOn(t, row.InstanceID, templateID, []int64{e.subject.ID}, stampWeekTwo)

	moved := e.templateRow(t, templateID, e.subject.ID, stampWeekTwo)
	assert.True(t, moved.IsAbsent, "manual absence stays")
	assert.Nil(t, moved.SickAbsenceID, "manual absence takes no sick stamp")
	if assert.NotNil(t, moved.AbsenceReason) {
		assert.Equal(t, "Fortbildung", *moved.AbsenceReason)
	}
}

// The re-plan restores a manual absence over the materializer's sick mark:
// the cascade had left the already absent row alone.
func TestSickReport_ReplanKeepsManualAbsenceOverSickMark(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekOne)
	row := e.templateRow(t, templateID, e.subject.ID, stampWeekOne)
	_, err := e.db.NewRaw(`UPDATE schedule.instance_staff SET is_absent = TRUE, absence_reason = 'Fortbildung' WHERE id = ? AND tenant_id = ?`, row.ID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)
	e.createSickAbsence(t, stampWeekOne, stampWeekOne)

	e.replan(t, templateID, stampWeekOne)

	replanned := e.templateRow(t, templateID, e.subject.ID, stampWeekOne)
	assert.True(t, replanned.IsAbsent)
	assert.Nil(t, replanned.SickAbsenceID, "the manual absence stays manual")
}

// The re-plan keeps the active substitute of a sick person: the sick mark the
// materializer already set counts as the absence the substitute covers.
func TestSickReport_ReplanKeepsActiveSubstituteOfSickPerson(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	e.materialize(t, stampWeekOne, stampWeekOne)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
	planned := e.templateRow(t, templateID, e.subject.ID, stampWeekOne)
	_, err := e.db.NewRaw(`INSERT INTO schedule.instance_staff (tenant_id, instance_id, staff_id, is_substitute) VALUES (?, ?, ?, TRUE)`,
		e.tenantID, planned.InstanceID, e.sub.ID).Exec(e.ctx)
	require.NoError(t, err)

	e.replan(t, templateID, stampWeekOne)

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, stampWeekOne), absenceID, "sick lead")
	substitute := e.templateRow(t, templateID, e.sub.ID, stampWeekOne)
	assert.True(t, substitute.IsSubstitute, "the substitute is recreated")
	assert.False(t, substitute.IsAbsent, "the substitute covers")
}
