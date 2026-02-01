package facilities_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupSchulhofService creates a Schulhof service with real database connection.
func setupSchulhofService(t *testing.T, db *bun.DB) facilitiesSvc.SchulhofService {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	facilityService := facilitiesSvc.NewService(
		repoFactory.Room,
		repoFactory.ActiveGroup,
		db,
	)

	activityService, err := activitiesSvc.NewService(
		repoFactory.ActivityCategory,
		repoFactory.ActivityGroup,
		repoFactory.ActivitySchedule,
		repoFactory.ActivitySupervisor,
		repoFactory.StudentEnrollment,
		db,
	)
	require.NoError(t, err)

	// Create minimal education and users services for active service
	educationService := educationSvc.NewService(
		repoFactory.Group,
		repoFactory.GroupTeacher,
		repoFactory.GroupSubstitution,
		repoFactory.Room,
		repoFactory.Teacher,
		repoFactory.Staff,
		db,
	)

	usersService := usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
		PersonRepo:         repoFactory.Person,
		RFIDRepo:           repoFactory.RFIDCard,
		AccountRepo:        repoFactory.Account,
		PersonGuardianRepo: repoFactory.PersonGuardian,
		StudentRepo:        repoFactory.Student,
		StaffRepo:          repoFactory.Staff,
		TeacherRepo:        repoFactory.Teacher,
		DB:                 db,
	})

	activeService := activeSvc.NewService(activeSvc.ServiceDependencies{
		GroupRepo:          repoFactory.ActiveGroup,
		VisitRepo:          repoFactory.ActiveVisit,
		SupervisorRepo:     repoFactory.GroupSupervisor,
		CombinedGroupRepo:  repoFactory.CombinedGroup,
		GroupMappingRepo:   repoFactory.GroupMapping,
		AttendanceRepo:     repoFactory.Attendance,
		StudentRepo:        repoFactory.Student,
		PersonRepo:         repoFactory.Person,
		TeacherRepo:        repoFactory.Teacher,
		StaffRepo:          repoFactory.Staff,
		RoomRepo:           repoFactory.Room,
		ActivityGroupRepo:  repoFactory.ActivityGroup,
		ActivityCatRepo:    repoFactory.ActivityCategory,
		EducationGroupRepo: repoFactory.Group,
		DeviceRepo:         repoFactory.Device,
		EducationService:   educationService,
		UsersService:       usersService,
		DB:                 db,
		Broadcaster:        nil, // broadcaster not needed for these tests
	})

	return facilitiesSvc.NewSchulhofService(
		facilityService,
		activityService,
		activeService,
		db,
	)
}

// ============================================================================
// GetSchulhofStatus Tests
// ============================================================================

func TestSchulhofService_GetSchulhofStatus_NoInfrastructure(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// ACT
	status, err := service.GetSchulhofStatus(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.False(t, status.Exists)
	assert.Equal(t, constants.SchulhofRoomName, status.RoomName)
	assert.Nil(t, status.RoomID)
	assert.Nil(t, status.ActivityGroupID)
	assert.Nil(t, status.ActiveGroupID)
	assert.False(t, status.IsUserSupervising)
	assert.Equal(t, 0, status.SupervisorCount)
	assert.Equal(t, 0, status.StudentCount)
	assert.Empty(t, status.Supervisors)
}

func TestSchulhofService_GetSchulhofStatus_WithInfrastructureNoSession(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Create infrastructure
	activityGroup, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, activityGroup.CategoryID, *activityGroup.PlannedRoomID)

	// ACT
	status, err := service.GetSchulhofStatus(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Exists)
	assert.NotNil(t, status.RoomID)
	assert.NotNil(t, status.ActivityGroupID)
	assert.Nil(t, status.ActiveGroupID) // No active session today
	assert.False(t, status.IsUserSupervising)
	assert.Equal(t, 0, status.SupervisorCount)
	assert.Equal(t, 0, status.StudentCount)
	assert.Empty(t, status.Supervisors)
}

func TestSchulhofService_GetSchulhofStatus_WithActiveSessionNoSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Create infrastructure and active group
	activeGroup, err := service.GetOrCreateActiveGroup(ctx, staff.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, activeGroup.GroupID, activeGroup.RoomID)

	// ACT
	status, err := service.GetSchulhofStatus(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Exists)
	assert.NotNil(t, status.ActiveGroupID)
	assert.False(t, status.IsUserSupervising)
	assert.Equal(t, 0, status.SupervisorCount)
	assert.Equal(t, 0, status.StudentCount)
	assert.Empty(t, status.Supervisors)
}

func TestSchulhofService_GetSchulhofStatus_WithSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "User")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Create infrastructure and active group
	activeGroup, err := service.GetOrCreateActiveGroup(ctx, staff.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, activeGroup.GroupID, activeGroup.RoomID)

	// Clean up any existing supervisors from previous test runs
	_, err = db.NewDelete().
		Model((*active.GroupSupervisor)(nil)).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".active_group_id = ?`, activeGroup.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Add supervisor
	supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "supervisor")
	defer testpkg.CleanupActivityFixtures(t, db, supervisor.ID)

	// ACT
	status, err := service.GetSchulhofStatus(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.IsUserSupervising)
	assert.NotNil(t, status.SupervisionID)
	assert.Equal(t, supervisor.ID, *status.SupervisionID)
	assert.Equal(t, 1, status.SupervisorCount)
	assert.Len(t, status.Supervisors, 1)
	assert.Equal(t, staff.ID, status.Supervisors[0].StaffID)
	assert.True(t, status.Supervisors[0].IsCurrentUser)
}

func TestSchulhofService_GetSchulhofStatus_WithMultipleSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff1 := testpkg.CreateTestStaff(t, db, "Supervisor", "One")
	staff2 := testpkg.CreateTestStaff(t, db, "Supervisor", "Two")
	defer testpkg.CleanupActivityFixtures(t, db, staff1.ID, staff2.ID)

	// Create infrastructure and active group
	activeGroup, err := service.GetOrCreateActiveGroup(ctx, staff1.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, activeGroup.GroupID, activeGroup.RoomID)

	// Clean up any existing supervisors from previous test runs
	_, err = db.NewDelete().
		Model((*active.GroupSupervisor)(nil)).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where(`"group_supervisor".active_group_id = ?`, activeGroup.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Add two supervisors
	supervisor1 := testpkg.CreateTestGroupSupervisor(t, db, staff1.ID, activeGroup.ID, "supervisor")
	supervisor2 := testpkg.CreateTestGroupSupervisor(t, db, staff2.ID, activeGroup.ID, "supervisor")
	defer testpkg.CleanupActivityFixtures(t, db, supervisor1.ID, supervisor2.ID)

	// ACT - Check status for staff1
	status, err := service.GetSchulhofStatus(ctx, staff1.ID)

	// ASSERT
	require.NoError(t, err)
	assert.True(t, status.IsUserSupervising)
	assert.Equal(t, 2, status.SupervisorCount)
	assert.Len(t, status.Supervisors, 2)

	// Verify current user flag
	var currentUserCount int
	for _, sup := range status.Supervisors {
		if sup.IsCurrentUser {
			currentUserCount++
			assert.Equal(t, staff1.ID, sup.StaffID)
		}
	}
	assert.Equal(t, 1, currentUserCount)
}

func TestSchulhofService_GetSchulhofStatus_WithStudents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, student1.ID, student2.ID)

	// Create infrastructure and active group
	activeGroup, err := service.GetOrCreateActiveGroup(ctx, staff.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, activeGroup.GroupID, activeGroup.RoomID)

	// Clean up any existing visits from previous test runs
	_, err = db.NewDelete().
		Model((*active.Visit)(nil)).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".active_group_id = ?`, activeGroup.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Add visits (one with exit, one without)
	visit1 := testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, time.Now(), nil)
	exitTime := time.Now()
	visit2 := testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, time.Now(), &exitTime)
	defer testpkg.CleanupActivityFixtures(t, db, visit1.ID, visit2.ID)

	// ACT
	status, err := service.GetSchulhofStatus(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 1, status.StudentCount) // Only student1 (no exit time)
}

// ============================================================================
// ToggleSupervision Tests
// ============================================================================

func TestSchulhofService_ToggleSupervision_StartSuccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// ACT
	result, err := service.ToggleSupervision(ctx, staff.ID, "start")

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "started", result.Action)
	assert.NotNil(t, result.SupervisionID)
	assert.NotZero(t, result.ActiveGroupID)

	// Cleanup
	if result.SupervisionID != nil {
		testpkg.CleanupActivityFixtures(t, db, *result.SupervisionID)
	}
	testpkg.CleanupActivityFixtures(t, db, result.ActiveGroupID)

	// Find created activity group and cleanup
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.SchulhofActivityName)
	options.Filter = filter
	repoFactory := repositories.NewFactory(db)
	activityService, _ := activitiesSvc.NewService(repoFactory.ActivityCategory, repoFactory.ActivityGroup, repoFactory.ActivitySchedule, repoFactory.ActivitySupervisor, repoFactory.StudentEnrollment, db)
	groups, _ := activityService.ListGroups(ctx, options)
	if len(groups) > 0 {
		testpkg.CleanupActivityFixtures(t, db, groups[0].ID, groups[0].CategoryID)
		if groups[0].PlannedRoomID != nil {
			testpkg.CleanupActivityFixtures(t, db, *groups[0].PlannedRoomID)
		}
	}
}

func TestSchulhofService_ToggleSupervision_StopSuccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Start supervision first
	startResult, err := service.ToggleSupervision(ctx, staff.ID, "start")
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, startResult.ActiveGroupID)

	// ACT - Stop supervision
	stopResult, err := service.ToggleSupervision(ctx, staff.ID, "stop")

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, stopResult)
	assert.Equal(t, "stopped", stopResult.Action)
	assert.NotZero(t, stopResult.ActiveGroupID)

	// Cleanup
	if startResult.SupervisionID != nil {
		testpkg.CleanupActivityFixtures(t, db, *startResult.SupervisionID)
	}

	// Find created activity group and cleanup
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.SchulhofActivityName)
	options.Filter = filter
	repoFactory := repositories.NewFactory(db)
	activityService, _ := activitiesSvc.NewService(repoFactory.ActivityCategory, repoFactory.ActivityGroup, repoFactory.ActivitySchedule, repoFactory.ActivitySupervisor, repoFactory.StudentEnrollment, db)
	groups, _ := activityService.ListGroups(ctx, options)
	if len(groups) > 0 {
		testpkg.CleanupActivityFixtures(t, db, groups[0].ID, groups[0].CategoryID)
		if groups[0].PlannedRoomID != nil {
			testpkg.CleanupActivityFixtures(t, db, *groups[0].PlannedRoomID)
		}
	}
}

func TestSchulhofService_ToggleSupervision_StopNotSupervising(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Create infrastructure and active group (but don't add as supervisor)
	activeGroup, err := service.GetOrCreateActiveGroup(ctx, staff.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, activeGroup.GroupID, activeGroup.RoomID)

	// ACT - Try to stop when not supervising
	result, err := service.ToggleSupervision(ctx, staff.ID, "stop")

	// ASSERT
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not currently supervising")
}

func TestSchulhofService_ToggleSupervision_InvalidAction(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// ACT
	result, err := service.ToggleSupervision(ctx, staff.ID, "invalid")

	// ASSERT
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid action")
}

// ============================================================================
// EnsureInfrastructure Tests
// ============================================================================

func TestSchulhofService_EnsureInfrastructure_CreatesAll(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// ACT
	activityGroup, err := service.EnsureInfrastructure(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, activityGroup)
	assert.Equal(t, constants.SchulhofActivityName, activityGroup.Name)
	assert.Equal(t, constants.SchulhofMaxParticipants, activityGroup.MaxParticipants)
	assert.True(t, activityGroup.IsOpen)
	assert.NotZero(t, activityGroup.CategoryID)
	assert.NotNil(t, activityGroup.PlannedRoomID)

	// Cleanup
	testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, activityGroup.CategoryID, *activityGroup.PlannedRoomID)
}

func TestSchulhofService_EnsureInfrastructure_Idempotent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Create infrastructure first time
	activityGroup1, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activityGroup1.ID, activityGroup1.CategoryID, *activityGroup1.PlannedRoomID)

	// ACT - Call again (should return existing)
	activityGroup2, err := service.EnsureInfrastructure(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, activityGroup2)
	assert.Equal(t, activityGroup1.ID, activityGroup2.ID)
	assert.Equal(t, activityGroup1.CategoryID, activityGroup2.CategoryID)
	assert.Equal(t, activityGroup1.PlannedRoomID, activityGroup2.PlannedRoomID)
}

// ============================================================================
// GetOrCreateActiveGroup Tests
// ============================================================================

func TestSchulhofService_GetOrCreateActiveGroup_Creates(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// ACT
	activeGroup, err := service.GetOrCreateActiveGroup(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, activeGroup)
	assert.NotZero(t, activeGroup.ID)
	assert.NotZero(t, activeGroup.GroupID)
	assert.NotZero(t, activeGroup.RoomID)
	assert.WithinDuration(t, time.Now(), activeGroup.StartTime, 5*time.Second)

	// Cleanup
	testpkg.CleanupActivityFixtures(t, db, activeGroup.ID, activeGroup.GroupID, activeGroup.RoomID)

	// Find created activity group and cleanup
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.SchulhofActivityName)
	options.Filter = filter
	repoFactory := repositories.NewFactory(db)
	activityService, _ := activitiesSvc.NewService(repoFactory.ActivityCategory, repoFactory.ActivityGroup, repoFactory.ActivitySchedule, repoFactory.ActivitySupervisor, repoFactory.StudentEnrollment, db)
	groups, _ := activityService.ListGroups(ctx, options)
	if len(groups) > 0 {
		testpkg.CleanupActivityFixtures(t, db, groups[0].CategoryID)
	}
}

func TestSchulhofService_GetOrCreateActiveGroup_ReturnsExisting(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Create first time
	activeGroup1, err := service.GetOrCreateActiveGroup(ctx, staff.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activeGroup1.ID, activeGroup1.GroupID, activeGroup1.RoomID)

	// ACT - Call again (should return same group)
	activeGroup2, err := service.GetOrCreateActiveGroup(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, activeGroup2)
	assert.Equal(t, activeGroup1.ID, activeGroup2.ID)
	assert.Equal(t, activeGroup1.GroupID, activeGroup2.GroupID)
	assert.Equal(t, activeGroup1.RoomID, activeGroup2.RoomID)

	// Find created activity group and cleanup
	options := base.NewQueryOptions()
	filter := base.NewFilter()
	filter.Equal("name", constants.SchulhofActivityName)
	options.Filter = filter
	repoFactory := repositories.NewFactory(db)
	activityService, _ := activitiesSvc.NewService(repoFactory.ActivityCategory, repoFactory.ActivityGroup, repoFactory.ActivitySchedule, repoFactory.ActivitySupervisor, repoFactory.StudentEnrollment, db)
	groups, _ := activityService.ListGroups(ctx, options)
	if len(groups) > 0 {
		testpkg.CleanupActivityFixtures(t, db, groups[0].CategoryID)
	}
}

func TestSchulhofService_GetOrCreateActiveGroup_IgnoresEndedGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupSchulhofService(t, db)
	ctx := context.Background()

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	// Ensure infrastructure exists
	activityGroup, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, activityGroup.ID, activityGroup.CategoryID, *activityGroup.PlannedRoomID)

	repoFactory := repositories.NewFactory(db)

	// Clean up any existing active groups for this room from previous test runs
	// This handles test pollution when tests run in parallel or don't clean up properly
	_, err = db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, *activityGroup.PlannedRoomID).
		Exec(ctx)
	require.NoError(t, err)

	// Create an ended active group from today
	endedTime := time.Now()
	endedGroup := &active.Group{
		GroupID:   activityGroup.ID,
		RoomID:    *activityGroup.PlannedRoomID,
		StartTime: time.Now().Add(-2 * time.Hour),
		EndTime:   &endedTime,
	}

	// Create minimal dependencies for active service
	educationService := educationSvc.NewService(
		repoFactory.Group,
		repoFactory.GroupTeacher,
		repoFactory.GroupSubstitution,
		repoFactory.Room,
		repoFactory.Teacher,
		repoFactory.Staff,
		db,
	)

	usersService := usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
		PersonRepo:         repoFactory.Person,
		RFIDRepo:           repoFactory.RFIDCard,
		AccountRepo:        repoFactory.Account,
		PersonGuardianRepo: repoFactory.PersonGuardian,
		StudentRepo:        repoFactory.Student,
		StaffRepo:          repoFactory.Staff,
		TeacherRepo:        repoFactory.Teacher,
		DB:                 db,
	})

	activeService := activeSvc.NewService(activeSvc.ServiceDependencies{
		GroupRepo:          repoFactory.ActiveGroup,
		VisitRepo:          repoFactory.ActiveVisit,
		SupervisorRepo:     repoFactory.GroupSupervisor,
		CombinedGroupRepo:  repoFactory.CombinedGroup,
		GroupMappingRepo:   repoFactory.GroupMapping,
		AttendanceRepo:     repoFactory.Attendance,
		StudentRepo:        repoFactory.Student,
		PersonRepo:         repoFactory.Person,
		TeacherRepo:        repoFactory.Teacher,
		StaffRepo:          repoFactory.Staff,
		RoomRepo:           repoFactory.Room,
		ActivityGroupRepo:  repoFactory.ActivityGroup,
		ActivityCatRepo:    repoFactory.ActivityCategory,
		EducationGroupRepo: repoFactory.Group,
		DeviceRepo:         repoFactory.Device,
		EducationService:   educationService,
		UsersService:       usersService,
		DB:                 db,
		Broadcaster:        nil,
	})
	err = activeService.CreateActiveGroup(ctx, endedGroup)
	require.NoError(t, err)
	defer testpkg.CleanupActivityFixtures(t, db, endedGroup.ID)

	// ACT - Should create a new one, not return the ended one
	activeGroup, err := service.GetOrCreateActiveGroup(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, activeGroup)
	assert.NotEqual(t, endedGroup.ID, activeGroup.ID) // Should be a different group
	assert.Nil(t, activeGroup.EndTime)                // Should not be ended

	// Cleanup
	testpkg.CleanupActivityFixtures(t, db, activeGroup.ID)
}
