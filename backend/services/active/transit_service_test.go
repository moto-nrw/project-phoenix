package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// activeSvcBypassAuth mirrors the old unauthenticated move path: the
// *Authorized variants with BypassResourceChecks behave identically to the
// deleted thin wrappers.
var activeSvcBypassAuth = activeSvc.StudentMoveAuthorization{BypassResourceChecks: true}

func TestActiveService_ListStudentsInTransit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	activity := testpkg.CreateTestActivityGroup(t, db, "transit-list")
	room := testpkg.CreateTestRoom(t, db, "Transit List Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "Transit", "List")
	device := testpkg.CreateTestDevice(t, db, "transit-list-device")
	transitStudent := testpkg.CreateTestStudent(t, db, "Transit", "Listed", "TLS1")
	inRoomStudent := testpkg.CreateTestStudent(t, db, "In", "Room", "TLS2")
	checkedOutStudent := testpkg.CreateTestStudent(t, db, "Checked", "Out", "TLS3")

	testpkg.CreateTestAttendance(t, db, transitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	testpkg.CreateTestAttendance(t, db, inRoomStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	checkOutTime := now.Add(-5 * time.Minute)
	testpkg.CreateTestAttendance(t, db, checkedOutStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), &checkOutTime)
	testpkg.CreateTestVisit(t, db, inRoomStudent.ID, activeGroup.ID, now.Add(-15*time.Minute), nil)

	ids, err := service.ListStudentsInTransit(ctx)

	require.NoError(t, err)
	assert.Contains(t, ids, transitStudent.ID)
	assert.NotContains(t, ids, inRoomStudent.ID)
	assert.NotContains(t, ids, checkedOutStudent.ID)
}

func TestActiveService_ListStudentsInTransit_NoOpenAttendance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(987654)

	ids, err := service.ListStudentsInTransit(ctx)

	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestActiveService_ListStudentsPresentToday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	staff := testpkg.CreateTestStaff(t, db, "Present", "List")
	device := testpkg.CreateTestDevice(t, db, "present-list-device")
	presentStudent := testpkg.CreateTestStudent(t, db, "Present", "Listed", "PLS1")
	checkedOutStudent := testpkg.CreateTestStudent(t, db, "Checked", "Out", "PLS2")
	absentStudent := testpkg.CreateTestStudent(t, db, "Absent", "PresentList", "PLS3")

	testpkg.CreateTestAttendance(t, db, presentStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	checkOutTime := now.Add(-5 * time.Minute)
	testpkg.CreateTestAttendance(t, db, checkedOutStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), &checkOutTime)

	ids, err := service.ListStudentsPresentToday(ctx)

	require.NoError(t, err)
	assert.Contains(t, ids, presentStudent.ID)
	assert.NotContains(t, ids, checkedOutStudent.ID)
	assert.NotContains(t, ids, absentStudent.ID)
}

func TestActiveService_ListStudentsPresentToday_NoOpenAttendance(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.TenantContext(987654)

	ids, err := service.ListStudentsPresentToday(ctx)

	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestActiveService_MoveStudentsToActiveGroup_PreservesVisitHistory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	sourceActivity := testpkg.CreateTestActivityGroup(t, db, "move-source")
	targetActivity := testpkg.CreateTestActivityGroup(t, db, "move-target")
	sourceRoom := testpkg.CreateTestRoom(t, db, "Move Source Room")
	targetRoom := testpkg.CreateTestRoom(t, db, "Move Target Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "Move", "Staff")
	device := testpkg.CreateTestDevice(t, db, "move-device")

	inSourceStudent := testpkg.CreateTestStudent(t, db, "Move", "FromSource", "MFS1")
	inTargetStudent := testpkg.CreateTestStudent(t, db, "Move", "AlreadyTarget", "MAT1")
	transitStudent := testpkg.CreateTestStudent(t, db, "Move", "FromTransit", "MFT1")
	absentStudent := testpkg.CreateTestStudent(t, db, "Move", "Absent", "MA1")

	testpkg.CreateTestAttendance(t, db, inSourceStudent.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestAttendance(t, db, inTargetStudent.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestAttendance(t, db, transitStudent.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	sourceVisit := testpkg.CreateTestVisit(t, db, inSourceStudent.ID, sourceGroup.ID, now.Add(-25*time.Minute), nil)
	targetVisit := testpkg.CreateTestVisit(t, db, inTargetStudent.ID, targetGroup.ID, now.Add(-20*time.Minute), nil)

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{inSourceStudent.ID, inTargetStudent.ID, transitStudent.ID, absentStudent.ID, inSourceStudent.ID}, targetGroup.ID, activeSvcBypassAuth)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ActiveGroupID)
	require.NotNil(t, result.RoomID)
	assert.Equal(t, targetGroup.ID, *result.ActiveGroupID)
	assert.Equal(t, targetRoom.ID, *result.RoomID)
	assert.ElementsMatch(t, []int64{inSourceStudent.ID, transitStudent.ID}, result.Moved)
	assert.Equal(t, []int64{inTargetStudent.ID}, result.Unchanged)
	assert.Equal(t, map[int64]int64{inSourceStudent.ID: sourceGroup.ID}, result.PreviousActiveGroupIDs)
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, absentStudent.ID, result.Skipped[0].StudentID)
	assert.Equal(t, activeSvc.StudentMoveSkipNotPresent, result.Skipped[0].Reason)

	endedSourceVisit, err := service.GetVisit(ctx, sourceVisit.ID)
	require.NoError(t, err)
	require.NotNil(t, endedSourceVisit.ExitTime, "source visit must be closed instead of mutated")

	currentMovedVisit, err := service.GetStudentCurrentVisit(ctx, inSourceStudent.ID)
	require.NoError(t, err)
	require.NotEqual(t, sourceVisit.ID, currentMovedVisit.ID)
	assert.Equal(t, targetGroup.ID, currentMovedVisit.ActiveGroupID)
	assert.Nil(t, currentMovedVisit.ExitTime)

	currentTransitVisit, err := service.GetStudentCurrentVisit(ctx, transitStudent.ID)
	require.NoError(t, err)
	assert.Equal(t, targetGroup.ID, currentTransitVisit.ActiveGroupID)
	assert.Nil(t, currentTransitVisit.ExitTime)

	currentTargetVisit, err := service.GetStudentCurrentVisit(ctx, inTargetStudent.ID)
	require.NoError(t, err)
	assert.Equal(t, targetVisit.ID, currentTargetVisit.ID, "idempotent target moves must not duplicate visits")

	_, err = service.GetStudentCurrentVisit(ctx, absentStudent.ID)
	assert.ErrorIs(t, err, activeSvc.ErrVisitNotFound)
}

func TestActiveService_MoveStudentsToActiveGroup_RejectsGraduatedStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	sourceActivity := testpkg.CreateTestActivityGroup(t, db, "move-graduate-source")
	targetActivity := testpkg.CreateTestActivityGroup(t, db, "move-graduate-target")
	sourceRoom := testpkg.CreateTestRoom(t, db, "Move Graduate Source Room")
	targetRoom := testpkg.CreateTestRoom(t, db, "Move Graduate Target Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "MoveGraduate", "Staff")
	device := testpkg.CreateTestDevice(t, db, "move-graduate-device")
	student := testpkg.CreateTestStudent(t, db, "MoveGraduate", "Student", "MGS1")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	visit := testpkg.CreateTestVisit(t, db, student.ID, sourceGroup.ID, now.Add(-20*time.Minute), nil)

	_, err := db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", usersModel.StudentStatusAlumnus).
		Where("id = ?", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, targetGroup.ID, activeSvcBypassAuth)

	require.Error(t, err)
	assert.ErrorIs(t, err, activeSvc.ErrStudentGraduated)
	assert.Nil(t, result)
	reloadedVisit, reloadErr := service.GetVisit(ctx, visit.ID)
	require.NoError(t, reloadErr)
	assert.Nil(t, reloadedVisit.ExitTime)
	assert.Equal(t, sourceGroup.ID, reloadedVisit.ActiveGroupID)
}

func TestActiveService_MoveStudentsToActiveGroup_RejectsWhenNoStudentsPresent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "move-all-absent-target")
	room := testpkg.CreateTestRoom(t, db, "Move All Absent Target Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	absentStudent := testpkg.CreateTestStudent(t, db, "Move", "AllAbsent", "MAA1")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{absentStudent.ID}, targetGroup.ID, activeSvcBypassAuth)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrStudentsNotPresent)
}

func TestActiveService_MoveStudentsToActiveGroup_InvalidInput(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, nil, 0, activeSvcBypassAuth)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrInvalidData)

	activity := testpkg.CreateTestActivityGroup(t, db, "move-invalid-input")
	room := testpkg.CreateTestRoom(t, db, "Move Invalid Input Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	result, err = service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{-42}, targetGroup.ID, activeSvcBypassAuth)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrInvalidData)
}

func TestActiveService_MoveStudentsToActiveGroup_EndedTargetFails(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "move-ended-target")
	room := testpkg.CreateTestRoom(t, db, "Move Ended Target Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Move", "Blocked", "MET1")
	endTime := time.Now()
	targetGroup.EndTime = &endTime
	require.NoError(t, service.UpdateActiveGroup(ctx, targetGroup))

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, targetGroup.ID, activeSvcBypassAuth)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrActiveGroupAlreadyEnded)
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_AllowsUnsupervisedSource(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	sourceActivity := testpkg.CreateTestActivityGroup(t, db, "move-auth-source")
	targetActivity := testpkg.CreateTestActivityGroup(t, db, "move-auth-target")
	sourceRoom := testpkg.CreateTestRoom(t, db, "Move Auth Source Room")
	targetRoom := testpkg.CreateTestRoom(t, db, "Move Auth Target Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "MoveAuth", "TargetOnly")
	device := testpkg.CreateTestDevice(t, db, "move-auth-reject-device")
	student := testpkg.CreateTestStudent(t, db, "MoveAuth", "Rejected", "MAR1")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestVisit(t, db, student.ID, sourceGroup.ID, now.Add(-25*time.Minute), nil)
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, targetGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: staff.ID})

	// #2329: pulling a child out of a colleague's room no longer requires
	// supervising the SOURCE — any staff may move any child; only the TARGET
	// must be a group the caller supervises (which it is here).
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Moved, student.ID)

	currentVisit, err := service.GetStudentCurrentVisit(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, targetGroup.ID, currentVisit.ActiveGroupID)
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_AllowsSupervisedSourceAndTarget(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	sourceActivity := testpkg.CreateTestActivityGroup(t, db, "move-auth-allowed-source")
	targetActivity := testpkg.CreateTestActivityGroup(t, db, "move-auth-allowed-target")
	sourceRoom := testpkg.CreateTestRoom(t, db, "Move Auth Allowed Source Room")
	targetRoom := testpkg.CreateTestRoom(t, db, "Move Auth Allowed Target Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "MoveAuth", "BothRooms")
	device := testpkg.CreateTestDevice(t, db, "move-auth-allow-device")
	student := testpkg.CreateTestStudent(t, db, "MoveAuth", "Allowed", "MAA1")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	sourceVisit := testpkg.CreateTestVisit(t, db, student.ID, sourceGroup.ID, now.Add(-25*time.Minute), nil)
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, sourceGroup.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, targetGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: staff.ID})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []int64{student.ID}, result.Moved)

	endedSourceVisit, err := service.GetVisit(ctx, sourceVisit.ID)
	require.NoError(t, err)
	require.NotNil(t, endedSourceVisit.ExitTime)

	currentVisit, err := service.GetStudentCurrentVisit(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, targetGroup.ID, currentVisit.ActiveGroupID)
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_AllowsOpenTransitIntoSupervisedTarget(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	targetActivity := testpkg.CreateTestActivityGroup(t, db, "move-auth-transit-target")
	targetRoom := testpkg.CreateTestRoom(t, db, "Move Auth Transit Target Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "MoveAuth", "Transit")
	device := testpkg.CreateTestDevice(t, db, "move-auth-transit-device")
	student := testpkg.CreateTestStudent(t, db, "MoveAuth", "Transit", "MAT1")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestGroupSupervisor(t, db, staff.ID, targetGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: staff.ID})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []int64{student.ID}, result.Moved)

	currentVisit, err := service.GetStudentCurrentVisit(ctx, student.ID)
	require.NoError(t, err)
	assert.Equal(t, targetGroup.ID, currentVisit.ActiveGroupID)
}

func TestActiveService_MoveStudentsToTransitAuthorized_AllowsUnsupervisedSource(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	activity := testpkg.CreateTestActivityGroup(t, db, "move-transit-auth-source")
	room := testpkg.CreateTestRoom(t, db, "Move Transit Auth Source Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "MoveTransitAuth", "Staff")
	device := testpkg.CreateTestDevice(t, db, "move-transit-auth-device")
	student := testpkg.CreateTestStudent(t, db, "MoveTransitAuth", "Rejected", "MTAR1")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	visit := testpkg.CreateTestVisit(t, db, student.ID, activeGroup.ID, now.Add(-20*time.Minute), nil)

	result, err := service.MoveStudentsToTransitAuthorized(ctx, []int64{student.ID}, activeSvc.StudentMoveAuthorization{StaffID: staff.ID})

	// #2329: sending a child to transit no longer requires supervising their
	// current room — any staff member may move any child.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Moved, student.ID)

	reloadedVisit, err := service.GetVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.NotNil(t, reloadedVisit.ExitTime, "the transit move closes the source visit")
}

func TestActiveService_MoveStudentsToActiveGroup_BinaryModeReturnsUnchanged(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (?, 'operations.presence_mode', '"binary"', NULL)
		ON CONFLICT (tenant_id, setting_key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = 'operations.presence_mode'`, testpkg.Tenant(t)).Exec(ctx)
	}()

	activity := testpkg.CreateTestActivityGroup(t, db, "move-binary-target")
	room := testpkg.CreateTestRoom(t, db, "Move Binary Target Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	const (
		studentA int64 = 50001
		studentB int64 = 50002
	)
	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{studentA, studentB, studentA}, targetGroup.ID, activeSvcBypassAuth)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ActiveGroupID)
	require.NotNil(t, result.RoomID)
	assert.Equal(t, targetGroup.ID, *result.ActiveGroupID)
	assert.Equal(t, room.ID, *result.RoomID)
	assert.Empty(t, result.Moved)
	assert.ElementsMatch(t, []int64{studentA, studentB}, result.Unchanged)
	assert.Empty(t, result.Skipped)
}

func TestActiveService_MoveStudentsToActiveGroup_BinaryModeRejectsStaleVisit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (?, 'operations.presence_mode', '"binary"', NULL)
		ON CONFLICT (tenant_id, setting_key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = 'operations.presence_mode'`, testpkg.Tenant(t)).Exec(ctx)
	}()

	now := time.Now()
	sourceActivity := testpkg.CreateTestActivityGroup(t, db, "move-binary-stale-source")
	sourceRoom := testpkg.CreateTestRoom(t, db, "Move Binary Stale Source Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, sourceRoom.ID)
	targetActivity := testpkg.CreateTestActivityGroup(t, db, "move-binary-stale-target")
	targetRoom := testpkg.CreateTestRoom(t, db, "Move Binary Stale Target Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "MoveBinaryStale", "Staff")
	device := testpkg.CreateTestDevice(t, db, "move-binary-stale-device")
	student := testpkg.CreateTestStudent(t, db, "MoveBinaryStale", "Student", "MBS1")
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	visit := testpkg.CreateTestVisit(t, db, student.ID, sourceGroup.ID, now.Add(-20*time.Minute), nil)

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, targetGroup.ID, activeSvcBypassAuth)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Moved)
	assert.Empty(t, result.Unchanged)
	assert.Equal(t, []activeSvc.StudentMoveSkipped{{StudentID: student.ID, Reason: activeSvc.StudentMoveSkipConflict}}, result.Skipped)

	reloadedVisit, err := service.GetVisit(ctx, visit.ID)
	require.NoError(t, err)
	assert.Nil(t, reloadedVisit.ExitTime, "rejecting the binary no-op must leave the source visit unchanged")
}

func TestActiveService_MoveStudentsToTransit_EndsVisitKeepsAttendanceOpen(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()

	activity := testpkg.CreateTestActivityGroup(t, db, "move-transit-source")
	room := testpkg.CreateTestRoom(t, db, "Move Transit Source Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, db, "MoveTransit", "Staff")
	device := testpkg.CreateTestDevice(t, db, "move-transit-device")

	inRoomStudent := testpkg.CreateTestStudent(t, db, "TransitMove", "InRoom", "TMI1")
	alreadyTransitStudent := testpkg.CreateTestStudent(t, db, "TransitMove", "Already", "TMA1")
	checkedOutStudent := testpkg.CreateTestStudent(t, db, "TransitMove", "CheckedOut", "TMC1")
	absentStudent := testpkg.CreateTestStudent(t, db, "TransitMove", "Absent", "TMA2")

	inRoomAttendance := testpkg.CreateTestAttendance(t, db, inRoomStudent.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestAttendance(t, db, alreadyTransitStudent.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	checkOutTime := now.Add(-5 * time.Minute)
	testpkg.CreateTestAttendance(t, db, checkedOutStudent.ID, staff.ID, device.ID, now.Add(-30*time.Minute), &checkOutTime)
	visit := testpkg.CreateTestVisit(t, db, inRoomStudent.ID, activeGroup.ID, now.Add(-20*time.Minute), nil)

	result, err := service.MoveStudentsToTransitAuthorized(ctx, []int64{inRoomStudent.ID, alreadyTransitStudent.ID, checkedOutStudent.ID, absentStudent.ID}, activeSvcBypassAuth)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.ActiveGroupID)
	assert.Nil(t, result.RoomID)
	assert.Equal(t, []int64{inRoomStudent.ID}, result.Moved)
	assert.Equal(t, []int64{alreadyTransitStudent.ID}, result.Unchanged)
	require.Len(t, result.Skipped, 2)
	assert.ElementsMatch(t, []activeSvc.StudentMoveSkipped{
		{StudentID: checkedOutStudent.ID, Reason: activeSvc.StudentMoveSkipNotPresent},
		{StudentID: absentStudent.ID, Reason: activeSvc.StudentMoveSkipNotPresent},
	}, result.Skipped)

	endedVisit, err := service.GetVisit(ctx, visit.ID)
	require.NoError(t, err)
	require.NotNil(t, endedVisit.ExitTime)

	_, err = service.GetStudentCurrentVisit(ctx, inRoomStudent.ID)
	assert.ErrorIs(t, err, activeSvc.ErrVisitNotFound)

	var reloaded activeModel.Attendance
	err = db.NewSelect().
		Model(&reloaded).
		ModelTableExpr(`active.attendance`).
		Where("id = ?", inRoomAttendance.ID).
		Scan(ctx)
	require.NoError(t, err)
	assert.Nil(t, reloaded.CheckOutTime, "moving to transit must not perform a daily checkout")
}

func TestActiveService_MoveStudentsToTransit_RejectsWhenNoStudentsPresent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	absentStudent := testpkg.CreateTestStudent(t, db, "TransitMove", "AllAbsent", "TMA3")

	result, err := service.MoveStudentsToTransitAuthorized(ctx, []int64{absentStudent.ID}, activeSvcBypassAuth)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrStudentsNotPresent)
}

func TestActiveService_MoveStudentsToTransit_InvalidInput(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	result, err := service.MoveStudentsToTransitAuthorized(ctx, nil, activeSvcBypassAuth)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrInvalidData)

	result, err = service.MoveStudentsToTransitAuthorized(ctx, []int64{-42}, activeSvcBypassAuth)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrInvalidData)
}

func TestActiveService_MoveStudentsToTransit_BinaryModeReturnsUnchanged(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (?, 'operations.presence_mode', '"binary"', NULL)
		ON CONFLICT (tenant_id, setting_key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = 'operations.presence_mode'`, testpkg.Tenant(t)).Exec(ctx)
	}()

	const (
		studentA int64 = 60001
		studentB int64 = 60002
	)
	result, err := service.MoveStudentsToTransitAuthorized(ctx, []int64{studentA, studentB, studentA}, activeSvcBypassAuth)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.ActiveGroupID)
	assert.Nil(t, result.RoomID)
	assert.Empty(t, result.Moved)
	assert.ElementsMatch(t, []int64{studentA, studentB}, result.Unchanged)
	assert.Empty(t, result.Skipped)
}

func TestActiveService_AssignTransitStudentsToActiveGroup(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
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

	testpkg.CreateTestAttendance(t, db, transitStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	testpkg.CreateTestAttendance(t, db, inRoomStudent.ID, staff.ID, device.ID, now.Add(-20*time.Minute), nil)
	testpkg.CreateTestVisit(t, db, inRoomStudent.ID, sourceGroup.ID, now.Add(-15*time.Minute), nil)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	result, err := service.AssignTransitStudentsToActiveGroup(ctx, nil, 42)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrInvalidData)

	activity := testpkg.CreateTestActivityGroup(t, db, "transit-invalid-input")
	room := testpkg.CreateTestRoom(t, db, "Transit Invalid Input Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)

	result, err = service.AssignTransitStudentsToActiveGroup(ctx, []int64{-42}, targetGroup.ID)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrInvalidData)
}

func TestActiveService_AssignTransitStudentsToActiveGroup_BinaryModeSkipsAll(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	_, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value, updated_by)
		VALUES (?, 'operations.presence_mode', '"binary"', NULL)
		ON CONFLICT (tenant_id, setting_key)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = db.NewRaw(`DELETE FROM config.setting_values WHERE tenant_id = ? AND setting_key = 'operations.presence_mode'`, testpkg.Tenant(t)).Exec(ctx)
	}()

	activity := testpkg.CreateTestActivityGroup(t, db, "transit-binary-target")
	room := testpkg.CreateTestRoom(t, db, "Transit Binary Target Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Transit", "Binary", "TBS1")

	result, err := service.AssignTransitStudentsToActiveGroup(ctx, []int64{student.ID}, targetGroup.ID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, targetGroup.ID, result.ActiveGroupID)
	assert.Equal(t, room.ID, result.RoomID)
	assert.Empty(t, result.Assigned)
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, activeSvc.TransitSkipNotInTransit, result.Skipped[0].Reason)
	assert.Equal(t, student.ID, result.Skipped[0].StudentID)

	// Binary-mode tenants must not gain room-visit rows through this path.
	currentVisit, err := service.GetStudentCurrentVisit(ctx, student.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, activeSvc.ErrVisitNotFound)
	assert.Nil(t, currentVisit)
}

func TestActiveService_AssignTransitStudentsToActiveGroup_EndedTargetFails(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)

	activity := testpkg.CreateTestActivityGroup(t, db, "transit-ended-target")
	room := testpkg.CreateTestRoom(t, db, "Transit Ended Target Room")
	targetGroup := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
	student := testpkg.CreateTestStudent(t, db, "Transit", "Blocked", "TES1")
	endTime := time.Now()
	targetGroup.EndTime = &endTime
	require.NoError(t, service.UpdateActiveGroup(ctx, targetGroup))

	result, err := service.AssignTransitStudentsToActiveGroup(ctx, []int64{student.ID}, targetGroup.ID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, activeSvc.ErrActiveGroupAlreadyEnded)
}

// moveAuthFixture is one present child sitting in a source room plus a target
// session in another room. Supervisions are left to the individual tests so
// each one states the push-or-pull rule (#2969) it pins.
type moveAuthFixture struct {
	staff       *usersModel.Staff
	colleague   *usersModel.Staff
	studentID   int64
	sourceGroup *activeModel.Group
	targetGroup *activeModel.Group
}

func newMoveAuthFixture(t *testing.T, db *bun.DB, tag string) moveAuthFixture {
	t.Helper()
	now := time.Now()

	sourceActivity := testpkg.CreateTestActivityGroup(t, db, tag+"-source")
	targetActivity := testpkg.CreateTestActivityGroup(t, db, tag+"-target")
	sourceRoom := testpkg.CreateTestRoom(t, db, tag+" Source Room")
	targetRoom := testpkg.CreateTestRoom(t, db, tag+" Target Room")
	sourceGroup := testpkg.CreateTestActiveGroup(t, db, sourceActivity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, db, targetActivity.ID, targetRoom.ID)
	staff := testpkg.CreateTestStaff(t, db, "MoveAuth", tag+"-Caller")
	colleague := testpkg.CreateTestStaff(t, db, "MoveAuth", tag+"-Colleague")
	device := testpkg.CreateTestDevice(t, db, tag+"-device")
	student := testpkg.CreateTestStudent(t, db, "MoveAuth", tag, tag)
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestVisit(t, db, student.ID, sourceGroup.ID, now.Add(-25*time.Minute), nil)

	return moveAuthFixture{
		staff:       staff,
		colleague:   colleague,
		studentID:   student.ID,
		sourceGroup: sourceGroup,
		targetGroup: targetGroup,
	}
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_AllowsSupervisedSourceIntoColleagueTarget(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fx := newMoveAuthFixture(t, db, "push")

	// Push (#2969): the caller supervises only the SOURCE; a colleague runs
	// the target session.
	testpkg.CreateTestGroupSupervisor(t, db, fx.staff.ID, fx.sourceGroup.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, fx.colleague.ID, fx.targetGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{fx.studentID}, fx.targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: fx.staff.ID})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []int64{fx.studentID}, result.Moved)

	currentVisit, err := service.GetStudentCurrentVisit(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Equal(t, fx.targetGroup.ID, currentVisit.ActiveGroupID)
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_RejectsWhenNeitherRoomSupervised(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fx := newMoveAuthFixture(t, db, "neither")

	// Both rooms are supervised, but by the colleague, not by the caller.
	testpkg.CreateTestGroupSupervisor(t, db, fx.colleague.ID, fx.sourceGroup.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, fx.colleague.ID, fx.targetGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{fx.studentID}, fx.targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: fx.staff.ID})

	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Nil(t, result)

	currentVisit, err := service.GetStudentCurrentVisit(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Equal(t, fx.sourceGroup.ID, currentVisit.ActiveGroupID, "the child must stay in the source room")
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_RejectsPushIntoUnsupervisedTarget(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fx := newMoveAuthFixture(t, db, "unsupervised")

	// Source supervised by the caller, but nobody is running the target.
	testpkg.CreateTestGroupSupervisor(t, db, fx.staff.ID, fx.sourceGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{fx.studentID}, fx.targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: fx.staff.ID})

	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Nil(t, result)
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_RejectsPushIntoTargetWithFutureSupervision(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fx := newMoveAuthFixture(t, db, "future-supervision")

	// The caller supervises the source room. The target's assignment begins
	// tomorrow, so it cannot make the target room safe for a push today.
	testpkg.CreateTestGroupSupervisor(t, db, fx.staff.ID, fx.sourceGroup.ID, "supervisor")
	targetSupervision := testpkg.CreateTestGroupSupervisor(t, db, fx.colleague.ID, fx.targetGroup.ID, "supervisor")
	targetSupervision.StartDate = timezone.TodayDate().AddDays(1)
	require.NoError(t, service.UpdateGroupSupervisor(ctx, targetSupervision))

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{fx.studentID}, fx.targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: fx.staff.ID})

	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Nil(t, result)

	currentVisit, err := service.GetStudentCurrentVisit(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Equal(t, fx.sourceGroup.ID, currentVisit.ActiveGroupID, "the child must stay in the source room")
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_RejectsPushIntoAmbiguousTargetRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fx := newMoveAuthFixture(t, db, "ambiguous")

	// A second running session in the target room makes the room ambiguous.
	otherActivity := testpkg.CreateTestActivityGroup(t, db, "ambiguous-second")
	testpkg.CreateTestActiveGroup(t, db, otherActivity.ID, fx.targetGroup.RoomID)
	testpkg.CreateTestGroupSupervisor(t, db, fx.staff.ID, fx.sourceGroup.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, fx.colleague.ID, fx.targetGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{fx.studentID}, fx.targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: fx.staff.ID})

	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Nil(t, result)
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_PullIgnoresAmbiguousTargetRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	fx := newMoveAuthFixture(t, db, "pull-ambiguous")

	// Pull (#2329) stays as it was: the caller supervises the target group
	// itself and therefore names it unambiguously, even with a second
	// session in the same room.
	otherActivity := testpkg.CreateTestActivityGroup(t, db, "pull-ambiguous-second")
	testpkg.CreateTestActiveGroup(t, db, otherActivity.ID, fx.targetGroup.RoomID)
	testpkg.CreateTestGroupSupervisor(t, db, fx.staff.ID, fx.targetGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{fx.studentID}, fx.targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: fx.staff.ID})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, []int64{fx.studentID}, result.Moved)
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_RejectsPushOfTransitChild(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()
	fx := newMoveAuthFixture(t, db, "transit-push")

	// A second child is present but has no room: there is no source
	// supervision that could authorize a push, and the caller does not
	// supervise the target.
	device := testpkg.CreateTestDevice(t, db, "transit-push-device")
	transitStudent := testpkg.CreateTestStudent(t, db, "MoveAuth", "Transit", "transit-push-2")
	testpkg.CreateTestAttendance(t, db, transitStudent.ID, fx.staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestGroupSupervisor(t, db, fx.staff.ID, fx.sourceGroup.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, fx.colleague.ID, fx.targetGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{fx.studentID, transitStudent.ID}, fx.targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: fx.staff.ID})

	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Nil(t, result)

	currentVisit, err := service.GetStudentCurrentVisit(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Equal(t, fx.sourceGroup.ID, currentVisit.ActiveGroupID, "a forbidden batch must move nobody")
}

func TestActiveService_MoveStudentsToActiveGroupAuthorized_RejectsPushWithChildFromUnsupervisedRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupActiveService(t, db)
	ctx := testpkg.Ctx(t)
	now := time.Now()
	fx := newMoveAuthFixture(t, db, "mixed-push")

	// The batch mixes a child from the caller's room with a child from a
	// third room the caller does not supervise.
	thirdActivity := testpkg.CreateTestActivityGroup(t, db, "mixed-push-third")
	thirdRoom := testpkg.CreateTestRoom(t, db, "Mixed Push Third Room")
	thirdGroup := testpkg.CreateTestActiveGroup(t, db, thirdActivity.ID, thirdRoom.ID)
	device := testpkg.CreateTestDevice(t, db, "mixed-push-device")
	otherStudent := testpkg.CreateTestStudent(t, db, "MoveAuth", "Third", "mixed-push-2")
	testpkg.CreateTestAttendance(t, db, otherStudent.ID, fx.staff.ID, device.ID, now.Add(-30*time.Minute), nil)
	testpkg.CreateTestVisit(t, db, otherStudent.ID, thirdGroup.ID, now.Add(-25*time.Minute), nil)
	testpkg.CreateTestGroupSupervisor(t, db, fx.staff.ID, fx.sourceGroup.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, fx.colleague.ID, fx.targetGroup.ID, "supervisor")
	testpkg.CreateTestGroupSupervisor(t, db, fx.colleague.ID, thirdGroup.ID, "supervisor")

	result, err := service.MoveStudentsToActiveGroupAuthorized(ctx, []int64{fx.studentID, otherStudent.ID}, fx.targetGroup.ID, activeSvc.StudentMoveAuthorization{StaffID: fx.staff.ID})

	require.ErrorIs(t, err, activeSvc.ErrStudentMoveForbidden)
	assert.Nil(t, result)
}
