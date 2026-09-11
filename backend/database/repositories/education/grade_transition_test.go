package education_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGradeTransitionRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	// Create test account for created_by
	account := testpkg.CreateTestAccount(t, db, "transition-creator")

	t.Run("creates transition successfully", func(t *testing.T) {
		transition := &educationModels.GradeTransition{
			AcademicYear: "2025-2026",
			Status:       educationModels.TransitionStatusDraft,
			CreatedBy:    account.ID,
		}

		err := repo.Create(ctx, transition)
		require.NoError(t, err)
		assert.NotZero(t, transition.ID)

		// Verify it was created
		found, err := repo.FindByID(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, "2025-2026", found.AcademicYear)
		assert.Equal(t, educationModels.TransitionStatusDraft, found.Status)
	})

	t.Run("creates transition with notes", func(t *testing.T) {
		notes := "Test notes for transition"
		transition := &educationModels.GradeTransition{
			AcademicYear: "2026-2027",
			Status:       educationModels.TransitionStatusDraft,
			CreatedBy:    account.ID,
			Notes:        &notes,
		}

		err := repo.Create(ctx, transition)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, transition.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Notes)
		assert.Equal(t, notes, *found.Notes)
	})
}

func TestGradeTransitionRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-find")

	t.Run("finds existing transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		found, err := repo.FindByID(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, transition.ID, found.ID)
		assert.Equal(t, "2025-2026", found.AcademicYear)
	})

	t.Run("returns error for non-existent transition", func(t *testing.T) {
		_, err := repo.FindByID(ctx, 999999)
		require.Error(t, err)
	})
}

func TestGradeTransitionRepository_FindByIDWithMappings(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-mappings")

	t.Run("finds transition with mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		toClass := "2a"
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", &toClass)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1b", nil) // Graduate

		found, err := repo.FindByIDWithMappings(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, found.Mappings, 2)
	})
}

func TestGradeTransitionRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-update")

	t.Run("updates transition notes", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		notes := "Updated notes"
		transition.Notes = &notes

		err := repo.Update(ctx, transition)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, transition.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Notes)
		assert.Equal(t, notes, *found.Notes)
	})

	t.Run("updates transition status", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		now := time.Now()
		transition.Status = educationModels.TransitionStatusApplied
		transition.AppliedAt = &now
		transition.AppliedBy = &account.ID

		err := repo.Update(ctx, transition)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, educationModels.TransitionStatusApplied, found.Status)
		require.NotNil(t, found.AppliedAt)
		require.NotNil(t, found.AppliedBy)
	})
}

func TestGradeTransitionRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-delete")

	t.Run("deletes transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		// No defer cleanup needed - we're testing delete

		err := repo.Delete(ctx, transition.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, transition.ID)
		require.Error(t, err)
	})

	t.Run("deletes transition with mappings (cascade)", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		toClass := "2a"
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", &toClass)

		err := repo.Delete(ctx, transition.ID)
		require.NoError(t, err)

		// Verify mappings are also deleted
		mappings, err := repo.GetMappings(ctx, transition.ID)
		require.NoError(t, err)
		assert.Empty(t, mappings)
	})
}

func TestGradeTransitionRepository_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-list")

	t.Run("lists all transitions", func(t *testing.T) {
		testpkg.CreateTestGradeTransition(t, db, "2024-2025", account.ID)
		testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		transitions, total, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(transitions), 2)
	})
}

func TestGradeTransitionRepository_FindByAcademicYear(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-year")

	t.Run("finds transitions by academic year", func(t *testing.T) {
		testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransition(t, db, "2024-2025", account.ID)

		transitions, err := repo.FindByAcademicYear(ctx, "2025-2026")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(transitions), 2)

		for _, tr := range transitions {
			assert.Equal(t, "2025-2026", tr.AcademicYear)
		}
	})
}

func TestGradeTransitionRepository_FindByStatus(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-status")

	t.Run("finds transitions by status", func(t *testing.T) {
		testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		transitions, err := repo.FindByStatus(ctx, educationModels.TransitionStatusDraft)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(transitions), 1)

		for _, tr := range transitions {
			assert.Equal(t, educationModels.TransitionStatusDraft, tr.Status)
		}
	})
}

func TestGradeTransitionRepository_MappingOperations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-mapping-ops")

	t.Run("creates and retrieves mapping", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		toClass := "2a"
		mapping := &educationModels.GradeTransitionMapping{
			TransitionID: transition.ID,
			FromClass:    "1a",
			ToClass:      &toClass,
		}

		err := repo.CreateMapping(ctx, mapping)
		require.NoError(t, err)
		assert.NotZero(t, mapping.ID)

		mappings, err := repo.GetMappings(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, mappings, 1)
		assert.Equal(t, "1a", mappings[0].FromClass)
		assert.Equal(t, "2a", *mappings[0].ToClass)
	})

	t.Run("creates multiple mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		toClass2a := "2a"
		toClass2b := "2b"
		mappings := []*educationModels.GradeTransitionMapping{
			{TransitionID: transition.ID, FromClass: "1a", ToClass: &toClass2a},
			{TransitionID: transition.ID, FromClass: "1b", ToClass: &toClass2b},
			{TransitionID: transition.ID, FromClass: "4a", ToClass: nil}, // Graduate
		}

		err := repo.CreateMappings(ctx, mappings)
		require.NoError(t, err)

		retrieved, err := repo.GetMappings(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, retrieved, 3)
	})

	t.Run("deletes mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		toClass := "2a"
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", &toClass)

		err := repo.DeleteMappings(ctx, transition.ID)
		require.NoError(t, err)

		mappings, err := repo.GetMappings(ctx, transition.ID)
		require.NoError(t, err)
		assert.Empty(t, mappings)
	})
}

func TestGradeTransitionRepository_HistoryOperations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-history-ops")

	t.Run("creates and retrieves history", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		toClass := "2a"
		history := &educationModels.GradeTransitionHistory{
			TransitionID: transition.ID,
			StudentID:    123,
			PersonName:   "Max Mustermann",
			FromClass:    "1a",
			ToClass:      &toClass,
			Action:       educationModels.ActionPromoted,
		}

		err := repo.CreateHistory(ctx, history)
		require.NoError(t, err)
		assert.NotZero(t, history.ID)

		historyRecords, err := repo.GetHistory(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, historyRecords, 1)
		assert.Equal(t, "Max Mustermann", historyRecords[0].PersonName)
	})

	t.Run("creates batch history", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		toClass := "2a"
		historyBatch := []*educationModels.GradeTransitionHistory{
			{
				TransitionID: transition.ID,
				StudentID:    1,
				PersonName:   "Student 1",
				FromClass:    "1a",
				ToClass:      &toClass,
				Action:       educationModels.ActionPromoted,
			},
			{
				TransitionID: transition.ID,
				StudentID:    2,
				PersonName:   "Student 2",
				FromClass:    "4a",
				ToClass:      nil,
				Action:       educationModels.ActionGraduated,
			},
		}

		err := repo.CreateHistoryBatch(ctx, historyBatch)
		require.NoError(t, err)

		historyRecords, err := repo.GetHistory(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, historyRecords, 2)
	})
}

func TestGradeTransitionRepository_GetDistinctClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns distinct classes from students", func(t *testing.T) {
		// Create test students with different classes
		testpkg.CreateTestStudent(t, db, "Test1", "Student1", "1a")
		testpkg.CreateTestStudent(t, db, "Test2", "Student2", "1b")
		testpkg.CreateTestStudent(t, db, "Test3", "Student3", "2a")

		classes, err := repo.GetDistinctClasses(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(classes), 3)
		assert.Contains(t, classes, "1a")
		assert.Contains(t, classes, "1b")
		assert.Contains(t, classes, "2a")
	})
}

func TestGradeTransitionRepository_GetStudentCountByClass(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("counts students in class", func(t *testing.T) {
		// Create unique class name using UUID to ensure test isolation
		className := fmt.Sprintf("test-count-%s", uuid.Must(uuid.NewV4()).String()[:8])
		testpkg.CreateTestStudent(t, db, "Count1", "Student1", className)
		testpkg.CreateTestStudent(t, db, "Count2", "Student2", className)

		count, err := repo.GetStudentCountByClass(ctx, className)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("returns zero for non-existent class", func(t *testing.T) {
		count, err := repo.GetStudentCountByClass(ctx, "non-existent-class")
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

func TestGradeTransitionRepository_GetStudentsByClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("gets students by classes", func(t *testing.T) {
		// Create unique class names using UUID to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		class1 := fmt.Sprintf("test-get-class1-%s", suffix)
		class2 := fmt.Sprintf("test-get-class2-%s", suffix)
		testpkg.CreateTestStudent(t, db, "Get1", "Student1", class1)
		testpkg.CreateTestStudent(t, db, "Get2", "Student2", class2)

		students, err := repo.GetStudentsByClasses(ctx, []string{class1, class2})
		require.NoError(t, err)
		assert.Equal(t, 2, len(students))
	})

	t.Run("returns empty for empty class list", func(t *testing.T) {
		students, err := repo.GetStudentsByClasses(ctx, []string{})
		require.NoError(t, err)
		assert.Empty(t, students)
	})

	t.Run("returns empty for non-existent classes", func(t *testing.T) {
		students, err := repo.GetStudentsByClasses(ctx, []string{"non-existent-class-xyz"})
		require.NoError(t, err)
		assert.Empty(t, students)
	})
}

func TestGradeTransitionRepository_UpdateStudentClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-update-classes")

	t.Run("updates student classes based on mappings", func(t *testing.T) {
		// Create unique class names for test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1x-%s", suffix)
		toClass := fmt.Sprintf("2x-%s", suffix)

		// Create students in fromClass
		student1 := testpkg.CreateTestStudent(t, db, "Update1", "Test1", fromClass)
		testpkg.CreateTestStudent(t, db, "Update2", "Test2", fromClass)

		// Create transition with mapping
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, &toClass)

		// Execute update
		affected, err := repo.UpdateStudentClasses(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(2), affected)

		// Verify students were updated
		var updatedClass string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("school_class").
			Where("id = ?", student1.ID).
			Scan(ctx, &updatedClass)
		require.NoError(t, err)
		assert.Equal(t, toClass, updatedClass)
	})

	t.Run("does not update students with graduate mapping (to_class = null)", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		graduateClass := fmt.Sprintf("4x-%s", suffix)

		// Create student in graduate class
		student := testpkg.CreateTestStudent(t, db, "Graduate", "Test", graduateClass)

		// Create transition with graduate mapping (to_class = null)
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, graduateClass, nil)

		// Execute update - should not affect graduate students
		affected, err := repo.UpdateStudentClasses(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)

		// Verify student still has original class
		var currentClass string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("school_class").
			Where("id = ?", student.ID).
			Scan(ctx, &currentClass)
		require.NoError(t, err)
		assert.Equal(t, graduateClass, currentClass)
	})

	t.Run("handles non-existent transition ID", func(t *testing.T) {
		affected, err := repo.UpdateStudentClasses(ctx, 999999)
		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})

	t.Run("handles transition with no matching students", func(t *testing.T) {
		// Create transition with mapping for non-existent class
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		nonExistent := "non-existent-class-xyz"
		toClass := "target-class-xyz"
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, nonExistent, &toClass)

		affected, err := repo.UpdateStudentClasses(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})
}

func TestGradeTransitionRepository_GraduateStudentsByClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	studentStatus := func(id int64) string {
		var status string
		err := db.NewSelect().
			TableExpr(`users.students`).
			Column("status").
			Where("id = ?", id).
			Scan(ctx, &status)
		require.NoError(t, err)
		return status
	}

	t.Run("marks students in specified classes as alumnus", func(t *testing.T) {
		// Create unique class names for test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		graduateClass := fmt.Sprintf("grad-%s", suffix)

		// Create students in graduate class
		student1 := testpkg.CreateTestStudent(t, db, "Grad1", "Test1", graduateClass)
		student2 := testpkg.CreateTestStudent(t, db, "Grad2", "Test2", graduateClass)

		// Execute graduation (soft delete)
		affected, err := repo.GraduateStudentsByClasses(ctx, []string{graduateClass})
		require.NoError(t, err)
		assert.Equal(t, int64(2), affected)

		// Verify students still exist but are alumni
		assert.Equal(t, string(users.StudentStatusAlumnus), studentStatus(student1.ID))
		assert.Equal(t, string(users.StudentStatusAlumnus), studentStatus(student2.ID))
	})

	t.Run("returns zero for empty class list", func(t *testing.T) {
		affected, err := repo.GraduateStudentsByClasses(ctx, []string{})
		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})

	t.Run("returns zero for non-existent classes", func(t *testing.T) {
		affected, err := repo.GraduateStudentsByClasses(ctx, []string{"non-existent-class-xyz"})
		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})

	t.Run("graduates only from specified classes", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		gradClass := fmt.Sprintf("del-%s", suffix)
		keepClass := fmt.Sprintf("keep-%s", suffix)

		// Create students
		studentToGraduate := testpkg.CreateTestStudent(t, db, "Graduate", "Test", gradClass)
		studentToKeep := testpkg.CreateTestStudent(t, db, "Keep", "Test", keepClass)

		// Graduate only gradClass
		affected, err := repo.GraduateStudentsByClasses(ctx, []string{gradClass})
		require.NoError(t, err)
		assert.Equal(t, int64(1), affected)

		assert.Equal(t, string(users.StudentStatusAlumnus), studentStatus(studentToGraduate.ID))
		assert.Equal(t, string(users.StudentStatusActive), studentStatus(studentToKeep.ID))
	})

	t.Run("already-alumnus students are not re-graduated", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		gradClass := fmt.Sprintf("regrad-%s", suffix)

		testpkg.CreateTestStudent(t, db, "Regrad", "Test", gradClass)

		affected, err := repo.GraduateStudentsByClasses(ctx, []string{gradClass})
		require.NoError(t, err)
		assert.Equal(t, int64(1), affected)

		// Second run must not count the alumnus again
		affected, err = repo.GraduateStudentsByClasses(ctx, []string{gradClass})
		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})
}

func TestGradeTransitionRepository_ReactivateStudentsByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("restores alumni back to active", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		gradClass := fmt.Sprintf("react-%s", suffix)

		student := testpkg.CreateTestStudent(t, db, "React", "Test", gradClass)

		_, err := repo.GraduateStudentsByClasses(ctx, []string{gradClass})
		require.NoError(t, err)

		affected, err := repo.ReactivateStudentsByIDs(ctx, []int64{student.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(1), affected)

		var status string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("status").
			Where("id = ?", student.ID).
			Scan(ctx, &status)
		require.NoError(t, err)
		assert.Equal(t, string(users.StudentStatusActive), status)
	})

	t.Run("does not touch non-alumnus students", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		keepClass := fmt.Sprintf("noreact-%s", suffix)

		student := testpkg.CreateTestStudent(t, db, "NoReact", "Test", keepClass)

		affected, err := repo.ReactivateStudentsByIDs(ctx, []int64{student.ID})
		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})

	t.Run("returns zero for empty id list", func(t *testing.T) {
		affected, err := repo.ReactivateStudentsByIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Equal(t, int64(0), affected)
	})
}

// ============================================================================
// Edge Case Tests for Create/Update with Nil
// ============================================================================

func TestGradeTransitionRepository_CreateWithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("create with nil transition returns error", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGradeTransitionRepository_UpdateWithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("update with nil transition returns error", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGradeTransitionRepository_CreateMappingWithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("create mapping with nil returns error", func(t *testing.T) {
		err := repo.CreateMapping(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGradeTransitionRepository_CreateMappingsWithEmpty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("create mappings with empty slice succeeds", func(t *testing.T) {
		err := repo.CreateMappings(ctx, []*educationModels.GradeTransitionMapping{})
		require.NoError(t, err)
	})
}

func TestGradeTransitionRepository_CreateHistoryWithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("create history with nil returns error", func(t *testing.T) {
		err := repo.CreateHistory(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestGradeTransitionRepository_CreateHistoryBatchWithEmpty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("create history batch with empty slice succeeds", func(t *testing.T) {
		err := repo.CreateHistoryBatch(ctx, []*educationModels.GradeTransitionHistory{})
		require.NoError(t, err)
	})
}

// ============================================================================
// Validation Error Tests
// ============================================================================

func TestGradeTransitionRepository_CreateWithInvalidData(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("create with invalid academic year format fails", func(t *testing.T) {
		transition := &educationModels.GradeTransition{
			AcademicYear: "invalid-format",
			Status:       educationModels.TransitionStatusDraft,
			CreatedBy:    1,
		}

		err := repo.Create(ctx, transition)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "format")
	})

	t.Run("create with invalid status fails", func(t *testing.T) {
		transition := &educationModels.GradeTransition{
			AcademicYear: "2025-2026",
			Status:       "invalid-status",
			CreatedBy:    1,
		}

		err := repo.Create(ctx, transition)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})

	t.Run("create with missing created_by fails", func(t *testing.T) {
		transition := &educationModels.GradeTransition{
			AcademicYear: "2025-2026",
			Status:       educationModels.TransitionStatusDraft,
			CreatedBy:    0,
		}

		err := repo.Create(ctx, transition)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "created_by")
	})
}

func TestGradeTransitionRepository_CreateMappingWithInvalidData(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "mapping-invalid")

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

	t.Run("create mapping with empty from_class fails", func(t *testing.T) {
		toClass := "2a"
		mapping := &educationModels.GradeTransitionMapping{
			TransitionID: transition.ID,
			FromClass:    "",
			ToClass:      &toClass,
		}

		err := repo.CreateMapping(ctx, mapping)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_class")
	})

	t.Run("create mapping with same from and to class fails", func(t *testing.T) {
		sameClass := "1a"
		mapping := &educationModels.GradeTransitionMapping{
			TransitionID: transition.ID,
			FromClass:    sameClass,
			ToClass:      &sameClass,
		}

		err := repo.CreateMapping(ctx, mapping)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be the same")
	})
}

func TestGradeTransitionRepository_CreateHistoryWithInvalidData(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "history-invalid")

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

	t.Run("create history with invalid action fails", func(t *testing.T) {
		history := &educationModels.GradeTransitionHistory{
			TransitionID: transition.ID,
			StudentID:    1,
			PersonName:   "Test Student",
			FromClass:    "1a",
			Action:       "invalid-action",
		}

		err := repo.CreateHistory(ctx, history)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid action")
	})

	t.Run("create history with missing student_id fails", func(t *testing.T) {
		history := &educationModels.GradeTransitionHistory{
			TransitionID: transition.ID,
			StudentID:    0,
			PersonName:   "Test Student",
			FromClass:    "1a",
			Action:       educationModels.ActionPromoted,
		}

		err := repo.CreateHistory(ctx, history)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "student_id")
	})
}
