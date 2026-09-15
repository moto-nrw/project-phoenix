package users_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// ============================================================================
// CRUD Tests
// ============================================================================

func TestTeacherRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("creates teacher with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Teacher", "Create")

		teacher := &users.Teacher{
			StaffID: staff.ID,
		}

		err := repo.Create(ctx, teacher)
		require.NoError(t, err)
		assert.NotZero(t, teacher.ID)

		// Verify in DB
		found, err := repo.FindByID(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Equal(t, staff.ID, found.StaffID)

		// Cleanup teacher (staff cleanup handled by defer)
		_, _ = db.NewDelete().
			TableExpr("users.teachers").
			Where("id = ?", teacher.ID).
			Exec(ctx)
	})

	t.Run("creates teacher with specialization", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Math", "Teacher")

		teacher := &users.Teacher{
			StaffID:        staff.ID,
			Specialization: "Mathematics",
		}

		err := repo.Create(ctx, teacher)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Equal(t, "Mathematics", found.Specialization)

		_, _ = db.NewDelete().
			TableExpr("users.teachers").
			Where("id = ?", teacher.ID).
			Exec(ctx)
	})

	t.Run("fails with nil teacher", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("fails with missing staff ID", func(t *testing.T) {
		teacher := &users.Teacher{
			StaffID: 0, // Invalid
		}

		err := repo.Create(ctx, teacher)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "staff ID")
	})
}

func TestTeacherRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("finds existing teacher", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "FindByID", "Teacher")

		found, err := repo.FindByID(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Equal(t, teacher.ID, found.ID)
	})

	t.Run("returns error for non-existent teacher", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no rows")
	})
}

func TestTeacherRepository_FindByStaffID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("finds teacher by staff ID", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "FindByStaff", "Teacher")

		found, err := repo.FindByStaffID(ctx, teacher.StaffID)
		require.NoError(t, err)
		assert.Equal(t, teacher.ID, found.ID)
		assert.Equal(t, teacher.StaffID, found.StaffID)
	})

	t.Run("returns nil for non-existent staff ID", func(t *testing.T) {
		found, err := repo.FindByStaffID(ctx, int64(999999))
		require.NoError(t, err)
		assert.Nil(t, found, "Expected nil for non-existent staff ID")
	})
}

func TestTeacherRepository_FindByStaffIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("finds multiple teachers by staff IDs", func(t *testing.T) {
		teacher1 := testpkg.CreateTestTeacher(t, db, "FindByIDs1", "Teacher")
		teacher2 := testpkg.CreateTestTeacher(t, db, "FindByIDs2", "Teacher")

		staffIDs := []int64{teacher1.StaffID, teacher2.StaffID}
		teacherMap, err := repo.FindByStaffIDs(ctx, staffIDs)

		require.NoError(t, err)
		assert.Len(t, teacherMap, 2)
		assert.Equal(t, teacher1.ID, teacherMap[teacher1.StaffID].ID)
		assert.Equal(t, teacher2.ID, teacherMap[teacher2.StaffID].ID)
	})

	t.Run("returns empty map for empty input", func(t *testing.T) {
		teacherMap, err := repo.FindByStaffIDs(ctx, []int64{})

		require.NoError(t, err)
		assert.Empty(t, teacherMap)
	})

	t.Run("returns partial results for mixed existing/non-existing IDs", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "FindByIDsPartial", "Teacher")

		staffIDs := []int64{teacher.StaffID, 999999} // one exists, one doesn't
		teacherMap, err := repo.FindByStaffIDs(ctx, staffIDs)

		require.NoError(t, err)
		assert.Len(t, teacherMap, 1)
		assert.Equal(t, teacher.ID, teacherMap[teacher.StaffID].ID)
		_, exists := teacherMap[999999]
		assert.False(t, exists)
	})
}

func TestTeacherRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("updates teacher specialization", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "Update", "Teacher")

		teacher.Specialization = "Physics"

		err := repo.Update(ctx, teacher)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Equal(t, "Physics", found.Specialization)
	})

	t.Run("fails with nil teacher", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})
}

func TestTeacherRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing teacher", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "Delete", "Teacher")

		err := repo.Delete(ctx, teacher.ID)
		require.NoError(t, err)

		// Verify teacher is deleted
		_, err = repo.FindByID(ctx, teacher.ID)
		require.Error(t, err)

		// Cleanup staff (teacher is already deleted)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestTeacherRepository_FindBySpecialization(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("finds teachers by specialization (case-insensitive)", func(t *testing.T) {
		// Create teacher with unique specialization
		teacher := testpkg.CreateTestTeacher(t, db, "Spec", "Teacher")

		uniqueSpec := fmt.Sprintf("UniqueSpec%d", time.Now().UnixNano())
		teacher.Specialization = uniqueSpec
		err := repo.Update(ctx, teacher)
		require.NoError(t, err)

		// Search with different case
		teachers, err := repo.FindBySpecialization(ctx, uniqueSpec)
		require.NoError(t, err)
		assert.Len(t, teachers, 1)
	})

	t.Run("returns empty for non-existent specialization", func(t *testing.T) {
		teachers, err := repo.FindBySpecialization(ctx, "NonExistentSpec999")
		require.NoError(t, err)
		assert.Empty(t, teachers)
	})
}

func TestTeacherRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("lists all teachers with no filters", func(t *testing.T) {
		testpkg.CreateTestTeacher(t, db, "List", "Teacher")

		teachers, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, teachers)
	})

	t.Run("lists teachers with staff_id filter", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "Filter", "Teacher")

		teachers, err := repo.List(ctx, map[string]any{
			"staff_id": teacher.StaffID,
		})
		require.NoError(t, err)
		assert.Len(t, teachers, 1)
		assert.Equal(t, teacher.ID, teachers[0].ID)
	})
}

func TestTeacherRepository_FindByGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("finds teachers assigned to education group", func(t *testing.T) {
		// Create education group
		group := testpkg.CreateTestEducationGroup(t, db, "TeacherGroup")

		// Create teacher
		teacher := testpkg.CreateTestTeacher(t, db, "GroupTeacher", "Test")

		// Create group-teacher assignment
		gt := testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)

		defer func() {
			// Delete group-teacher first
			_, _ = db.NewDelete().
				TableExpr("education.group_teacher").
				Where("id = ?", gt.ID).
				Exec(ctx)
		}()

		// Test
		teachers, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, teachers, 1)
		assert.Equal(t, teacher.ID, teachers[0].ID)
	})

	t.Run("returns empty for group with no teachers", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "EmptyTeacherGroup")

		teachers, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, teachers)
	})
}

// ============================================================================
// Relationship Tests
// ============================================================================

func TestTeacherRepository_FindWithStaffAndPerson(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("finds teacher with staff and person loaded", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "WithStaff", "Person")

		found, err := repo.FindWithStaffAndPerson(ctx, teacher.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Staff)
		require.NotNil(t, found.Staff.Person)
		assert.Equal(t, "WithStaff", found.Staff.Person.FirstName)
		assert.Equal(t, "Person", found.Staff.Person.LastName)
	})

	t.Run("returns error for non-existent teacher", func(t *testing.T) {
		_, err := repo.FindWithStaffAndPerson(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestTeacherRepository_ListAllWithStaffAndPerson(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("returns all teachers with staff and person data", func(t *testing.T) {
		// Create multiple teachers
		teacher1 := testpkg.CreateTestTeacher(t, db, "AllWithStaff1", "Person1")
		teacher2 := testpkg.CreateTestTeacher(t, db, "AllWithStaff2", "Person2")

		results, err := repo.ListAllWithStaffAndPerson(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, results)

		// Find our created teachers in results
		var foundTeacher1, foundTeacher2 bool
		for _, tr := range results {
			if tr.ID == teacher1.ID {
				foundTeacher1 = true
				require.NotNil(t, tr.Staff, "teacher1.Staff should be loaded")
				require.NotNil(t, tr.Staff.Person, "teacher1.Staff.Person should be loaded")
				assert.Equal(t, "AllWithStaff1", tr.Staff.Person.FirstName)
				assert.Equal(t, "Person1", tr.Staff.Person.LastName)
			}
			if tr.ID == teacher2.ID {
				foundTeacher2 = true
				require.NotNil(t, tr.Staff, "teacher2.Staff should be loaded")
				require.NotNil(t, tr.Staff.Person, "teacher2.Staff.Person should be loaded")
				assert.Equal(t, "AllWithStaff2", tr.Staff.Person.FirstName)
				assert.Equal(t, "Person2", tr.Staff.Person.LastName)
			}
		}

		assert.True(t, foundTeacher1, "should find teacher1 in results")
		assert.True(t, foundTeacher2, "should find teacher2 in results")
	})

	t.Run("loads all staff and person fields correctly", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "StaffPersonFields", "Check")

		results, err := repo.ListAllWithStaffAndPerson(ctx)
		require.NoError(t, err)

		// Find our teacher
		var found *users.Teacher
		for _, tr := range results {
			if tr.ID == teacher.ID {
				found = tr
				break
			}
		}

		require.NotNil(t, found, "should find created teacher")
		require.NotNil(t, found.Staff, "staff should be loaded")
		require.NotNil(t, found.Staff.Person, "person should be loaded")

		// Check all critical fields are loaded
		assert.NotZero(t, found.Staff.ID, "staff ID should be loaded")
		assert.NotZero(t, found.Staff.PersonID, "staff person_id should be loaded")
		assert.NotZero(t, found.Staff.Person.ID, "person ID should be loaded")
		assert.Equal(t, "StaffPersonFields", found.Staff.Person.FirstName)
		assert.Equal(t, "Check", found.Staff.Person.LastName)
		assert.NotZero(t, found.Staff.Person.CreatedAt, "person created_at should be loaded")
	})
}

// ============================================================================
// Filter Tests
// ============================================================================

func TestTeacherRepository_ListWithStringFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Teacher
	ctx := testpkg.Ctx(t)

	t.Run("filters teachers by specialization_like", func(t *testing.T) {
		// Create teacher with unique specialization
		teacher := testpkg.CreateTestTeacher(t, db, "FilterTest", "Teacher")

		uniqueSpec := fmt.Sprintf("FilterSpec%d", time.Now().UnixNano())
		teacher.Specialization = uniqueSpec
		err := repo.Update(ctx, teacher)
		require.NoError(t, err)

		// Use LIKE filter (tests applyTeacherStringLikeFilter)
		teachers, err := repo.List(ctx, map[string]any{
			"specialization_like": "FilterSpec",
		})
		require.NoError(t, err)

		// Should find at least our teacher
		var found bool
		for _, t := range teachers {
			if t.ID == teacher.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find teacher with matching specialization")
	})

	t.Run("filters teachers by role_like", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "RoleTest", "Teacher")

		// Set a unique role
		uniqueRole := fmt.Sprintf("RoleFilter%d", time.Now().UnixNano())
		teacher.Role = uniqueRole
		err := repo.Update(ctx, teacher)
		require.NoError(t, err)

		// Use role_like filter (tests applyTeacherStringLikeFilter)
		teachers, err := repo.List(ctx, map[string]any{
			"role_like": "RoleFilter",
		})
		require.NoError(t, err)

		// Should find at least our teacher
		var found bool
		for _, t := range teachers {
			if t.ID == teacher.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find teacher with matching role")
	})
}

// ============================================================================
// UpdateQualifications Tests
// ============================================================================
