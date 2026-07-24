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
)

// TestGradeTransitionService_Apply_ReconcilesFutureRosters covers the P1 fix:
// the materializer is insert-only, so a graduation applied after upcoming
// instances were materialized must delete the departed child's future roster
// rows, and a revert must re-add them. Past rows are historical and never
// touched.
func TestGradeTransitionService_Apply_ReconcilesFutureRosters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

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
