package active_test

import (
	"testing"
	"time"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveService_ListStudentsInTransit(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)
	now := time.Now()

	activity := testpkg.CreateTestActivityGroup(t, db, "transit-list")
	room := testpkg.CreateTestRoom(t, db, "Transit List Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "Transit", "List")
	device := testpkg.CreateTestDevice(t, db, "transit-list-device")
	transitStudent := testpkg.CreateTestStudent(t, db, "Transit", "Listed", "TLS1")
	inRoomStudent := testpkg.CreateTestStudent(t, db, "In", "Room", "TLS2")
	checkedOutStudent := testpkg.CreateTestStudent(t, db, "Checked", "Out", "TLS3")

	transitAttendance := testpkg.CreateTestAttendance(t, db, transitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	inRoomAttendance := testpkg.CreateTestAttendance(t, db, inRoomStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	checkOutTime := now.Add(-5 * time.Minute)
	checkedOutAttendance := testpkg.CreateTestAttendance(t, db, checkedOutStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), &checkOutTime)
	visit := testpkg.CreateTestVisit(t, db, inRoomStudent.ID, activeGroup.ID, now.Add(-15*time.Minute), nil)

	defer testpkg.CleanupActivityFixtures(
		t, db,
		activity.ID, room.ID, activeGroup.ID, staff.ID, device.ID,
		transitStudent.ID, inRoomStudent.ID, checkedOutStudent.ID, visit.ID,
	)
	defer testpkg.CleanupTableRecords(t, db, "active.attendance", transitAttendance.ID, inRoomAttendance.ID, checkedOutAttendance.ID)

	ids, err := service.ListStudentsInTransit(ctx)

	require.NoError(t, err)
	assert.Contains(t, ids, transitStudent.ID)
	assert.NotContains(t, ids, inRoomStudent.ID)
	assert.NotContains(t, ids, checkedOutStudent.ID)
}

func TestActiveService_ListStudentsInTransit_NoOpenAttendance(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	ids, err := service.ListStudentsInTransit(ctx)

	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestActiveService_AssignTransitStudentsToActiveGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)
	now := time.Now()

	sourceActivity := testpkg.CreateTestActivityGroup(t, db, "transit-assign-source")
	targetActivity := testpkg.CreateTestActivityGroup(t, db, "transit-assign-target")
	sourceRoom := testpkg.CreateTestRoom(t, db, "Transit Source Room")
	targetRoom := testpkg.CreateTestRoom(t, db, "Transit Target Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "Transit", "Assign")
	device := testpkg.CreateTestDevice(t, db, "transit-assign-device")
	transitStudent := testpkg.CreateTestStudent(t, db, "Transit", "Assignable", "TAS1")
	inRoomStudent := testpkg.CreateTestStudent(t, db, "Already", "Roomed", "TAS2")

	transitAttendance := testpkg.CreateTestAttendance(t, db, transitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	inRoomAttendance := testpkg.CreateTestAttendance(t, db, inRoomStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	existingVisit := testpkg.CreateTestVisit(t, db, inRoomStudent.ID, sourceGroup.ID, now.Add(-15*time.Minute), nil)

	defer testpkg.CleanupActivityFixtures(
		t, db,
		sourceActivity.ID, targetActivity.ID, sourceRoom.ID, targetRoom.ID,
		sourceGroup.ID, targetGroup.ID, staff.ID, device.ID,
		transitStudent.ID, inRoomStudent.ID, existingVisit.ID,
	)
	defer testpkg.CleanupTableRecords(t, db, "active.attendance", transitAttendance.ID, inRoomAttendance.ID)

	result, err := service.AssignTransitStudentsToActiveGroup(ctx, []int64{transitStudent.ID, inRoomStudent.ID, transitStudent.ID}, targetGroup.ID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, targetGroup.ID, result.ActiveGroupID)
	assert.Equal(t, targetRoom.ID, result.RoomID)
	assert.Equal(t, []int64{transitStudent.ID}, result.Assigned)
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, activeSvc.TransitSkipNotInTransit, result.Skipped[0].Reason)
	assert.Equal(t, inRoomStudent.ID, result.Skipped[0].StudentID)

	currentVisit, err := service.GetStudentCurrentVisit(ctx, transitStudent.ID)
	require.NoError(t, err)
	require.NotNil(t, currentVisit)
	assert.Equal(t, targetGroup.ID, currentVisit.ActiveGroupID)
}

func TestActiveService_AssignTransitStudentsToActiveGroup_InvalidInput(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	result, err := service.AssignTransitStudentsToActiveGroup(ctx, nil, 42)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrInvalidData)

	activity := testpkg.CreateTestActivityGroup(t, db, "transit-invalid-input")
	room := testpkg.CreateTestRoom(t, db, "Transit Invalid Input Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, targetGroup.ID)

	result, err = service.AssignTransitStudentsToActiveGroup(ctx, []int64{-42}, targetGroup.ID)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrInvalidData)
}

func TestActiveService_AssignTransitStudentsToActiveGroup_EndedTargetFails(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(1)

	activity := testpkg.CreateTestActivityGroup(t, db, "transit-ended-target")
	room := testpkg.CreateTestRoom(t, db, "Transit Ended Target Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Transit", "Blocked", "TES1")
	endTime := time.Now()
	targetGroup.EndTime = &endTime
	require.NoError(t, service.UpdateActiveGroup(ctx, targetGroup))

	defer testpkg.CleanupActivityFixtures(t, db, activity.ID, room.ID, targetGroup.ID, student.ID)

	result, err := service.AssignTransitStudentsToActiveGroup(ctx, []int64{student.ID}, targetGroup.ID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrActiveGroupAlreadyEnded)
}
