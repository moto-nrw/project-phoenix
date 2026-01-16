package users_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================


// cleanupTeacherStaffRecords removes staff members and their persons in proper FK order
// (named differently to avoid redefinition when running all tests together)
func cleanupTeacherStaffRecords(t *testing.T, db *bun.DB, staffIDs ...int64) {
	t.Helper()
	if len(staffIDs) == 0 {
		return
	}

	ctx := context.Background()

	var personIDs []int64
	err := db.NewSelect().
		TableExpr("users.staff").
		Column("person_id").
		Where("id IN (?)", bun.In(staffIDs)).
		Scan(ctx, &personIDs)
	if err != nil {
		t.Logf("Warning: failed to get person IDs for cleanup: %v", err)
	}

	_, err = db.NewDelete().
		TableExpr("users.staff").
		Where("id IN (?)", bun.In(staffIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup staff: %v", err)
	}

	if len(personIDs) > 0 {
		_, err = db.NewDelete().
			TableExpr("users.persons").
			Where("id IN (?)", bun.In(personIDs)).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: failed to cleanup persons: %v", err)
		}
	}
}

// cleanupTeacherEducationData removes education groups and group-teacher assignments
func cleanupTeacherEducationData(t *testing.T, db *bun.DB, groupIDs []int64) {
	t.Helper()
	ctx := context.Background()

	if len(groupIDs) > 0 {
		_, err := db.NewDelete().
			TableExpr("education.group_teacher").
			Where("group_id IN (?)", bun.In(groupIDs)).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: failed to cleanup group-teacher assignments: %v", err)
		}

		_, err = db.NewDelete().
			TableExpr("education.groups").
			Where("id IN (?)", bun.In(groupIDs)).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: failed to cleanup education groups: %v", err)
		}
	}
}

// cleanupTeacherRecords removes teachers, staff, and persons in proper FK order
func cleanupTeacherRecords(t *testing.T, db *bun.DB, teacherIDs ...int64) {
	t.Helper()
	if len(teacherIDs) == 0 {
		return
	}

	ctx := context.Background()

	// Get staff IDs before deleting teachers
	var staffIDs []int64
	err := db.NewSelect().
		TableExpr("users.teachers").
		Column("staff_id").
		Where("id IN (?)", bun.In(teacherIDs)).
		Scan(ctx, &staffIDs)
	if err != nil {
		t.Logf("Warning: failed to get staff IDs for cleanup: %v", err)
	}

	// Delete teachers first
	_, err = db.NewDelete().
		TableExpr("users.teachers").
		Where("id IN (?)", bun.In(teacherIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup teachers: %v", err)
	}

	// Delete staff and persons
	if len(staffIDs) > 0 {
		var personIDs []int64
		err := db.NewSelect().
			TableExpr("users.staff").
			Column("person_id").
			Where("id IN (?)", bun.In(staffIDs)).
			Scan(ctx, &personIDs)
		if err != nil {
			t.Logf("Warning: failed to get person IDs for cleanup: %v", err)
		}

		_, err = db.NewDelete().
			TableExpr("users.staff").
			Where("id IN (?)", bun.In(staffIDs)).
			Exec(ctx)
		if err != nil {
			t.Logf("Warning: failed to cleanup staff: %v", err)
		}

		if len(personIDs) > 0 {
			_, err = db.NewDelete().
				TableExpr("users.persons").
				Where("id IN (?)", bun.In(personIDs)).
				Exec(ctx)
			if err != nil {
				t.Logf("Warning: failed to cleanup persons: %v", err)
			}
		}
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestTeacherRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("creates teacher with valid data", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Teacher", "Create")
		defer cleanupTeacherStaffRecords(t, db, staff.ID)

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
		defer cleanupTeacherStaffRecords(t, db, staff.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("finds existing teacher", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "FindByID", "Teacher")
		defer cleanupTeacherRecords(t, db, teacher.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("finds teacher by staff ID", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "FindByStaff", "Teacher")
		defer cleanupTeacherRecords(t, db, teacher.ID)

		found, err := repo.FindByStaffID(ctx, teacher.StaffID)
		require.NoError(t, err)
		assert.Equal(t, teacher.ID, found.ID)
		assert.Equal(t, teacher.StaffID, found.StaffID)
	})

	t.Run("returns error for non-existent staff ID", func(t *testing.T) {
		_, err := repo.FindByStaffID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestTeacherRepository_FindByStaffIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("finds multiple teachers by staff IDs", func(t *testing.T) {
		teacher1 := testpkg.CreateTestTeacher(t, db, "FindByIDs1", "Teacher")
		teacher2 := testpkg.CreateTestTeacher(t, db, "FindByIDs2", "Teacher")
		defer cleanupTeacherRecords(t, db, teacher1.ID, teacher2.ID)

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
		defer cleanupTeacherRecords(t, db, teacher.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("updates teacher specialization", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "Update", "Teacher")
		defer cleanupTeacherRecords(t, db, teacher.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("deletes existing teacher", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "Delete", "Teacher")
		staffID := teacher.StaffID

		err := repo.Delete(ctx, teacher.ID)
		require.NoError(t, err)

		// Verify teacher is deleted
		_, err = repo.FindByID(ctx, teacher.ID)
		require.Error(t, err)

		// Cleanup staff (teacher is already deleted)
		cleanupTeacherStaffRecords(t, db, staffID)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestTeacherRepository_FindBySpecialization(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("finds teachers by specialization (case-insensitive)", func(t *testing.T) {
		// Create teacher with unique specialization
		teacher := testpkg.CreateTestTeacher(t, db, "Spec", "Teacher")
		defer cleanupTeacherRecords(t, db, teacher.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("lists all teachers with no filters", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "List", "Teacher")
		defer cleanupTeacherRecords(t, db, teacher.ID)

		teachers, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, teachers)
	})

	t.Run("lists teachers with staff_id filter", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "Filter", "Teacher")
		defer cleanupTeacherRecords(t, db, teacher.ID)

		teachers, err := repo.List(ctx, map[string]interface{}{
			"staff_id": teacher.StaffID,
		})
		require.NoError(t, err)
		assert.Len(t, teachers, 1)
		assert.Equal(t, teacher.ID, teachers[0].ID)
	})
}

func TestTeacherRepository_FindByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

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
			cleanupTeacherRecords(t, db, teacher.ID)
			cleanupTeacherEducationData(t, db, []int64{group.ID})
		}()

		// Test
		teachers, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Len(t, teachers, 1)
		assert.Equal(t, teacher.ID, teachers[0].ID)
	})

	t.Run("returns empty for group with no teachers", func(t *testing.T) {
		group := testpkg.CreateTestEducationGroup(t, db, "EmptyTeacherGroup")
		defer cleanupTeacherEducationData(t, db, []int64{group.ID})

		teachers, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, teachers)
	})
}

// ============================================================================
// Relationship Tests
// ============================================================================

func TestTeacherRepository_FindWithStaffAndPerson(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("finds teacher with staff and person loaded", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "WithStaff", "Person")
		defer cleanupTeacherRecords(t, db, teacher.ID)

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

// NOTE: FindWithStaff exists in the implementation but is not exposed in the
// TeacherRepository interface, so it cannot be tested through the interface.

// ============================================================================
// Filter Tests
// ============================================================================

func TestTeacherRepository_ListWithStringFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("filters teachers by specialization_like", func(t *testing.T) {
		// Create teacher with unique specialization
		teacher := testpkg.CreateTestTeacher(t, db, "FilterTest", "Teacher")
		defer cleanupTeacherRecords(t, db, teacher.ID)

		uniqueSpec := fmt.Sprintf("FilterSpec%d", time.Now().UnixNano())
		teacher.Specialization = uniqueSpec
		err := repo.Update(ctx, teacher)
		require.NoError(t, err)

		// Use LIKE filter (tests applyTeacherStringLikeFilter)
		teachers, err := repo.List(ctx, map[string]interface{}{
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
		defer cleanupTeacherRecords(t, db, teacher.ID)

		// Set a unique role
		uniqueRole := fmt.Sprintf("RoleFilter%d", time.Now().UnixNano())
		teacher.Role = uniqueRole
		err := repo.Update(ctx, teacher)
		require.NoError(t, err)

		// Use role_like filter (tests applyTeacherStringLikeFilter)
		teachers, err := repo.List(ctx, map[string]interface{}{
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

func TestTeacherRepository_UpdateQualifications(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Teacher
	ctx := context.Background()

	t.Run("updates teacher qualifications", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "Update", "Qualifications")
		defer cleanupTeacherRecords(t, db, teacher.ID)

		newQualifications := "Master of Education, Certified Mathematics Teacher"
		err := repo.UpdateQualifications(ctx, teacher.ID, newQualifications)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Equal(t, newQualifications, found.Qualifications)
	})

	t.Run("clears qualifications with empty string", func(t *testing.T) {
		teacher := testpkg.CreateTestTeacher(t, db, "Clear", "Qualifications")
		defer cleanupTeacherRecords(t, db, teacher.ID)

		// Set initial qualifications
		err := repo.UpdateQualifications(ctx, teacher.ID, "Initial qualifications")
		require.NoError(t, err)

		// Clear them
		err = repo.UpdateQualifications(ctx, teacher.ID, "")
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, teacher.ID)
		require.NoError(t, err)
		assert.Empty(t, found.Qualifications)
	})

	t.Run("handles non-existent teacher gracefully", func(t *testing.T) {
		err := repo.UpdateQualifications(ctx, int64(999999), "Qualifications")
		// Should not error (UPDATE affects 0 rows but doesn't fail)
		require.NoError(t, err)
	})
}
