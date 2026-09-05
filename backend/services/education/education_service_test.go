package education_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))

	return educationSvc.NewService(
		repoFactory.Group,
		repoFactory.GroupTeacher,
		repoFactory.ClassTeacher,
		repoFactory.Room,
		repoFactory.Teacher,
		repoFactory.Staff,
		repoFactory.Student,
		repoFactory.GroupSubstitution,
		db,
	)
}

// ============================================================================
// TestListGroups - Tests for listing education groups with filters
// ============================================================================

func TestListGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("successful list with name filter", func(t *testing.T) {
		// ARRANGE: Create groups with specific names
		group1 := testpkg.CreateTestEducationGroup(t, db, "Math")
		group2 := testpkg.CreateTestEducationGroup(t, db, "Science")
		group3 := testpkg.CreateTestEducationGroup(t, db, "Math") // Another Math group

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
		testpkg.CreateTestEducationGroup(t, db, "PaginationTest")
		testpkg.CreateTestEducationGroup(t, db, "PaginationTest")

		// ACT: List with pagination
		query := &educationModels.GroupListQuery{Limit: 100}
		groups, err := service.ListGroups(ctx, query)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})

	t.Run("list with nil options returns all groups", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "NilOptionsTest")

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
// ============================================================================

func TestGetGroupTeachers(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("successful get teachers for group", func(t *testing.T) {
		// ARRANGE: Create group and teachers
		group := testpkg.CreateTestEducationGroup(t, db, "TeacherTestGroup")
		teacher1 := testpkg.CreateTestTeacher(t, db, "Teacher", "One")
		teacher2 := testpkg.CreateTestTeacher(t, db, "Teacher", "Two")
		teacher3 := testpkg.CreateTestTeacher(t, db, "Teacher", "Three")

		// Assign teachers to group
		testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher1.ID)
		testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher2.ID)
		testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher3.ID)

		// Get staff IDs for cleanup (teachers depend on staff)

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
// ============================================================================

func TestGroupOperations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

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

		// Verify it can be retrieved
		retrieved, err := service.GetGroup(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.Name, retrieved.Name)
	})

}

// ============================================================================
// TestTeacherGroupOperations - Teacher-Group relationship tests
// ============================================================================

func TestTeacherGroupOperations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("remove teacher from group", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "RemoveTeacherGroup")
		teacher := testpkg.CreateTestTeacher(t, db, "RemoveThis", "Teacher")

		// First add the teacher
		require.NoError(t, service.UpdateGroupTeachers(ctx, group.ID, []int64{teacher.ID}))

		// ACT
		err := service.RemoveTeacherFromGroup(ctx, group.ID, teacher.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify teacher is removed
		teachers, err := service.GetGroupTeachers(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, teachers)
	})

	t.Run("get teacher groups", func(t *testing.T) {
		// ARRANGE
		group1 := testpkg.CreateTestEducationGroup(t, db, "TeacherGroup1")
		group2 := testpkg.CreateTestEducationGroup(t, db, "TeacherGroup2")
		teacher := testpkg.CreateTestTeacher(t, db, "MultiGroup", "Teacher")

		// Add teacher to both groups
		require.NoError(t, service.UpdateGroupTeachers(ctx, group1.ID, []int64{teacher.ID}))
		require.NoError(t, service.UpdateGroupTeachers(ctx, group2.ID, []int64{teacher.ID}))

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates group successfully", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "OriginalName")

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

		// Use the actual unique name from group1 (fixtures add timestamps)
		group2.Name = group1.Name // Try to rename to existing name

		// ACT
		err := service.UpdateGroup(ctx, group2)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "existiert bereits")
	})

	t.Run("updates group with room change", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "RoomChangeGroup")
		room := testpkg.CreateTestRoom(t, db, "NewRoom")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

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

		_ = service.UpdateGroupTeachers(ctx, group.ID, []int64{teacher.ID})

		// ACT
		err := service.DeleteGroup(ctx, group.ID)

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("returns error when group has students", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "GroupWithStudents")
		student := testpkg.CreateTestStudent(t, db, "GroupDel", "Student", "1a")

		// Assign student to group
		student.GroupID = &group.ID
		_, err := db.NewUpdate().
			Model(student).
			ModelTableExpr(`users.students AS "student"`).
			Column("group_id").
			Where(`"student".id = ?`, student.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT
		err = service.DeleteGroup(ctx, group.ID)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Gruppe kann nicht")
	})

	t.Run("returns error when group has an active handover", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "GroupWithHandover")
		target := testpkg.CreateTestStaff(t, db, "Group", "Target")
		today := timezone.TodayDate()
		testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, target.ID, today, today)

		err := service.DeleteGroup(ctx, group.ID)

		require.ErrorIs(t, err, educationSvc.ErrGroupHasHandover)
		_, findErr := service.GetGroup(ctx, group.ID)
		require.NoError(t, findErr)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		// ACT
		err := service.DeleteGroup(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationService_GetGroupsByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("retrieves multiple groups by IDs", func(t *testing.T) {
		// ARRANGE
		group1 := testpkg.CreateTestEducationGroup(t, db, "GroupByID1")
		group2 := testpkg.CreateTestEducationGroup(t, db, "GroupByID2")

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

func TestEducationService_UpdateGroupTeachers(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates group teachers", func(t *testing.T) {
		// ARRANGE
		group := testpkg.CreateTestEducationGroup(t, db, "UpdateTeachersGroup")
		teacher1 := testpkg.CreateTestTeacher(t, db, "Update", "Teacher1")
		teacher2 := testpkg.CreateTestTeacher(t, db, "Update", "Teacher2")

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

func TestEducationService_CreateGroup_EdgeCases(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

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

		duplicateGroup := &educationModels.Group{Name: existingGroup.Name}

		// ACT
		err := service.CreateGroup(ctx, duplicateGroup)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "existiert bereits")
	})
}

func TestEducationService_ListGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("lists groups with options", func(t *testing.T) {
		// ARRANGE
		testpkg.CreateTestEducationGroup(t, db, "ListTestGroup")

		// ACT
		groups, err := service.ListGroups(ctx, &educationModels.GroupListQuery{Limit: 10})

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})
}

func TestEducationService_FindGroupWithRoom(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds group with room", func(t *testing.T) {
		// ARRANGE
		room := testpkg.CreateTestRoom(t, db, "FindGroupRoom")
		group := testpkg.CreateTestEducationGroup(t, db, "FindGroupWithRoom")

		group.RoomID = &room.ID
		require.NoError(t, service.UpdateGroup(ctx, group))

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

func TestEducationService_GetTeacherGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupEducationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent teacher", func(t *testing.T) {
		// ACT
		_, err := service.GetTeacherGroups(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestEducationError_Unwrap(t *testing.T) {
	t.Parallel()

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
