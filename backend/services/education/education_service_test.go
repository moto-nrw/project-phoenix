package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupEducationService creates an education service with real database connection.
func setupEducationService(t *testing.T, db *bun.DB) educationSvc.Service {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	return educationSvc.NewService(
		repoFactory.Group,
		repoFactory.GroupTeacher,
		repoFactory.GroupSubstitution,
		repoFactory.Room,
		repoFactory.Teacher,
		repoFactory.Staff,
		db,
	)
}

// ============================================================================
// TestListGroups - Tests for listing education groups with filters
// ============================================================================

func TestListGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("successful list with name filter", func(t *testing.T) {
		// ARRANGE: Create groups with specific names
		group1 := testpkg.CreateTestEducationGroup(t, db, "Math")
		group2 := testpkg.CreateTestEducationGroup(t, db, "Science")
		group3 := testpkg.CreateTestEducationGroup(t, db, "Math") // Another Math group

		defer testpkg.CleanupActivityFixtures(t, db, group1.ID, group2.ID, group3.ID)

		// ACT: List all groups (no specific filter - verify our groups exist)
		groups, err := service.ListGroups(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		// Verify our created groups are in the list
		foundGroup1, foundGroup2, foundGroup3 := false, false, false
		for _, g := range groups {
			if g.ID == group1.ID {
				foundGroup1 = true
			}
			if g.ID == group2.ID {
				foundGroup2 = true
			}
			if g.ID == group3.ID {
				foundGroup3 = true
			}
		}
		assert.True(t, foundGroup1, "group1 should be in list")
		assert.True(t, foundGroup2, "group2 should be in list")
		assert.True(t, foundGroup3, "group3 should be in list")
	})

	t.Run("list with pagination", func(t *testing.T) {
		// ARRANGE: Create a few groups
		group1 := testpkg.CreateTestEducationGroup(t, db, "PaginationTest")
		group2 := testpkg.CreateTestEducationGroup(t, db, "PaginationTest")

		defer testpkg.CleanupActivityFixtures(t, db, group1.ID, group2.ID)

		// ACT: List with pagination
		options := base.NewQueryOptions().WithPagination(1, 100)
		groups, err := service.ListGroups(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})

	t.Run("list with nil options returns all groups", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "NilOptionsTest")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID)

		// ACT
		groups, err := service.ListGroups(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		// Verify our group is in the list
		found := false
		for _, g := range groups {
			if g.ID == group.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Created group should be in list")
	})
}

// ============================================================================
// TestListSubstitutions - Tests for listing substitutions with filters
// ============================================================================

func TestListSubstitutions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("successful list substitutions", func(t *testing.T) {
		// ARRANGE: Create required entities
		group := testpkg.CreateTestEducationGroup(t, db, "SubstitutionListGroup")
		regularStaff := testpkg.CreateTestStaff(t, db, "Regular", "ListStaff")
		substituteStaff := testpkg.CreateTestStaff(t, db, "Substitute", "ListStaff")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, regularStaff.ID, substituteStaff.ID)

		// Create a substitution for future dates (service validates no backdating)
		tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		nextWeek := tomorrow.AddDate(0, 0, 7)

		sub := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    &regularStaff.ID,
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         tomorrow,
			EndDate:           nextWeek,
			Reason:            "Test substitution for list",
		}
		err := service.CreateSubstitution(ctx, sub)
		require.NoError(t, err)

		// ACT: List substitutions
		substitutions, err := service.ListSubstitutions(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, substitutions)

		// Find our substitution
		found := false
		for _, s := range substitutions {
			if s.GroupID == group.ID && s.SubstituteStaffID == substituteStaff.ID {
				found = true
				assert.Equal(t, "Test substitution for list", s.Reason)
				break
			}
		}
		assert.True(t, found, "Created substitution should be in list")
	})
}

// ============================================================================
// TestGetGroupTeachers - Tests for retrieving teachers assigned to a group
// ============================================================================

func TestGetGroupTeachers(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("successful get teachers for group", func(t *testing.T) {
		// ARRANGE: Create group and teachers
		group := testpkg.CreateTestEducationGroup(t, db, "TeacherTestGroup")
		teacher1 := testpkg.CreateTestTeacher(t, db, "Teacher", "One")
		teacher2 := testpkg.CreateTestTeacher(t, db, "Teacher", "Two")
		teacher3 := testpkg.CreateTestTeacher(t, db, "Teacher", "Three")

		// Assign teachers to group
		gt1 := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher1.ID)
		gt2 := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher2.ID)
		gt3 := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher3.ID)

		// Get staff IDs for cleanup (teachers depend on staff)
		staff1ID := teacher1.Staff.ID
		staff2ID := teacher2.Staff.ID
		staff3ID := teacher3.Staff.ID

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID,
			teacher1.ID, teacher2.ID, teacher3.ID,
			gt1.ID, gt2.ID, gt3.ID,
			staff1ID, staff2ID, staff3ID)

		// ACT
		teachers, err := service.GetGroupTeachers(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, teachers, 3)

		teacherIDs := make(map[int64]bool)
		for _, t := range teachers {
			teacherIDs[t.ID] = true
		}
		assert.True(t, teacherIDs[teacher1.ID], "teacher1 should be in result")
		assert.True(t, teacherIDs[teacher2.ID], "teacher2 should be in result")
		assert.True(t, teacherIDs[teacher3.ID], "teacher3 should be in result")
	})

	t.Run("returns empty for group with no teachers", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "EmptyTeacherGroup")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID)

		// ACT
		teachers, err := service.GetGroupTeachers(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, teachers)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ACT
		_, err := service.GetGroupTeachers(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// ============================================================================
// TestCreateSubstitution_DateValidation - Tests for substitution date validation
// ============================================================================

func TestCreateSubstitution_DateValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("accepts future date", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "FutureSubGroup")
		regularStaff := testpkg.CreateTestStaff(t, db, "FutureReg", "Staff")
		substituteStaff := testpkg.CreateTestStaff(t, db, "FutureSub", "Staff")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, regularStaff.ID, substituteStaff.ID)

		tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		nextWeek := tomorrow.AddDate(0, 0, 7)

		sub := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    &regularStaff.ID,
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         tomorrow,
			EndDate:           nextWeek,
			Reason:            "Medical leave",
		}

		// ACT
		err := service.CreateSubstitution(ctx, sub)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, sub.ID, "Substitution should have been assigned an ID")
	})

	t.Run("rejects past date", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "PastSubGroup")
		regularStaff := testpkg.CreateTestStaff(t, db, "PastReg", "Staff")
		substituteStaff := testpkg.CreateTestStaff(t, db, "PastSub", "Staff")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, regularStaff.ID, substituteStaff.ID)

		yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
		today := time.Now().Truncate(24 * time.Hour)

		sub := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    &regularStaff.ID,
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         yesterday,
			EndDate:           today,
			Reason:            "Trying to backdate",
		}

		// ACT
		err := service.CreateSubstitution(ctx, sub)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "past dates")
	})

	t.Run("accepts today's date", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "TodaySubGroup")
		regularStaff := testpkg.CreateTestStaff(t, db, "TodayReg", "Staff")
		substituteStaff := testpkg.CreateTestStaff(t, db, "TodaySub", "Staff")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, regularStaff.ID, substituteStaff.ID)

		today := time.Now().Truncate(24 * time.Hour)
		nextWeek := today.AddDate(0, 0, 7)

		sub := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    &regularStaff.ID,
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         today,
			EndDate:           nextWeek,
			Reason:            "Emergency coverage",
		}

		// ACT
		err := service.CreateSubstitution(ctx, sub)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, sub.ID, "Substitution should have been assigned an ID")
	})

	t.Run("allows substitution without regular staff", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "NoRegularStaffGroup")
		substituteStaff := testpkg.CreateTestStaff(t, db, "OnlySub", "Staff")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, substituteStaff.ID)

		tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		nextWeek := tomorrow.AddDate(0, 0, 7)

		sub := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    nil, // No regular staff
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         tomorrow,
			EndDate:           nextWeek,
			Reason:            "Additional coverage",
		}

		// ACT
		err := service.CreateSubstitution(ctx, sub)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, sub.ID)
	})
}

// ============================================================================
// TestUpdateSubstitution_DateValidation - Tests for update validation
// ============================================================================

func TestUpdateSubstitution_DateValidation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("accepts future date update", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "UpdateFutureGroup")
		regularStaff := testpkg.CreateTestStaff(t, db, "UpdateFutureReg", "Staff")
		substituteStaff := testpkg.CreateTestStaff(t, db, "UpdateFutureSub", "Staff")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, regularStaff.ID, substituteStaff.ID)

		tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		nextWeek := tomorrow.AddDate(0, 0, 7)

		// Create initial substitution
		sub := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    &regularStaff.ID,
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         tomorrow,
			EndDate:           nextWeek,
			Reason:            "Original reason",
		}
		err := service.CreateSubstitution(ctx, sub)
		require.NoError(t, err)

		// ACT: Update to extend end date
		sub.EndDate = nextWeek.AddDate(0, 0, 7)
		sub.Reason = "Extended leave"
		err = service.UpdateSubstitution(ctx, sub)

		// ASSERT
		require.NoError(t, err)

		// Verify the update persisted
		updated, err := service.GetSubstitution(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, "Extended leave", updated.Reason)
	})

	t.Run("rejects backdated update", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "BackdateUpdateGroup")
		regularStaff := testpkg.CreateTestStaff(t, db, "BackdateReg", "Staff")
		substituteStaff := testpkg.CreateTestStaff(t, db, "BackdateSub", "Staff")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, regularStaff.ID, substituteStaff.ID)

		tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		nextWeek := tomorrow.AddDate(0, 0, 7)

		// Create initial substitution
		sub := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    &regularStaff.ID,
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         tomorrow,
			EndDate:           nextWeek,
			Reason:            "Original reason",
		}
		err := service.CreateSubstitution(ctx, sub)
		require.NoError(t, err)

		// ACT: Try to backdate
		yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
		sub.StartDate = yesterday
		err = service.UpdateSubstitution(ctx, sub)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "past dates")
	})

	t.Run("accepts today's date for update", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "TodayUpdateGroup")
		regularStaff := testpkg.CreateTestStaff(t, db, "TodayUpdateReg", "Staff")
		substituteStaff := testpkg.CreateTestStaff(t, db, "TodayUpdateSub", "Staff")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, regularStaff.ID, substituteStaff.ID)

		today := time.Now().Truncate(24 * time.Hour)
		nextWeek := today.AddDate(0, 0, 7)

		// Create initial substitution starting today
		sub := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    &regularStaff.ID,
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         today,
			EndDate:           nextWeek,
			Reason:            "Emergency coverage",
		}
		err := service.CreateSubstitution(ctx, sub)
		require.NoError(t, err)

		// ACT: Update reason (keep dates the same)
		sub.Reason = "Updated emergency coverage"
		err = service.UpdateSubstitution(ctx, sub)

		// ASSERT
		require.NoError(t, err)

		// Verify the update persisted
		updated, err := service.GetSubstitution(ctx, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated emergency coverage", updated.Reason)
	})
}

// ============================================================================
// TestGroupOperations - Additional group CRUD tests
// ============================================================================

func TestGroupOperations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("create group successfully", func(t *testing.T) {
		// ARRANGE
		group := &educationModels.Group{
			Name: "New Test Group " + time.Now().Format("20060102150405"),
		}

		// ACT
		err := service.CreateGroup(ctx, group)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, group.ID)

		// Cleanup
		defer testpkg.CleanupActivityFixtures(t, db, group.ID)

		// Verify it can be retrieved
		retrieved, err := service.GetGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.Name, retrieved.Name)
	})

	t.Run("assign room to group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "RoomAssignGroup")
		room := testpkg.CreateTestRoom(t, db, "TestRoom")

		defer testpkg.CleanupActivityFixtures(t, db, group.ID, room.ID)

		// ACT
		err := service.AssignRoomToGroup(ctx, group.ID, room.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify room is assigned
		groupWithRoom, err := service.FindGroupWithRoom(ctx, group.ID)
		require.NoError(t, err)
		require.NotNil(t, groupWithRoom.RoomID)
		assert.Equal(t, room.ID, *groupWithRoom.RoomID)
	})

	t.Run("remove room from group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "RoomRemoveGroup")
		room := testpkg.CreateTestRoom(t, db, "RoomToRemove")

		defer testpkg.CleanupActivityFixtures(t, db, group.ID, room.ID)

		// First assign the room
		err := service.AssignRoomToGroup(ctx, group.ID, room.ID)
		require.NoError(t, err)

		// ACT
		err = service.RemoveRoomFromGroup(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify room is removed
		groupWithoutRoom, err := service.GetGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.Nil(t, groupWithoutRoom.RoomID)
	})
}

// ============================================================================
// TestTeacherGroupOperations - Teacher-Group relationship tests
// ============================================================================

func TestTeacherGroupOperations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("add teacher to group via service", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "AddTeacherGroup")
		teacher := testpkg.CreateTestTeacher(t, db, "ServiceAdd", "Teacher")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, teacher.ID, teacher.Staff.ID)

		// ACT
		err := service.AddTeacherToGroup(ctx, group.ID, teacher.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify teacher is in group
		teachers, err := service.GetGroupTeachers(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, teachers, 1)
		assert.Equal(t, teacher.ID, teachers[0].ID)
	})

	t.Run("remove teacher from group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "RemoveTeacherGroup")
		teacher := testpkg.CreateTestTeacher(t, db, "RemoveThis", "Teacher")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, teacher.ID, teacher.Staff.ID)

		// First add the teacher
		err := service.AddTeacherToGroup(ctx, group.ID, teacher.ID)
		require.NoError(t, err)

		// ACT
		err = service.RemoveTeacherFromGroup(ctx, group.ID, teacher.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify teacher is removed
		teachers, err := service.GetGroupTeachers(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, teachers)
	})

	t.Run("prevents duplicate teacher assignment", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "DuplicateTeacherGroup")
		teacher := testpkg.CreateTestTeacher(t, db, "Duplicate", "Teacher")

		defer testpkg.CleanupActivityFixtures(t, db,
			group.ID, teacher.ID, teacher.Staff.ID)

		// First add the teacher
		err := service.AddTeacherToGroup(ctx, group.ID, teacher.ID)
		require.NoError(t, err)

		// ACT: Try to add again
		err = service.AddTeacherToGroup(ctx, group.ID, teacher.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already assigned")
	})

	t.Run("get teacher groups", func(t *testing.T) {
		// ARRANGE
		group1 := testpkg.CreateTestEducationGroup(t, db, "TeacherGroup1")
		group2 := testpkg.CreateTestEducationGroup(t, db, "TeacherGroup2")
		teacher := testpkg.CreateTestTeacher(t, db, "MultiGroup", "Teacher")

		defer testpkg.CleanupActivityFixtures(t, db,
			group1.ID, group2.ID, teacher.ID, teacher.Staff.ID)

		// Add teacher to both groups
		err := service.AddTeacherToGroup(ctx, group1.ID, teacher.ID)
		require.NoError(t, err)
		err = service.AddTeacherToGroup(ctx, group2.ID, teacher.ID)
		require.NoError(t, err)

		// ACT
		groups, err := service.GetTeacherGroups(ctx, teacher.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, groups, 2)

		groupIDs := make(map[int64]bool)
		for _, g := range groups {
			groupIDs[g.ID] = true
		}
		assert.True(t, groupIDs[group1.ID])
		assert.True(t, groupIDs[group2.ID])
	})
}

// ============================================================================
// Additional Coverage Tests
// ============================================================================

func TestEducationService_UpdateGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("updates group successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "OriginalName")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID)

		// Use a unique name to avoid conflicts with other test data
		newName := fmt.Sprintf("UpdatedName-%d", time.Now().UnixNano())
		group.Name = newName

		// ACT
		err := service.UpdateGroup(ctx, group)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, updated.Name)
	})

	t.Run("rejects update with duplicate name", func(t *testing.T) {
		// ARRANGE
		group1 := testpkg.CreateTestEducationGroup(t, db, "ExistingName")
		group2 := testpkg.CreateTestEducationGroup(t, db, "ToBeRenamed")
		defer testpkg.CleanupActivityFixtures(t, db, group1.ID, group2.ID)

		// Use the actual unique name from group1 (fixtures add timestamps)
		group2.Name = group1.Name // Try to rename to existing name

		// ACT
		err := service.UpdateGroup(ctx, group2)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("updates group with room change", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "RoomChangeGroup")
		room := testpkg.CreateTestRoom(t, db, "NewRoom")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, room.ID)

		group.RoomID = &room.ID

		// ACT
		err := service.UpdateGroup(ctx, group)

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ARRANGE
		group := &educationModels.Group{Name: "NonExistent"}
		group.ID = 999999999

		// ACT
		err := service.UpdateGroup(ctx, group)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_DeleteGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("deletes group successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "ToDelete")

		// ACT
		err := service.DeleteGroup(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetGroup(ctx, group.ID)
		require.Error(t, err)
	})

	t.Run("deletes group with teacher relations", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "GroupWithTeacher")
		teacher := testpkg.CreateTestTeacher(t, db, "GroupDelete", "Teacher")
		defer testpkg.CleanupActivityFixtures(t, db, teacher.ID, teacher.Staff.ID)

		_ = service.AddTeacherToGroup(ctx, group.ID, teacher.ID)

		// ACT
		err := service.DeleteGroup(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ACT
		err := service.DeleteGroup(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_GetGroupsByIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("retrieves multiple groups by IDs", func(t *testing.T) {
		// ARRANGE
		group1 := testpkg.CreateTestEducationGroup(t, db, "GroupByID1")
		group2 := testpkg.CreateTestEducationGroup(t, db, "GroupByID2")
		defer testpkg.CleanupActivityFixtures(t, db, group1.ID, group2.ID)

		// ACT
		groups, err := service.GetGroupsByIDs(ctx, []int64{group1.ID, group2.ID})

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, groups, 2)
		assert.NotNil(t, groups[group1.ID])
		assert.NotNil(t, groups[group2.ID])
	})

	t.Run("returns empty map for empty IDs", func(t *testing.T) {
		// ACT
		groups, err := service.GetGroupsByIDs(ctx, []int64{})

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestEducationService_FindGroupByName(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("finds group by name", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "FindByNameTest")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID)

		// ACT - use actual unique name from fixture (fixtures add timestamps)
		found, err := service.FindGroupByName(ctx, group.Name)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, group.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		// ACT
		_, err := service.FindGroupByName(ctx, "NonExistentGroupName12345")

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_FindGroupsByRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("finds groups by room", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "RoomForGroups")
		group := testpkg.CreateTestEducationGroup(t, db, "GroupInRoom")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, room.ID)

		// Assign room to group
		_ = service.AssignRoomToGroup(ctx, group.ID, room.ID)

		// ACT
		groups, err := service.FindGroupsByRoom(ctx, room.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		// ACT
		_, err := service.FindGroupsByRoom(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_UpdateGroupTeachers(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("updates group teachers", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "UpdateTeachersGroup")
		teacher1 := testpkg.CreateTestTeacher(t, db, "Update", "Teacher1")
		teacher2 := testpkg.CreateTestTeacher(t, db, "Update", "Teacher2")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, teacher1.ID, teacher2.ID,
			teacher1.Staff.ID, teacher2.Staff.ID)

		// ACT
		err := service.UpdateGroupTeachers(ctx, group.ID, []int64{teacher1.ID, teacher2.ID})

		// ASSERT
		require.NoError(t, err)

		// Verify teachers were added
		teachers, err := service.GetGroupTeachers(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, teachers, 2)
	})

	t.Run("removes teachers not in new list", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "RemoveTeachersGroup")
		teacher1 := testpkg.CreateTestTeacher(t, db, "Keep", "Teacher")
		teacher2 := testpkg.CreateTestTeacher(t, db, "Remove", "Teacher")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, teacher1.ID, teacher2.ID,
			teacher1.Staff.ID, teacher2.Staff.ID)

		// Add both teachers first
		_ = service.UpdateGroupTeachers(ctx, group.ID, []int64{teacher1.ID, teacher2.ID})

		// ACT - Update with only teacher1
		err := service.UpdateGroupTeachers(ctx, group.ID, []int64{teacher1.ID})

		// ASSERT
		require.NoError(t, err)

		// Verify only teacher1 remains
		teachers, err := service.GetGroupTeachers(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, teachers, 1)
		assert.Equal(t, teacher1.ID, teachers[0].ID)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ACT
		err := service.UpdateGroupTeachers(ctx, 999999999, []int64{})

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_DeleteSubstitution(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("deletes substitution successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "SubDeleteGroup")
		staff := testpkg.CreateTestStaff(t, db, "SubDelete", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, staff.ID)

		today := time.Now().Truncate(24 * time.Hour)
		substitution := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			SubstituteStaffID: staff.ID,
			StartDate:         today,
			EndDate:           today.AddDate(0, 0, 7),
		}
		err := service.CreateSubstitution(ctx, substitution)
		require.NoError(t, err)

		// ACT
		err = service.DeleteSubstitution(ctx, substitution.ID)

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("returns error for non-existent substitution", func(t *testing.T) {
		// ACT
		err := service.DeleteSubstitution(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_WithTx(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("WithTx returns transactional service", func(t *testing.T) {
		// ARRANGE
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// ACT
		txService := service.WithTx(tx)

		// ASSERT
		require.NotNil(t, txService)
		_, ok := txService.(educationSvc.Service)
		assert.True(t, ok, "WithTx should return Service interface")
	})
}

func TestEducationService_GetActiveSubstitutions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("retrieves active substitutions for date", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "ActiveSubGroup")
		staff := testpkg.CreateTestStaff(t, db, "ActiveSub", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, staff.ID)

		today := time.Now().Truncate(24 * time.Hour)
		substitution := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			SubstituteStaffID: staff.ID,
			StartDate:         today,
			EndDate:           today.AddDate(0, 0, 7),
		}
		err := service.CreateSubstitution(ctx, substitution)
		require.NoError(t, err)
		defer func() { _ = service.DeleteSubstitution(ctx, substitution.ID) }()

		// ACT
		subs, err := service.GetActiveSubstitutions(ctx, today)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})
}

func TestEducationService_GetStaffSubstitutions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("retrieves substitutions as substitute", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "StaffSubGroup")
		staff := testpkg.CreateTestStaff(t, db, "StaffSub", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, staff.ID)

		today := time.Now().Truncate(24 * time.Hour)
		substitution := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			SubstituteStaffID: staff.ID,
			StartDate:         today,
			EndDate:           today.AddDate(0, 0, 7),
		}
		err := service.CreateSubstitution(ctx, substitution)
		require.NoError(t, err)
		defer func() { _ = service.DeleteSubstitution(ctx, substitution.ID) }()

		// ACT
		subs, err := service.GetStaffSubstitutions(ctx, staff.ID, false)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})

	t.Run("retrieves substitutions as regular staff", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "RegularStaffSubGroup")
		regularStaff := testpkg.CreateTestStaff(t, db, "Regular", "Staff")
		substituteStaff := testpkg.CreateTestStaff(t, db, "Substitute", "Staff2")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, regularStaff.ID, substituteStaff.ID)

		today := time.Now().Truncate(24 * time.Hour)
		substitution := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			RegularStaffID:    &regularStaff.ID,
			SubstituteStaffID: substituteStaff.ID,
			StartDate:         today,
			EndDate:           today.AddDate(0, 0, 7),
		}
		err := service.CreateSubstitution(ctx, substitution)
		require.NoError(t, err)
		defer func() { _ = service.DeleteSubstitution(ctx, substitution.ID) }()

		// ACT
		subs, err := service.GetStaffSubstitutions(ctx, regularStaff.ID, true)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, subs)
	})

	t.Run("returns error for non-existent staff", func(t *testing.T) {
		// ACT
		_, err := service.GetStaffSubstitutions(ctx, 999999999, false)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_CheckSubstitutionConflicts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("detects no conflicts for available period", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "ConflictCheck", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

		future := time.Now().AddDate(1, 0, 0).Truncate(24 * time.Hour)

		// ACT
		conflicts, err := service.CheckSubstitutionConflicts(ctx, staff.ID, future, future.AddDate(0, 0, 7))

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, conflicts)
	})

	t.Run("returns error for non-existent staff", func(t *testing.T) {
		// ARRANGE
		future := time.Now().AddDate(1, 0, 0).Truncate(24 * time.Hour)

		// ACT
		_, err := service.CheckSubstitutionConflicts(ctx, 999999999, future, future.AddDate(0, 0, 7))

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid date range", func(t *testing.T) {
		// ARRANGE
		staff := testpkg.CreateTestStaff(t, db, "InvalidRange", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

		future := time.Now().AddDate(1, 0, 0).Truncate(24 * time.Hour)

		// ACT - end date before start date
		_, err := service.CheckSubstitutionConflicts(ctx, staff.ID, future, future.AddDate(0, 0, -7))

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_CreateGroup_EdgeCases(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("rejects group with invalid name", func(t *testing.T) {
		// ARRANGE
		group := &educationModels.Group{Name: ""} // Empty name is invalid

		// ACT
		err := service.CreateGroup(ctx, group)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects group with non-existent room", func(t *testing.T) {
		// ARRANGE
		nonExistentRoomID := int64(999999999)
		uniqueName := fmt.Sprintf("GroupWithBadRoom-%d", time.Now().UnixNano())
		group := &educationModels.Group{
			Name:   uniqueName,
			RoomID: &nonExistentRoomID,
		}

		// ACT
		err := service.CreateGroup(ctx, group)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "room")
	})

	t.Run("creates group with existing room", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "GroupCreateRoom")
		uniqueName := fmt.Sprintf("GroupWithRoom-%d", time.Now().UnixNano())
		group := &educationModels.Group{
			Name:   uniqueName,
			RoomID: &room.ID,
		}
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		err := service.CreateGroup(ctx, group)

		// ASSERT
		require.NoError(t, err)
		defer func() { _ = service.DeleteGroup(ctx, group.ID) }()
		assert.NotZero(t, group.ID)
	})

	t.Run("rejects duplicate group name", func(t *testing.T) {
		// ARRANGE
		existingGroup := testpkg.CreateTestEducationGroup(t, db, "DuplicateTest")
		defer testpkg.CleanupActivityFixtures(t, db, existingGroup.ID)

		duplicateGroup := &educationModels.Group{Name: existingGroup.Name}

		// ACT
		err := service.CreateGroup(ctx, duplicateGroup)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestEducationService_ListGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("lists groups with options", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "ListTestGroup")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID)

		options := base.NewQueryOptions()
		options.WithPagination(1, 10)

		// ACT
		groups, err := service.ListGroups(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})
}

func TestEducationService_FindGroupWithRoom(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("finds group with room", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "FindGroupRoom")
		group := testpkg.CreateTestEducationGroup(t, db, "FindGroupWithRoom")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, room.ID)

		_ = service.AssignRoomToGroup(ctx, group.ID, room.ID)

		// ACT
		found, err := service.FindGroupWithRoom(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, group.ID, found.ID)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ACT
		_, err := service.FindGroupWithRoom(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_AssignRoomToGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "AssignRoomTest")
		defer testpkg.CleanupActivityFixtures(t, db, room.ID)

		// ACT
		err := service.AssignRoomToGroup(ctx, 999999999, room.ID)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent room", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "AssignRoomGroup")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID)

		// ACT
		err := service.AssignRoomToGroup(ctx, group.ID, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_RemoveRoomFromGroup(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ACT
		err := service.RemoveRoomFromGroup(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_GetTeacherGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("returns error for non-existent teacher", func(t *testing.T) {
		// ACT
		_, err := service.GetTeacherGroups(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_GetSubstitution(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("retrieves substitution by ID", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "GetSubGroup")
		staff := testpkg.CreateTestStaff(t, db, "GetSub", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, group.ID, staff.ID)

		today := time.Now().Truncate(24 * time.Hour)
		substitution := &educationModels.GroupSubstitution{
			GroupID:           group.ID,
			SubstituteStaffID: staff.ID,
			StartDate:         today,
			EndDate:           today.AddDate(0, 0, 7),
		}
		err := service.CreateSubstitution(ctx, substitution)
		require.NoError(t, err)
		defer func() { _ = service.DeleteSubstitution(ctx, substitution.ID) }()

		// ACT
		found, err := service.GetSubstitution(ctx, substitution.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, substitution.ID, found.ID)
	})

	t.Run("returns error for non-existent substitution", func(t *testing.T) {
		// ACT
		_, err := service.GetSubstitution(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_ListSubstitutions(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupEducationService(t, db)
	ctx := context.Background()

	t.Run("lists substitutions with options", func(t *testing.T) {
		// ARRANGE
		options := base.NewQueryOptions()
		options.WithPagination(1, 10)

		// ACT
		subs, err := service.ListSubstitutions(ctx, options)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, subs)
	})
}

func TestEducationError_Unwrap(t *testing.T) {
	t.Run("unwraps inner error", func(t *testing.T) {
		// ARRANGE
		innerErr := educationSvc.ErrGroupNotFound
		err := &educationSvc.EducationError{Op: "TestOp", Err: innerErr}

		// ACT
		unwrapped := err.Unwrap()

		// ASSERT
		assert.Equal(t, innerErr, unwrapped)
	})
}
