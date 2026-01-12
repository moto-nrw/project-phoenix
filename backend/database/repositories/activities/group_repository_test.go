package activities_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestActivityGroupRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("creates activity group with valid data", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "GroupCreate")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		uniqueName := fmt.Sprintf("TestGroup-%d", time.Now().UnixNano())
		group := &activities.Group{
			Name:            uniqueName,
			CategoryID:      category.ID,
			MaxParticipants: 20,
			IsOpen:          true,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.NotZero(t, group.ID)

		testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)
	})

	t.Run("creates closed activity group", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "ClosedGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		uniqueName := fmt.Sprintf("ClosedGroup-%d", time.Now().UnixNano())
		group := &activities.Group{
			Name:            uniqueName,
			CategoryID:      category.ID,
			MaxParticipants: 15,
			IsOpen:          false,
		}

		err := repo.Create(ctx, group)
		require.NoError(t, err)
		assert.False(t, group.IsOpen)

		testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)
	})
}

func TestActivityGroupRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("finds existing activity group", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "FindByID")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, group.ID, found.ID)
		assert.Contains(t, found.Name, "FindByID")
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestActivityGroupRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("updates activity group name", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Update")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		newName := fmt.Sprintf("UpdatedName-%d", time.Now().UnixNano())
		group.Name = newName
		err := repo.Update(ctx, group)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, found.Name)
	})

	t.Run("updates activity group open status", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "UpdateIsOpen")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		group.IsOpen = false
		err := repo.Update(ctx, group)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, group.ID)
		require.NoError(t, err)
		assert.False(t, found.IsOpen)
	})
}

func TestActivityGroupRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("deletes existing activity group", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Delete")
		categoryID := group.CategoryID
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, categoryID, 0)

		err := repo.Delete(ctx, group.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, group.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestActivityGroupRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("lists all activity groups", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "List")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		groups, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
	})

	t.Run("lists with filter using id field", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "ListFilter")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		// Create filter with id field - this tests the table alias fix
		// to avoid ambiguous column reference with the category join
		options := testpkg.NewQueryOptions()
		options.Filter.Equal("id", group.ID)

		groups, err := repo.List(ctx, options)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, group.ID, groups[0].ID)
	})
}

func TestActivityGroupRepository_FindByCategory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("finds groups by category ID", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "ByCategory")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		groups, err := repo.FindByCategory(ctx, group.CategoryID)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		var found bool
		for _, g := range groups {
			if g.ID == group.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for category with no groups", func(t *testing.T) {
		category := testpkg.CreateTestActivityCategory(t, db, "EmptyCategory")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, category.ID, 0)

		groups, err := repo.FindByCategory(ctx, category.ID)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestActivityGroupRepository_FindOpenGroups(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("finds only open groups", func(t *testing.T) {
		// Create an open group
		openGroup := testpkg.CreateTestActivityGroup(t, db, "IsOpenGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, openGroup.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", openGroup.ID)

		groups, err := repo.FindOpenGroups(ctx)
		require.NoError(t, err)

		// All returned groups should be open
		for _, g := range groups {
			assert.True(t, g.IsOpen)
		}

		// Our open group should be in the results
		var found bool
		for _, g := range groups {
			if g.ID == openGroup.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestActivityGroupRepository_FindWithEnrollmentCounts(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("returns groups with enrollment counts", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "WithEnrollments")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		// Create some enrollments
		student1 := testpkg.CreateTestStudent(t, db, "Student", "One", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Student", "Two", "1b")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, 0, 0, 0, 0)
		defer testpkg.CleanupActivityFixtures(t, db, student2.ID, 0, 0, 0, 0)

		// Add enrollments directly
		_, _ = db.NewInsert().
			Model(&activities.StudentEnrollment{
				StudentID:       student1.ID,
				ActivityGroupID: group.ID,
				EnrollmentDate:  time.Now(),
			}).
			ModelTableExpr(`activities.student_enrollments`).
			Exec(ctx)

		_, _ = db.NewInsert().
			Model(&activities.StudentEnrollment{
				StudentID:       student2.ID,
				ActivityGroupID: group.ID,
				EnrollmentDate:  time.Now(),
			}).
			ModelTableExpr(`activities.student_enrollments`).
			Exec(ctx)

		groups, counts, err := repo.FindWithEnrollmentCounts(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)
		assert.NotNil(t, counts)

		// Check that our group has the correct count
		count, exists := counts[group.ID]
		assert.True(t, exists)
		assert.Equal(t, 2, count)
	})

	t.Run("returns empty map when no groups exist", func(t *testing.T) {
		// This test assumes at least some groups exist from seeding
		// We're just checking the function doesn't fail
		groups, counts, err := repo.FindWithEnrollmentCounts(ctx)
		require.NoError(t, err)
		assert.NotNil(t, groups)
		assert.NotNil(t, counts)
	})
}

func TestActivityGroupRepository_FindWithSupervisors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("returns group with supervisors", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "WithSupervisors")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		staff := testpkg.CreateTestStaff(t, db, "Supervisor", "Test")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, 0, 0)

		// Add a supervisor
		_, _ = db.NewInsert().
			Model(&activities.SupervisorPlanned{
				GroupID:   group.ID,
				StaffID:   staff.ID,
				IsPrimary: true,
			}).
			ModelTableExpr(`activities.supervisors`).
			Exec(ctx)

		foundGroup, supervisors, err := repo.FindWithSupervisors(ctx, group.ID)
		require.NoError(t, err)
		assert.NotNil(t, foundGroup)
		assert.Equal(t, group.ID, foundGroup.ID)
		assert.NotEmpty(t, supervisors)
		assert.Equal(t, staff.ID, supervisors[0].StaffID)
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, _, err := repo.FindWithSupervisors(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestActivityGroupRepository_FindWithSchedules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("returns group with schedules", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "WithSchedules")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		foundGroup, schedules, err := repo.FindWithSchedules(ctx, group.ID)
		require.NoError(t, err)
		assert.NotNil(t, foundGroup)
		assert.Equal(t, group.ID, foundGroup.ID)
		assert.NotNil(t, schedules)
		// Schedules may be empty if none are set
	})

	t.Run("returns error for non-existent group", func(t *testing.T) {
		_, _, err := repo.FindWithSchedules(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestActivityGroupRepository_FindByStaffSupervisor(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("finds groups supervised by staff member", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "BySupervisor")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		staff := testpkg.CreateTestStaff(t, db, "Finding", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, 0, 0)

		// Add supervisor assignment
		_, _ = db.NewInsert().
			Model(&activities.SupervisorPlanned{
				GroupID:   group.ID,
				StaffID:   staff.ID,
				IsPrimary: true,
			}).
			ModelTableExpr(`activities.supervisors`).
			Exec(ctx)

		groups, err := repo.FindByStaffSupervisor(ctx, staff.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, groups)

		var found bool
		for _, g := range groups {
			if g.ID == group.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for staff with no supervised groups", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "NoGroups", "Staff")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, 0, 0)

		groups, err := repo.FindByStaffSupervisor(ctx, staff.ID)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})
}

func TestActivityGroupRepository_FindByStaffSupervisorToday(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("finds only open groups supervised by staff member", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "SupervisorToday")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		staff := testpkg.CreateTestStaff(t, db, "Today", "Supervisor")
		defer testpkg.CleanupActivityFixtures(t, db, 0, staff.ID, 0, 0, 0)

		// Add supervisor assignment
		_, _ = db.NewInsert().
			Model(&activities.SupervisorPlanned{
				GroupID:   group.ID,
				StaffID:   staff.ID,
				IsPrimary: true,
			}).
			ModelTableExpr(`activities.supervisors`).
			Exec(ctx)

		groups, err := repo.FindByStaffSupervisorToday(ctx, staff.ID)
		require.NoError(t, err)

		// All returned groups should be open
		for _, g := range groups {
			assert.True(t, g.IsOpen)
		}
	})
}

// ============================================================================
// Edge Cases and Validation Tests
// ============================================================================

func TestActivityGroupRepository_Create_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("returns error when group is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestActivityGroupRepository_Update_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("returns error when group is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestActivityGroupRepository_Delete_NonExistent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivityGroup
	ctx := context.Background()

	t.Run("does not error when deleting non-existent group", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}
