package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	activitiesRepo "github.com/moto-nrw/project-phoenix/database/repositories/activities"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestGradeTransitionService_Apply_ReconcilesFutureRosters covers the P1 fix:
// the materializer is insert-only, so a graduation applied after upcoming
// instances were materialized must delete the departed child's future roster
// rows, and a revert must re-add them. Past rows are historical and never
// touched.
func TestGradeTransitionService_Apply_ReconcilesFutureRosters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	reconciler := scheduleSvc.NewRosterReconciler(
		scheduleRepo.NewActivityInstanceRepository(db),
		scheduleRepo.NewInstanceStudentRepository(db),
		activitiesRepo.NewStudentEnrollmentRepository(db),
		nil,
	)
	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo:   educationRepo.NewGradeTransitionRepository(db),
		StudentRepo:      usersRepo.NewStudentRepository(db),
		PersonRepo:       usersRepo.NewPersonRepository(db),
		VisitRepo:        activeRepo.NewVisitRepository(db),
		AttendanceRepo:   activeRepo.NewAttendanceRepository(db),
		RosterReconciler: reconciler,
		DB:               db,
	})

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-roster-reconcile@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4roster-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Roster", "Child", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, activityGroup.ID, room.ID)

	today := timezone.TodayDate()

	// A future planned template-backed instance the child is already on, plus a
	// past instance to prove the historical row is never disturbed.
	futureInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(7), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})
	pastInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(-7), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", futureInstance.ID, pastInstance.ID)

	futureRow := testpkg.CreateTestInstanceStudent(t, db, futureInstance.ID, student.ID, scheduleModel.AttendanceStatusExpected)
	pastRow := testpkg.CreateTestInstanceStudent(t, db, pastInstance.ID, student.ID, scheduleModel.AttendanceStatusExpected)
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", futureRow.ID, pastRow.ID)

	// The enrollment that makes the child belong to the group — kept on
	// graduation so the revert can restore the future roster row.
	validFrom := today.AddDays(-30)
	enrollment := &activitiesModel.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: activityGroup.ID,
		ValidFrom:       validFrom,
	}
	enrollment.SetTenantID(1)
	_, err := db.NewInsert().Model(enrollment).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
	require.NoError(t, err)
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

	countRow := func(instanceID int64) int {
		n, cErr := db.NewSelect().
			TableExpr(`schedule.instance_students`).
			Where("instance_id = ?", instanceID).
			Where("student_id = ?", student.ID).
			Count(ctx)
		require.NoError(t, cErr)
		return n
	}

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	// APPLY: the future roster row is removed, the past one survives.
	_, err = service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	var status string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	require.Equal(t, string(users.StudentStatusAlumnus), status)

	assert.Equal(t, 0, countRow(futureInstance.ID), "graduated child must be dropped from the future roster")
	assert.Equal(t, 1, countRow(pastInstance.ID), "past roster rows are historical and must survive graduation")

	// REVERT: the future roster row is restored from the surviving enrollment.
	_, err = service.Revert(ctx, transition.ID, account.ID)
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

// TestGradeTransitionService_Revert_PreservesPerOccurrenceRosterEdits covers the
// P1 fix: apply/revert must be exact inverses of each other on the roster.
// Reconstructing future rosters from enrollments is not an inverse — it
// resurrects a child a supervisor deliberately removed from one occurrence, and
// it can never recreate a child hand-added to one occurrence with no enrollment
// behind them. The apply therefore archives every row it deletes and the revert
// replays exactly those.
func TestGradeTransitionService_Revert_PreservesPerOccurrenceRosterEdits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := newRosterReconcilingTransitionService(t, db)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-roster-edits@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4edits-%s", suffix)

	enrolledGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-enrolled-%s", suffix))
	guestGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-guest-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Edited", "Roster", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, enrolledGroup.ID, guestGroup.ID, room.ID)

	today := timezone.TodayDate()

	// Two occurrences of the group the child is enrolled in, and one occurrence
	// of a group they are NOT enrolled in.
	keptInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(7), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &enrolledGroup.ID})
	excusedInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(14), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &enrolledGroup.ID})
	guestInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(10), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &guestGroup.ID})
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances",
		keptInstance.ID, excusedInstance.ID, guestInstance.ID)

	// The roster as a supervisor left it: on the first occurrence, hand-removed
	// from the second (no row despite the enrollment), and hand-added as a guest
	// to an occurrence of a group they have no enrollment for.
	keptRow := testpkg.CreateTestInstanceStudent(t, db, keptInstance.ID, student.ID,
		scheduleModel.AttendanceStatusExpected)
	guestRow := testpkg.CreateTestInstanceStudent(t, db, guestInstance.ID, student.ID,
		scheduleModel.AttendanceStatusExpected)
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", keptRow.ID, guestRow.ID)

	enrollment := &activitiesModel.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: enrolledGroup.ID,
		ValidFrom:       today.AddDays(-30),
	}
	enrollment.SetTenantID(1)
	_, err := db.NewInsert().Model(enrollment).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
	require.NoError(t, err)
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

	countRow := func(instanceID int64) int {
		n, cErr := db.NewSelect().
			TableExpr(`schedule.instance_students`).
			Where("instance_id = ?", instanceID).
			Where("student_id = ?", student.ID).
			Count(ctx)
		require.NoError(t, cErr)
		return n
	}

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err = service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, 0, countRow(keptInstance.ID))
	assert.Equal(t, 0, countRow(guestInstance.ID), "the hand-added guest row is dropped like any other planned row")
	assert.Equal(t, 0, countRow(excusedInstance.ID))

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, 1, countRow(keptInstance.ID), "the row the apply removed comes back")
	assert.Equal(t, 1, countRow(guestInstance.ID),
		"a hand-added row with no enrollment behind it must come back too")
	assert.Equal(t, 0, countRow(excusedInstance.ID),
		"an occurrence the child was deliberately taken off must NOT be resurrected from the enrollment")
}

// TestGradeTransitionService_Apply_RemovesTodaysPlannedRows covers the P2 fix:
// a transition applied during the school day must also clear the graduate's
// still-planned rows on the day's remaining blocks. Slot-list reads decide
// visibility from the enrollment interval, not from alumnus status, so a
// leftover row keeps the departed child in today's Plan/Abgleich lists and
// staffing counts. Rows that already recorded an event stay as history.
func TestGradeTransitionService_Apply_RemovesTodaysPlannedRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := newRosterReconcilingTransitionService(t, db)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-roster-today@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4today-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Today", "Child", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, activityGroup.ID, room.ID)

	today := timezone.TodayDate()

	// Two blocks of the same group on the same day (distinct time windows — the
	// template-unique index forbids two identical ones).
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
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances",
		plannedToday.ID, observedToday.ID)

	plannedRow := testpkg.CreateTestInstanceStudent(t, db, plannedToday.ID, student.ID,
		scheduleModel.AttendanceStatusExpected)
	observedRow := testpkg.CreateTestInstanceStudent(t, db, observedToday.ID, student.ID,
		scheduleModel.AttendanceStatusPresent)
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", plannedRow.ID, observedRow.ID)

	countRow := func(instanceID int64) int {
		n, cErr := db.NewSelect().
			TableExpr(`schedule.instance_students`).
			Where("instance_id = ?", instanceID).
			Where("student_id = ?", student.ID).
			Count(ctx)
		require.NoError(t, cErr)
		return n
	}

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, 0, countRow(plannedToday.ID),
		"a still-planned block later today must not keep showing the departed child")
	assert.Equal(t, 1, countRow(observedToday.ID),
		"a row recording an actual presence today is history and must survive")

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, 1, countRow(plannedToday.ID), "the revert puts today's planned row back")
	assert.Equal(t, 1, countRow(observedToday.ID))
}

// TestGradeTransitionService_Revert_FillsTodaysInstanceMaterializedWhileAlumnus
// covers the boundary case of the alumnus-window fill (#405 review): an
// instance materialized AFTER the apply and dated TODAY carries no archive row
// — the materializer skipped the alumnus outright, so there was never a row to
// remove — which makes the revert's enrollment fill the only thing that can put
// the child back. Excluding the boundary date left them off today's roster
// permanently, with nothing left to repair it.
func TestGradeTransitionService_Revert_FillsTodaysInstanceMaterializedWhileAlumnus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := newRosterReconcilingTransitionService(t, db)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-roster-boundary@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4bound-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Boundary", "Child", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, activityGroup.ID, room.ID)

	today := timezone.TodayDate()

	enrollment := &activitiesModel.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: activityGroup.ID,
		ValidFrom:       today.AddDays(-30),
	}
	enrollment.SetTenantID(1)
	_, err := db.NewInsert().Model(enrollment).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
	require.NoError(t, err)
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err = service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	// The materializer runs while the child is an alumnus and builds a block for
	// later today. It skips the alumnus, so the instance has no row for them —
	// and no archive entry either.
	todayInstance := testpkg.CreateTestActivityInstance(t, db, today, room.ID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activityGroup.ID,
			StartHHMM:       "16:30",
			EndHHMM:         "17:30",
		})
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", todayInstance.ID)

	countRow := func(instanceID int64) int {
		n, cErr := db.NewSelect().
			TableExpr(`schedule.instance_students`).
			Where("instance_id = ?", instanceID).
			Where("student_id = ?", student.ID).
			Count(ctx)
		require.NoError(t, cErr)
		return n
	}
	require.Equal(t, 0, countRow(todayInstance.ID))

	_, err = service.Revert(ctx, transition.ID, account.ID)
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
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", createdID)
}

// newRosterReconcilingTransitionService wires the grade transition service with
// a real RosterReconciler (the production wiring in services.Factory).
func newRosterReconcilingTransitionService(t *testing.T, db *bun.DB) *educationService.GradeTransitionService {
	t.Helper()

	reconciler := scheduleSvc.NewRosterReconciler(
		scheduleRepo.NewActivityInstanceRepository(db),
		scheduleRepo.NewInstanceStudentRepository(db),
		activitiesRepo.NewStudentEnrollmentRepository(db),
		nil,
	)
	return educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo:   educationRepo.NewGradeTransitionRepository(db),
		StudentRepo:      usersRepo.NewStudentRepository(db),
		PersonRepo:       usersRepo.NewPersonRepository(db),
		VisitRepo:        activeRepo.NewVisitRepository(db),
		AttendanceRepo:   activeRepo.NewAttendanceRepository(db),
		RosterReconciler: reconciler,
		DB:               db,
	})
}

// TestGradeTransitionService_Apply_PreservesRecordedAttendance covers the P1
// fix: the archive pass must not delete rows that record something a human
// observed. A supervisor's hand-finalized absence (manual_status_at) and any row
// on an instance that already ran to completion are attendance, not a plan —
// and the revert deliberately refuses to replay rows into completed or past
// instances while consuming their ledger entries, so anything deleted there is
// gone for good.
func TestGradeTransitionService_Apply_PreservesRecordedAttendance(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := newRosterReconcilingTransitionService(t, db)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-roster-finalized@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4final-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Finalized", "Child", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, activityGroup.ID, room.ID)

	today := timezone.TodayDate()
	manualAt := time.Now().Add(-30 * time.Minute)

	// Today's block that already finished, with the absence a supervisor
	// finalized by hand on it.
	completedToday := testpkg.CreateTestActivityInstance(t, db, today, room.ID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activityGroup.ID,
			Status:          scheduleModel.InstanceStatusCompleted,
			StartHHMM:       "08:00",
			EndHHMM:         "09:00",
		})
	// A still-planned block later today carrying a hand-set status too: the
	// decision is a record even though the occurrence has not run yet.
	plannedToday := testpkg.CreateTestActivityInstance(t, db, today, room.ID,
		testpkg.ActivityInstanceOpts{
			ActivityGroupID: &activityGroup.ID,
			StartHHMM:       "09:30",
			EndHHMM:         "10:30",
		})
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances",
		completedToday.ID, plannedToday.ID)

	completedRow := testpkg.CreateTestInstanceStudent(t, db, completedToday.ID, student.ID,
		scheduleModel.AttendanceStatusAbsent, testpkg.InstanceStudentOpts{ManualStatusAt: &manualAt})
	manualPlannedRow := testpkg.CreateTestInstanceStudent(t, db, plannedToday.ID, student.ID,
		scheduleModel.AttendanceStatusAbsent, testpkg.InstanceStudentOpts{ManualStatusAt: &manualAt})
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students",
		completedRow.ID, manualPlannedRow.ID)

	countRow := func(instanceID int64) int {
		n, cErr := db.NewSelect().
			TableExpr(`schedule.instance_students`).
			Where("instance_id = ?", instanceID).
			Where("student_id = ?", student.ID).
			Count(ctx)
		require.NoError(t, cErr)
		return n
	}

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, 1, countRow(completedToday.ID),
		"attendance on a completed block is history the revert cannot restore — it must never be deleted")
	assert.Equal(t, 1, countRow(plannedToday.ID),
		"a status a supervisor finalized by hand is a record, not a plan")
}

// TestGradeTransitionService_Revert_FillsBackdatedInstance covers the P1 fix:
// the revert must decide "materialized during the alumnus window" from an
// ordering marker, not from created_at. A materialization transaction that
// started before the apply committed and then waited on the tenant lock stamps
// its rows with a pre-apply created_at although it inserted them afterwards;
// classifying those as pre-transition leaves the restored child off their
// rosters with no archive row to repair it.
func TestGradeTransitionService_Revert_FillsBackdatedInstance(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := newRosterReconcilingTransitionService(t, db)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-roster-backdated@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4backd-%s", suffix)

	activityGroup := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("AG-%s", suffix))
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Room-%s", suffix))
	student := testpkg.CreateTestStudent(t, db, "Backdated", "Child", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID, activityGroup.ID, room.ID)

	today := timezone.TodayDate()

	enrollment := &activitiesModel.StudentEnrollment{
		StudentID:       student.ID,
		ActivityGroupID: activityGroup.ID,
		ValidFrom:       today.AddDays(-30),
	}
	enrollment.SetTenantID(1)
	_, err := db.NewInsert().Model(enrollment).ModelTableExpr(`activities.student_enrollments`).Exec(ctx)
	require.NoError(t, err)
	defer testpkg.CleanupTableRecords(t, db, "activities.student_enrollments", enrollment.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err = service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	// The blocked materializer commits AFTER the apply, but with the created_at
	// its transaction started with — an hour before the apply.
	lateInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(3), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})
	defer testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", lateInstance.ID)

	_, err = db.NewUpdate().
		TableExpr(`schedule.activity_instances`).
		Set("created_at = ?", time.Now().Add(-time.Hour)).
		Where("id = ?", lateInstance.ID).
		Exec(ctx)
	require.NoError(t, err)

	countRow := func(instanceID int64) int {
		n, cErr := db.NewSelect().
			TableExpr(`schedule.instance_students`).
			Where("instance_id = ?", instanceID).
			Where("student_id = ?", student.ID).
			Count(ctx)
		require.NoError(t, cErr)
		return n
	}
	require.Equal(t, 0, countRow(lateInstance.ID))

	_, err = service.Revert(ctx, transition.ID, account.ID)
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
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_students", createdID)
}
