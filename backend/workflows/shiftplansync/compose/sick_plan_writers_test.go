// Hermetic integration tests for the PR #6300 design: every writer of block
// assignments goes through one stamp helper (I-1), and the sick-report
// writers lock every day of the report, past days included, before they read
// the stamps (I-2). Each test is red without the guard it names (I-3).
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

// Mondays before the suite's fixed cascade clock (2026-08-24).
var (
	pastWeekOne = calendar.NewDate(2026, time.August, 10)
	pastWeekTwo = calendar.NewDate(2026, time.August, 17)
)

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

// Repro of the review finding: a materialization of a past day stamps the
// rows of report S and holds its day locks while S is deleted. The deletion
// waits for the day and releases the materialized stamp instead of leaving
// it on the deleted report.
func TestSickReport_PastMaterializationAgainstDeletion(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	absenceID := e.createSickAbsence(t, pastWeekTwo, pastWeekTwo)
	release, materialization := e.holdMaterialization(t, pastWeekTwo)

	deletionDone := make(chan error, 1)
	go func() {
		deletionDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			return e.absences.SickReports.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
		})
	}()
	e.releaseAfterDayLockWait(t, pastWeekTwo, release, materialization, deletionDone, "deletion")

	row := e.templateRow(t, templateID, e.subject.ID, pastWeekTwo)
	assert.Nil(t, row.SickAbsenceID, "no stamp names the deleted report")
}

// Repro of the review finding for the merge: a materialization of a past day
// stamps report S while a bridging report merges S into P. The reassignment
// waits for the day and moves the materialized stamp to P.
func TestSickReport_PastMaterializationAgainstMerge(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	survivorID := e.createSickAbsence(t, pastWeekOne, pastWeekOne)
	mergedID := e.createSickAbsence(t, pastWeekTwo, pastWeekTwo)
	release, materialization := e.holdMaterialization(t, pastWeekTwo)

	mergeDone := make(chan error, 1)
	go func() {
		mergeDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			_, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
				AbsenceType: "sick", DateStart: pastWeekOne.String(), DateEnd: pastWeekTwo.String(),
			})
			return err
		})
	}()
	e.releaseAfterDayLockWait(t, pastWeekTwo, release, materialization, mergeDone, "merge")

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, pastWeekTwo), survivorID, "materialized row of the merged report")
	_, err := e.shifts.FindStaffAbsence(e.ctx, mergedID)
	require.Error(t, err, "the merged report is gone")
}

// holdMaterialization materializes day inside an open transaction and keeps
// it open, with its day locks and rows uncommitted, until release closes.
func (e *sickCascadeEnv) holdMaterialization(t *testing.T, day calendar.Date) (chan struct{}, chan error) {
	t.Helper()
	materialized := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			if _, err := e.timetable.Materialization.MaterializeForTenant(ctx, day, day, timetable.MaterializationSourceManual); err != nil {
				return err
			}
			close(materialized)
			<-release
			return nil
		})
	}()
	select {
	case <-materialized:
	case err := <-done:
		require.FailNow(t, "materialization failed before it held its days", "%v", err)
	}
	return release, done
}

// releaseAfterDayLockWait requires the sick-report write to wait for the day
// lock the held materialization owns, then commits the materialization and
// requires both to succeed.
func (e *sickCascadeEnv) releaseAfterDayLockWait(t *testing.T, day calendar.Date, release chan struct{}, materialization, write chan error, name string) {
	t.Helper()
	if returned, err := e.awaitDayLockWaiter(t, day, write); returned {
		close(release)
		require.NoError(t, <-materialization)
		require.FailNow(t, name+" did not wait for the materialization's day lock", "returned early: %v", err)
	}
	close(release)
	require.NoError(t, <-materialization)
	require.NoError(t, <-write)
}

// I-2 for a new report on a past day: the report's write holds the day while
// a materialization of that day runs. The run waits and stamps its rows.
func TestSickReport_PastReportAgainstMaterialization(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	templateID := e.createSupervisedTemplate(t)
	created := make(chan int64, 1)
	release := make(chan struct{})
	reportDone := make(chan error, 1)
	go func() {
		reportDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			absence, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
				AbsenceType: "sick", DateStart: pastWeekTwo.String(), DateEnd: pastWeekTwo.String(),
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

	materialized := make(chan error, 1)
	go func() {
		_, err := e.timetable.Materialization.MaterializeForTenant(e.ctx, pastWeekTwo, pastWeekTwo, timetable.MaterializationSourceManual)
		materialized <- err
	}()
	if returned, err := e.awaitDayLockWaiter(t, pastWeekTwo, materialized); returned {
		close(release)
		require.NoError(t, <-reportDone)
		require.FailNow(t, "materialization did not wait for the past sick report", "returned early: %v", err)
	}
	close(release)
	require.NoError(t, <-reportDone)
	require.NoError(t, <-materialized)

	assertSickStamp(t, e.templateRow(t, templateID, e.subject.ID, pastWeekTwo), absenceID, "past block materialized beside the report")
}

// The reproduced deadlock: a writer holding a day lock inserts an assignment
// of the subject (the foreign key takes a key share on the staff row), the
// way a materialization does, while a sick report for that day is filed.
// The report takes its day locks before Workforce locks the staff row, so it
// waits for the writer instead of deadlocking against it; both succeed and
// the report marks the new row.
func TestSickReport_DayLockWriterAndReportDoNotDeadlock(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	block, _ := e.createBlock(t, stampWeekOne, timetable.InstanceStatusPlanned, "10:00", e.admin.ID, testpkg.InstanceStaffOpts{})
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
			_, err := tx.NewRaw(`INSERT INTO schedule.instance_staff (tenant_id, instance_id, staff_id) VALUES (?, ?, ?)`,
				e.tenantID, block.ID, e.subject.ID).Exec(ctx)
			return err
		})
	}()
	<-holding
	reportDone := make(chan error, 1)
	go func() {
		reportDone <- testpkg.WithTenantTx(t, context.Background(), e.db, e.tenantID, func(ctx context.Context, _ bun.Tx) error {
			_, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
				AbsenceType: "sick", DateStart: stampWeekOne.String(), DateEnd: stampWeekOne.String(),
			})
			return err
		})
	}()
	if returned, err := e.awaitDayLockWaiter(t, stampWeekOne, reportDone); returned {
		close(proceed)
		require.NoError(t, <-writerDone)
		require.FailNow(t, "sick report did not wait for the day", "returned early: %v", err)
	}
	close(proceed)
	require.NoError(t, <-writerDone, "the day-lock writer must not be a deadlock victim")
	require.NoError(t, <-reportDone, "the sick report must not be a deadlock victim")

	row := e.instanceRow(t, block.ID, e.subject.ID)
	assert.True(t, row.IsAbsent, "the report marks the row the writer added")
	assert.NotNil(t, row.SickAbsenceID)
}
