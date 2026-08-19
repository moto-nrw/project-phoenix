package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteTimetableInstancesForEndedSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Daily Sync Room %d", time.Now().UnixNano()))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Daily Sync Activity %d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "DailySync", "Student", "9z")

	instance := testpkg.CreateTestActivityInstance(t, db, timezone.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
		Status:        scheduleModels.InstanceStatusActive,
		ActiveGroupID: &activeGroup.ID,
		Title:         fmt.Sprintf("Daily Sync Instance %d", time.Now().UnixNano()),
		IsSpontaneous: true,
	})
	instanceStudent := testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, scheduleModels.AttendanceStatusExpected)

	// A second student who is checked in and still open — the bridge must
	// stamp their slot checkout when the daily session end closes the visits.
	presentStudent := testpkg.CreateTestStudent(t, db, "DailySync", "Present", "9z")
	presentRow := testpkg.CreateTestInstanceStudent(t, db, instance.ID, presentStudent.ID, scheduleModels.AttendanceStatusPresent)
	checkedInAt := time.Now().Add(-2 * time.Hour)
	_, err := db.NewUpdate().
		Table("schedule.instance_students").
		Set("checked_in_at = ?", checkedInAt).
		Where("id = ?", presentRow.ID).
		Exec(context.Background())
	require.NoError(t, err)

	instanceRepo := scheduleRepo.NewActivityInstanceRepository(db)
	instanceStudentRepo := scheduleRepo.NewInstanceStudentRepository(db)
	s := &Scheduler{
		instanceRepo:        instanceRepo,
		instanceStudentRepo: instanceStudentRepo,
		// Completion runs through the shared bridge now, so the nightly job and
		// the force-start path leave identical rows behind (#1747). Care days
		// stay unwired here: with no care plans in play every expected row is
		// stamped absent, exactly as this test asserts below.
		timetableBridge: scheduleSvc.NewTimetableBridgeService(scheduleSvc.TimetableBridgeDependencies{
			Instances:        instanceRepo,
			InstanceStudents: instanceStudentRepo,
		}),
		logger: slog.Default(),
	}
	completed, err := s.completeTimetableInstancesForEndedSessions(context.Background(), &activeSvc.DailySessionCleanupResult{
		EndedActiveGroupIDs: []int64{activeGroup.ID},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, completed)

	var reloaded scheduleModels.ActivityInstance
	err = db.NewSelect().
		Model(&reloaded).
		ModelTableExpr(`schedule.activity_instances AS "activity_instance"`).
		Where(`"activity_instance".id = ?`, instance.ID).
		Scan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.InstanceStatusCompleted, reloaded.Status)
	assert.NotNil(t, reloaded.CompletedAt)

	var reloadedStudent scheduleModels.InstanceStudent
	err = db.NewSelect().
		Model(&reloadedStudent).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".id = ?`, instanceStudent.ID).
		Scan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusAbsent, reloadedStudent.Status)

	var reloadedPresent scheduleModels.InstanceStudent
	err = db.NewSelect().
		Model(&reloadedPresent).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".id = ?`, presentRow.ID).
		Scan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.AttendanceStatusPresent, reloadedPresent.Status, "observed presence must be preserved")
	require.NotNil(t, reloadedPresent.CheckedOutAt, "daily session end must close the open slot checkout")
	assert.False(t, reloadedPresent.CheckedOutAt.Before(checkedInAt))
}
