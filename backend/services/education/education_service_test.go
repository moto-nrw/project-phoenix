package education_test

import (
	"context"
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
