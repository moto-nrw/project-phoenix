package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteTimetableInstancesForEndedSessions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Daily Sync Room %d", time.Now().UnixNano()))
	activity := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Daily Sync Activity %d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "DailySync", "Student", "9z")

	instance := testpkg.CreateTestActivityInstance(t, db, time.Now(), room.ID, testpkg.ActivityInstanceOpts{
		Status:        scheduleModels.InstanceStatusActive,
		ActiveGroupID: &activeGroup.ID,
		Title:         fmt.Sprintf("Daily Sync Instance %d", time.Now().UnixNano()),
		IsSpontaneous: true,
	})
	instanceStudent := testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, scheduleModels.AttendanceStatusExpected)
	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", instanceStudent.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", instance.ID)
		testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, activity.ID, room.ID, student.ID)
	})

	s := &Scheduler{
		instanceRepo:        scheduleRepo.NewActivityInstanceRepository(db),
		instanceStudentRepo: scheduleRepo.NewInstanceStudentRepository(db),
		logger:              slog.Default(),
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
}
