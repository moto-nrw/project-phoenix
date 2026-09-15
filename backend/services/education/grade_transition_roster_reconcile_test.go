package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// rosterRowCounter counts the roster rows a student holds on one instance.
func rosterRowCounter(t *testing.T, ctx context.Context, db *bun.DB, studentID int64) func(instanceID int64) int {
	t.Helper()
	return func(instanceID int64) int {
		n, err := db.NewSelect().
			TableExpr(`schedule.instance_students`).
			Where("instance_id = ?", instanceID).
			Where("student_id = ?", studentID).
			Count(ctx)
		require.NoError(t, err)
		return n
	}
}

// createRosterEnrollment links a student to an activity group from the given
// day on, which is what makes their roster rows enrollment-derived.
func createRosterEnrollment(t *testing.T, ctx context.Context, db *bun.DB, studentID, activityGroupID int64, validFrom timezone.Date) {
	t.Helper()
	enrollment := &activitiesModel.StudentEnrollment{
		StudentID:       studentID,
		ActivityGroupID: activityGroupID,
		ValidFrom:       activitiesModel.Date(validFrom),
	}
	enrollment.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(enrollment).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
	require.NoError(t, err)
}

// TestGradeTransitionWorkflow_Apply_ReconcilesFutureRosters covers the P1 fix:
// the materializer is insert-only, so a graduation applied after upcoming
// instances were materialized must delete the departed child's future roster
// rows, and a revert must re-add them. Past rows are historical and never
// touched.
func TestGradeTransitionWorkflow_Apply_ReconcilesFutureRosters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4roster-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Roster", "Child", gradClass)

	today := f.today()

	// A future planned template-backed instance the child is already on, plus a
	// past instance to prove the historical row is never disturbed.
	futureInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(7), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})
	pastInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(-7), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})

	testpkg.CreateTestInstanceStudent(t, db, futureInstance.ID, student.ID, scheduleModel.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, db, pastInstance.ID, student.ID, scheduleModel.AttendanceStatusExpected)

	// The enrollment that makes the child belong to the group — kept on
	// graduation so the revert can restore the future roster row.
	createRosterEnrollment(t, ctx, db, student.ID, activityGroup.ID, today.AddDays(-30))

	countRow := rosterRowCounter(t, ctx, db, student.ID)

	transitionID := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	// APPLY: the future roster row is removed, the past one survives.
	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	var status string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	require.Equal(t, string(users.StudentStatusAlumnus), status)

	assert.Equal(t, 0, countRow(futureInstance.ID), "graduated child must be dropped from the future roster")
	assert.Equal(t, 1, countRow(pastInstance.ID), "past roster rows are historical and must survive graduation")

	// REVERT: the future roster row is restored from the surviving enrollment.
	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	require.Equal(t, string(users.StudentStatusActive), status)

	assert.Equal(t, 1, countRow(futureInstance.ID), "revert must re-add the restored child to the future roster")
	assert.Equal(t, 1, countRow(pastInstance.ID), "past roster row stays a single historical entry after revert")

	// The restored row is a clean planned expectation.
	var restoredStatus string
	require.NoError(t, db.NewSelect().TableExpr(`schedule.instance_students`).Column("status").
		Where("instance_id = ?", futureInstance.ID).Where("student_id = ?", student.ID).Scan(ctx, &restoredStatus))
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, restoredStatus)
}

// TestGradeTransitionWorkflow_Revert_PreservesPerOccurrenceRosterEdits covers
// the P1 fix: apply/revert must be exact inverses of each other on the roster.
// Reconstructing future rosters from enrollments is not an inverse — it
// resurrects a child a supervisor deliberately removed from one occurrence, and
// it can never recreate a child hand-added to one occurrence with no enrollment
// behind them. The apply therefore archives every row it deletes and the revert
// replays exactly those.
func TestGradeTransitionWorkflow_Revert_PreservesPerOccurrenceRosterEdits(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4edits-%s", suffix)

	enrolledGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-enrolled-%s", suffix))
	guestGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-guest-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Edited", "Roster", gradClass)

	today := f.today()

	// Two occurrences of the group the child is enrolled in, and one occurrence
	// of a group they are NOT enrolled in.
	keptInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(7), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &enrolledGroup.ID})
	excusedInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(14), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &enrolledGroup.ID})
	guestInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(10), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &guestGroup.ID})

	// The roster as a supervisor left it: on the first occurrence, hand-removed
	// from the second (no row despite the enrollment), and hand-added as a guest
	// to an occurrence of a group they have no enrollment for.
	testpkg.CreateTestInstanceStudent(t, db, keptInstance.ID, student.ID,
		scheduleModel.AttendanceStatusExpected)
	testpkg.CreateTestInstanceStudent(t, db, guestInstance.ID, student.ID,
		scheduleModel.AttendanceStatusExpected)

	createRosterEnrollment(t, ctx, db, student.ID, enrolledGroup.ID, today.AddDays(-30))

	countRow := rosterRowCounter(t, ctx, db, student.ID)

	transitionID := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	assert.Equal(t, 0, countRow(keptInstance.ID))
	assert.Equal(t, 0, countRow(guestInstance.ID), "the hand-added guest row is dropped like any other planned row")
	assert.Equal(t, 0, countRow(excusedInstance.ID))

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, 1, countRow(keptInstance.ID), "the row the apply removed comes back")
	assert.Equal(t, 1, countRow(guestInstance.ID),
		"a hand-added row with no enrollment behind it must come back too")
	assert.Equal(t, 0, countRow(excusedInstance.ID),
		"an occurrence the child was deliberately taken off must NOT be resurrected from the enrollment")
}

// TestGradeTransitionWorkflow_Apply_RemovesTodaysPlannedRows covers the P2 fix:
// a transition applied during the school day must also clear the graduate's
// still-planned rows on the day's remaining blocks. Slot-list reads decide
// visibility from the enrollment interval, not from alumnus status, so a
// leftover row keeps the departed child in today's Plan/Abgleich lists and
// staffing counts. Rows that already recorded an event stay as history.
func TestGradeTransitionWorkflow_Apply_RemovesTodaysPlannedRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4today-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Today", "Child", gradClass)

	today := f.today()

	// Two blocks of the same group on the same day (distinct time windows — the
	// template-unique index forbids two identical ones), both still ahead of the
	// fixture's 12:00 apply clock.
	plannedToday := testpkg.CreateTestActivityInstance(t, db, today, room.ID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activityGroup.ID,
			StartHHMM:       "14:00",
			EndHHMM:         "15:00",
		})
	observedToday := testpkg.CreateTestActivityInstance(t, db, today, room.ID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activityGroup.ID,
			StartHHMM:       "15:15",
			EndHHMM:         "16:15",
		})

	testpkg.CreateTestInstanceStudent(t, db, plannedToday.ID, student.ID,
		scheduleModel.AttendanceStatusExpected)
	// An observed presence carries the check-in stamp every real check-in path
	// writes; a bare 'present' on a block that has not started is a plan (#405
	// review).
	checkedInAt := time.Date(2026, 8, 24, 11, 45, 0, 0, timezone.Berlin)
	testpkg.CreateTestInstanceStudent(t, db, observedToday.ID, student.ID,
		scheduleModel.AttendanceStatusPresent, testpkg.InstanceStudentOpts{CheckedInAt: &checkedInAt})

	countRow := rosterRowCounter(t, ctx, db, student.ID)

	transitionID := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	assert.Equal(t, 0, countRow(plannedToday.ID),
		"a still-planned block later today must not keep showing the departed child")
	assert.Equal(t, 1, countRow(observedToday.ID),
		"a row recording an actual presence today is history and must survive")

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, 1, countRow(plannedToday.ID), "the revert puts today's planned row back")
	assert.Equal(t, 1, countRow(observedToday.ID))
}

// TestGradeTransitionWorkflow_Revert_FillsTodaysInstanceMaterializedWhileAlumnus
// covers the boundary case of the alumnus-window fill (#405 review): an
// instance materialized AFTER the apply and dated TODAY carries no archive row
// — the materializer skipped the alumnus outright, so there was never a row to
// remove — which makes the revert's enrollment fill the only thing that can put
// the child back. Excluding the boundary date left them off today's roster
// permanently, with nothing left to repair it.
func TestGradeTransitionWorkflow_Revert_FillsTodaysInstanceMaterializedWhileAlumnus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4bound-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Boundary", "Child", gradClass)

	today := f.today()

	createRosterEnrollment(t, ctx, db, student.ID, activityGroup.ID, today.AddDays(-30))

	transitionID := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	// The materializer runs while the child is an alumnus and builds a block for
	// later today. It skips the alumnus, so the instance has no row for them —
	// and no archive entry either. calendar_period_id is what identifies the row
	// as materialized (and its roster as enrollment-derived).
	period := testpkg.CreateTestCalendarPeriod(t, db, fmt.Sprintf("Period-%s", suffix),
		today.AddDays(-60), today.AddDays(60))

	todayInstance := testpkg.CreateTestActivityInstance(t, db, today, room.ID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID:  &activityGroup.ID,
			CalendarPeriodID: &period.ID,
			StartHHMM:        "16:30",
			EndHHMM:          "17:30",
		})

	countRow := rosterRowCounter(t, ctx, db, student.ID)
	require.Equal(t, 0, countRow(todayInstance.ID))

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, 1, countRow(todayInstance.ID),
		"a block materialized today while the child was an alumnus must be filled on revert")

	var createdID int64
	require.NoError(t, db.NewSelect().
		TableExpr(`schedule.instance_students`).
		Column("id").
		Where("instance_id = ?", todayInstance.ID).
		Where("student_id = ?", student.ID).
		Scan(ctx, &createdID))
}

// TestGradeTransitionWorkflow_Apply_PreservesRecordedAttendance covers the P1
// fix: the archive pass must not delete rows that record something a human
// observed. A row on an instance that already ran to completion is attendance,
// not a plan — and the revert deliberately refuses to replay rows into completed
// or past instances while consuming their ledger entries, so anything deleted
// there is gone for good.
//
// Its counterpart is the second case: the same hand-set status on an occurrence
// that has NOT started yet is still a plan and IS archived (the repo-level
// TestInstanceStudentRepository_ArchivePlannedByStudentIDsFrom_ManualStatusRows
// pins both sides with explicit clocks). manual_status_at alone is therefore
// not the exemption — where the occurrence sits relative to now is.
func TestGradeTransitionWorkflow_Apply_PreservesRecordedAttendance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4final-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Finalized", "Child", gradClass)

	today := f.today()
	manualAt := time.Date(2026, 8, 24, 11, 30, 0, 0, timezone.Berlin)

	// Today's block that already finished, with the absence a supervisor
	// finalized by hand on it.
	completedToday := testpkg.CreateTestActivityInstance(t, db, today, room.ID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activityGroup.ID,
			Status:          scheduleModel.InstanceStatusCompleted,
			StartHHMM:       "08:00",
			EndHHMM:         "09:00",
		})
	// A block that has NOT started yet, carrying a hand-set status: nothing was
	// observed there, so the decision is still a plan and gets archived. Dated
	// tomorrow on purpose — the apply compares the occurrence's start time
	// against the workflow clock.
	plannedTomorrow := testpkg.CreateTestActivityInstance(t, db, today.AddDays(1), room.ID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activityGroup.ID,
			StartHHMM:       "09:30",
			EndHHMM:         "10:30",
		})

	testpkg.CreateTestInstanceStudent(t, db, completedToday.ID, student.ID,
		scheduleModel.AttendanceStatusAbsent, testpkg.InstanceStudentOpts{ManualStatusAt: &manualAt})
	testpkg.CreateTestInstanceStudent(t, db, plannedTomorrow.ID, student.ID,
		scheduleModel.AttendanceStatusAbsent, testpkg.InstanceStudentOpts{ManualStatusAt: &manualAt})

	countRow := rosterRowCounter(t, ctx, db, student.ID)

	transitionID := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	assert.Equal(t, 1, countRow(completedToday.ID),
		"attendance on a completed block is history the revert cannot restore — it must never be deleted")
	assert.Equal(t, 0, countRow(plannedTomorrow.ID),
		"a hand-set status on a block that has not started is still a plan and is archived")
}

// TestGradeTransitionWorkflow_Revert_FillsBackdatedInstance covers the P1 fix:
// the revert must decide "materialized during the alumnus window" from an
// ordering marker, not from created_at. A materialization transaction that
// started before the apply committed and then waited on the tenant lock stamps
// its rows with a pre-apply created_at although it inserted them afterwards;
// classifying those as pre-transition leaves the restored child off their
// rosters with no archive row to repair it.
func TestGradeTransitionWorkflow_Revert_FillsBackdatedInstance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4backd-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Backdated", "Child", gradClass)

	today := f.today()

	createRosterEnrollment(t, ctx, db, student.ID, activityGroup.ID, today.AddDays(-30))

	transitionID := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	// The blocked materializer commits AFTER the apply, but with the created_at
	// its transaction started with — an hour before the apply. It stamps the
	// calendar period, which is what marks the roster as enrollment-derived.
	period := testpkg.CreateTestCalendarPeriod(t, db, fmt.Sprintf("Period-%s", suffix),
		today.AddDays(-60), today.AddDays(60))

	lateInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(3), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID, CalendarPeriodID: &period.ID})

	_, err = db.NewUpdate().
		TableExpr(`schedule.activity_instances`).
		Set("created_at = ?", time.Date(2026, 8, 24, 11, 0, 0, 0, timezone.Berlin)).
		Where("id = ?", lateInstance.ID).
		Exec(ctx)
	require.NoError(t, err)

	countRow := rosterRowCounter(t, ctx, db, student.ID)
	require.Equal(t, 0, countRow(lateInstance.ID))

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, 1, countRow(lateInstance.ID),
		"an instance inserted after the apply must be filled even when its created_at predates the apply")

	var createdID int64
	require.NoError(t, db.NewSelect().
		TableExpr(`schedule.instance_students`).
		Column("id").
		Where("instance_id = ?", lateInstance.ID).
		Where("student_id = ?", student.ID).
		Scan(ctx, &createdID))
}

// TestGradeTransitionWorkflow_Revert_SkipsHandPlannedInstance pins the boundary
// of the enrollment fill (#405 review). A planner can create a single block by
// hand and link a template to it for its metadata; the roster of that block is
// the list of children the planner submitted, NOT a copy of the template's
// enrollments. Filling it on revert would put a restored child on a roster
// nobody assigned them to. Only rows the materializer produced — the ones
// carrying a calendar_period_id — are enrollment-derived.
func TestGradeTransitionWorkflow_Revert_SkipsHandPlannedInstance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4hand-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "HandPlanned", "Child", gradClass)

	today := f.today()

	createRosterEnrollment(t, ctx, db, student.ID, activityGroup.ID, today.AddDays(-30))

	transitionID := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)

	// Created after the apply (so it is inside the alumnus window) and linked to
	// the template — but by hand: no calendar period, and a roster of exactly
	// the children the planner picked (here: none).
	handPlanned := testpkg.CreateTestActivityInstance(t, db, today.AddDays(3), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})

	countRow := rosterRowCounter(t, ctx, db, student.ID)
	require.Equal(t, 0, countRow(handPlanned.ID))

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)

	assert.Equal(t, 0, countRow(handPlanned.ID),
		"a hand-planned block's roster is not enrollment-derived and must not be refilled")
}
