package facilities_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupSchulhofService creates a Schulhof service with real database connection.
func setupSchulhofService(t *testing.T, db *bun.DB) facilitiesSvc.SchulhofService {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	facilityService := facilitiesSvc.NewServiceWithConfig(facilitiesSvc.ServiceConfig{
		RoomRepo: repoFactory.Room, ActiveGroupRepo: repoFactory.ActiveGroup,
	})

	activityService, err := activitiesSvc.NewService(
		repoFactory.ActivityCategory,
		repoFactory.ActivityGroup,
		repoFactory.ActivitySchedule,
		repoFactory.ActivitySupervisor,
		repoFactory.StudentEnrollment,
		repoFactory.ActiveGroup,
		repoFactory.Staff,
		repoFactory.Student,
	)
	require.NoError(t, err)

	// Create minimal education and users services for active service
	educationService := educationSvc.NewService(
		repoFactory.Group,
		repoFactory.GroupTeacher,
		repoFactory.ClassTeacher,
		repoFactory.GroupSubstitution,
		repoFactory.Room,
		repoFactory.Teacher,
		repoFactory.Staff,
		repoFactory.Student,
	)

	usersService := usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
		PersonRepo:  repoFactory.Person,
		RFIDRepo:    repoFactory.RFIDCard,
		AccountRepo: repoFactory.Account,
		StudentRepo: repoFactory.Student,
		StaffRepo:   repoFactory.Staff,
		TeacherRepo: repoFactory.Teacher,
		DB:          db,
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
		slog.Default(),
	)
}

// createOpenSchulhofGroup provisions the Schulhof infrastructure and inserts an
// open active group in the Schulhof room, mirroring what a started planned
// block, a spontaneous session, or an IoT scan produces (#2161). Replaces the
// retired GetOrCreateActiveGroup as fixture setup.
func createOpenSchulhofGroup(t *testing.T, db *bun.DB, service facilitiesSvc.SchulhofService, ctx context.Context, startTime time.Time) *active.Group {
	t.Helper()

	activityGroup, err := service.EnsureInfrastructure(ctx, 0)
	require.NoError(t, err)
	require.NotNil(t, activityGroup.PlannedRoomID, "EnsureInfrastructure should set PlannedRoomID")

	activityGroupID := activityGroup.ID
	group := &active.Group{
		GroupID:   &activityGroupID,
		RoomID:    *activityGroup.PlannedRoomID,
		StartTime: startTime,
	}
	group.SetTenantID(tenant.FromContext(ctx))
	repoFactory := repositories.NewFactory(db)
	require.NoError(t, repoFactory.ActiveGroup.Create(ctx, group))
	return group
}

// ============================================================================
// GetSchulhofStatus Tests
// ============================================================================

func TestSchulhofService_GetSchulhofStatus_NoInfrastructure(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Test", "Staff")

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

func TestSchulhofService_EnsureInfrastructureRejectsLegacyActivityInNonCanonicalRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)
	repoFactory := repositories.NewFactory(db)
	legacyRoom := &facilitiesModel.Room{Name: "schulhof", Building: "Außengelände"}
	legacyRoom.SetTenantID(tenantID)
	require.NoError(t, repoFactory.Room.Create(ctx, legacyRoom))
	legacyCategory := &activityModels.Category{
		Name:        constants.SchulhofCategoryName,
		Description: constants.SchulhofCategoryDescription,
		Color:       constants.SchulhofColor,
		IsSystem:    true,
	}
	legacyCategory.SetTenantID(tenantID)
	require.NoError(t, repoFactory.ActivityCategory.Create(ctx, legacyCategory))
	legacyActivity := &activityModels.Group{
		Name:            constants.SchulhofActivityName,
		MaxParticipants: constants.SchulhofMaxParticipants,
		IsOpen:          true,
		CategoryID:      legacyCategory.ID,
		PlannedRoomID:   &legacyRoom.ID,
		IsSystem:        true,
	}
	legacyActivity.SetTenantID(tenantID)
	require.NoError(t, repoFactory.ActivityGroup.Create(ctx, legacyActivity))
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Test", "Staff")
	service := setupSchulhofService(t, db)

	status, statusErr := service.GetSchulhofStatus(ctx, staff.ID)
	require.Error(t, statusErr)
	assert.Contains(t, statusErr.Error(), "non-canonical room name")
	assert.Nil(t, status)

	activityGroup, err := service.EnsureInfrastructure(ctx, staff.ID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-canonical room name")
	assert.Nil(t, activityGroup)
	persisted, findErr := repoFactory.Room.FindByID(ctx, legacyRoom.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "schulhof", persisted.Name)
	assert.False(t, persisted.IsSystem, "legacy case variant must never be adopted as Schulhof infrastructure")
}

func TestSchulhofService_GetSchulhofStatus_WithInfrastructureNoSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")

	// Create infrastructure
	_, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")

	// Create infrastructure and active group
	_ = createOpenSchulhofGroup(t, db, service, ctx, time.Now())

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Supervisor", "User")

	// First ensure infrastructure to get the room ID
	activityGroup, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)

	// End all existing active groups for this room to get a fresh one
	require.NotNil(t, activityGroup.PlannedRoomID, "EnsureInfrastructure should set PlannedRoomID")
	_, err = db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, *activityGroup.PlannedRoomID).
		Exec(ctx)
	require.NoError(t, err)

	// Now create fresh active group
	activeGroup := createOpenSchulhofGroup(t, db, service, ctx, time.Now())

	// Add supervisor
	supervisor := testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "supervisor")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	// Clean up any Schulhof artifacts left by parallel test packages

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff1 := testpkg.CreateTestStaff(t, db, "Supervisor", "One")
	staff2 := testpkg.CreateTestStaff(t, db, "Supervisor", "Two")

	// First ensure infrastructure to get the room ID
	activityGroup, err := service.EnsureInfrastructure(ctx, staff1.ID)
	require.NoError(t, err)

	// End all existing active groups for this room to get a fresh one
	require.NotNil(t, activityGroup.PlannedRoomID, "EnsureInfrastructure should set PlannedRoomID")
	_, err = db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, *activityGroup.PlannedRoomID).
		Exec(ctx)
	require.NoError(t, err)

	// Now create fresh active group
	activeGroup := createOpenSchulhofGroup(t, db, service, ctx, time.Now())

	// Add two supervisors
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff1.ID, activeGroup.ID, "supervisor")
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff2.ID, activeGroup.ID, "supervisor")

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
	t.Parallel()

	// Use a dedicated DB connection to avoid cross-test interference.
	// CleanupActivityFixtures silently fails (BUN nil model bug), so stale
	// active groups from earlier tests can cause findTodayActiveGroup to
	// return a group that has no visits, yielding StudentCount=0.
	db := testpkg.SetupTestDB(t)

	// Clean up any Schulhof artifacts left by parallel test packages (e.g. api/iot/checkin)
	// to ensure findSchulhofActivity returns the activity group created by THIS test.

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1a")

	// First ensure infrastructure to get the room ID
	activityGroup, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)

	// End ALL existing active groups for this activity AND room to get a clean slate.
	// This prevents flakiness from stale groups left by previous test runs that may
	// reference different room IDs (each test creates its own infrastructure).
	_, err = db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".end_time IS NULL AND ("group".room_id = ? OR "group".group_id = ?)`,
			*activityGroup.PlannedRoomID, activityGroup.ID).
		Exec(ctx)
	require.NoError(t, err)

	// Now create fresh active group
	activeGroup := createOpenSchulhofGroup(t, db, service, ctx, time.Now())

	// Add visits (one with exit, one without)
	// Note: entry_time must be captured BEFORE exit_time to satisfy the
	// chk_entry_before_exit constraint (entry_time <= exit_time)
	entryTime := time.Now()
	_ = testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, entryTime, nil)
	exitTime := time.Now()
	_ = testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, entryTime, &exitTime)

	// ACT
	status, err := service.GetSchulhofStatus(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, status.ActiveGroupID, "Should have an active group for Schulhof")
	assert.Equal(t, activeGroup.ID, *status.ActiveGroupID, "GetSchulhofStatus should find the active group we created")
	assert.Equal(t, 1, status.StudentCount) // Only student1 (no exit time)
}

// ============================================================================
// EnsureInfrastructure Tests
// ============================================================================

func TestSchulhofService_EnsureInfrastructure_CreatesAll(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")

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
}

func TestSchulhofService_EnsureInfrastructure_Idempotent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")

	// Create infrastructure first time
	activityGroup1, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)

	// ACT - Call again (should return existing)
	activityGroup2, err := service.EnsureInfrastructure(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, activityGroup2)
	assert.Equal(t, activityGroup1.ID, activityGroup2.ID)
	assert.Equal(t, activityGroup1.CategoryID, activityGroup2.CategoryID)
	assert.Equal(t, activityGroup1.PlannedRoomID, activityGroup2.PlannedRoomID)
}

// TestSchulhofService_GetSchulhofStatus_SkipsEndedSupervisions verifies that ended
// supervisions are excluded from the status response.
func TestSchulhofService_GetSchulhofStatus_SkipsEndedSupervisions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff1 := testpkg.CreateTestStaff(t, db, "Active", "Supervisor")
	staff2 := testpkg.CreateTestStaff(t, db, "Ended", "Supervisor")

	// Ensure infrastructure
	activityGroup, err := service.EnsureInfrastructure(ctx, staff1.ID)
	require.NoError(t, err)

	// End all existing active groups for this room
	_, err = db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, *activityGroup.PlannedRoomID).
		Exec(ctx)
	require.NoError(t, err)

	// Create fresh active group
	activeGroup := createOpenSchulhofGroup(t, db, service, ctx, time.Now())

	// Add active supervisor
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff1.ID, activeGroup.ID, "supervisor")

	// Add supervisor with end_date (should be excluded)
	supervisor2 := testpkg.CreateTestGroupSupervisor(t, db, staff2.ID, activeGroup.ID, "supervisor")

	// Set end_date on supervisor2 to mark it as ended
	endTime := time.Now()
	_, err = db.NewUpdate().
		Model((*active.GroupSupervisor)(nil)).
		ModelTableExpr(`active.group_supervisors AS "supervisor"`).
		Set("end_date = ?", endTime).
		Where(`"supervisor".id = ?`, supervisor2.ID).
		Exec(ctx)
	require.NoError(t, err)

	// ACT
	status, err := service.GetSchulhofStatus(ctx, staff1.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, status)
	// Should only count the active supervisor (staff1), not the ended one (staff2)
	assert.Equal(t, 1, status.SupervisorCount, "Ended supervisors should be excluded from count")
	assert.Len(t, status.Supervisors, 1, "Ended supervisors should be excluded from list")
	assert.Equal(t, staff1.ID, status.Supervisors[0].StaffID)
}

// TestSchulhofService_GetSchulhofStatus_OtherStaffNotSupervising verifies that status
// correctly shows IsUserSupervising=false for a staff member who is NOT the supervisor.
func TestSchulhofService_GetSchulhofStatus_OtherStaffNotSupervising(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff1 := testpkg.CreateTestStaff(t, db, "Actual", "Supervisor")
	staff2 := testpkg.CreateTestStaff(t, db, "Other", "Staff")

	// Ensure infrastructure
	activityGroup, err := service.EnsureInfrastructure(ctx, staff1.ID)
	require.NoError(t, err)

	// End all existing active groups
	_, err = db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, *activityGroup.PlannedRoomID).
		Exec(ctx)
	require.NoError(t, err)

	// Create fresh active group
	activeGroup := createOpenSchulhofGroup(t, db, service, ctx, time.Now())

	// Add staff1 as supervisor
	_ = testpkg.CreateTestGroupSupervisor(t, db, staff1.ID, activeGroup.ID, "supervisor")

	// ACT — Check status for staff2 (not the supervisor)
	status, err := service.GetSchulhofStatus(ctx, staff2.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Exists)
	assert.NotNil(t, status.ActiveGroupID)
	assert.False(t, status.IsUserSupervising, "Other staff should NOT be supervising")
	assert.Nil(t, status.SupervisionID, "Other staff should have no supervision ID")
	assert.Equal(t, 1, status.SupervisorCount)
	assert.Len(t, status.Supervisors, 1)
	// The supervisor should have IsCurrentUser=false from staff2's perspective
	assert.False(t, status.Supervisors[0].IsCurrentUser)
}

// TestSchulhofService_EnsureInfrastructure_ExistingRoomAndCategory verifies that
// ensureSchulhofRoom and ensureSchulhofCategory reuse existing entities when
// only the activity group is missing.
func TestSchulhofService_EnsureInfrastructure_ExistingRoomAndCategory(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")

	// First call creates everything
	activityGroup1, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, activityGroup1)

	roomID := *activityGroup1.PlannedRoomID
	categoryID := activityGroup1.CategoryID

	// Delete only the activity group to simulate partial infrastructure
	// Ignore error — there might be no active groups yet
	_, _ = db.NewDelete().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".group_id = ?`, activityGroup1.ID).
		Exec(ctx)

	// Delete the activity group itself
	_, err = db.NewDelete().
		TableExpr("activities.groups").
		Where("id = ?", activityGroup1.ID).
		Exec(ctx)
	require.NoError(t, err)

	// ACT — EnsureInfrastructure should reuse existing room and category
	activityGroup2, err := service.EnsureInfrastructure(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, activityGroup2)
	assert.NotEqual(t, activityGroup1.ID, activityGroup2.ID, "New activity group should have a different ID")
	assert.Equal(t, roomID, *activityGroup2.PlannedRoomID, "Should reuse existing room")
	assert.Equal(t, categoryID, activityGroup2.CategoryID, "Should reuse existing category")
}

// TestSchulhofService_GetSchulhofStatus_WithStudentsAllExited verifies that
// student count is 0 when all visits have exit times.
func TestSchulhofService_GetSchulhofStatus_WithStudentsAllExited(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupSchulhofService(t, db)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Test", "Staff")
	student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
	student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1a")

	// Ensure infrastructure
	activityGroup, err := service.EnsureInfrastructure(ctx, staff.ID)
	require.NoError(t, err)

	// End all existing active groups for this room
	_, err = db.NewUpdate().
		Model((*active.Group)(nil)).
		ModelTableExpr(`active.groups AS "group"`).
		Set("end_time = ?", time.Now()).
		Where(`"group".room_id = ? AND "group".end_time IS NULL`, *activityGroup.PlannedRoomID).
		Exec(ctx)
	require.NoError(t, err)

	// Create fresh active group
	activeGroup := createOpenSchulhofGroup(t, db, service, ctx, time.Now())

	// Add visits where ALL have exited (both have exit times)
	entryTime := time.Now()
	exitTime := time.Now()
	_ = testpkg.CreateTestVisit(t, db, student1.ID, activeGroup.ID, entryTime, &exitTime)
	_ = testpkg.CreateTestVisit(t, db, student2.ID, activeGroup.ID, entryTime, &exitTime)

	// ACT
	status, err := service.GetSchulhofStatus(ctx, staff.ID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 0, status.StudentCount, "All students have exited, count should be 0")
}
