package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories/education"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGradeTransitionRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	// Create test account for created_by
	account := testpkg.CreateTestAccount(t, db, "transition-creator")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("creates transition successfully", func(t *testing.T) {
		transition := &educationModels.GradeTransition{
			AcademicYear: "2025-2026",
			Status:       educationModels.TransitionStatusDraft,
			CreatedBy:    account.ID,
		}

		err := repo.Create(ctx, transition)
		require.NoError(t, err)
		assert.NotZero(t, transition.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		found, err := repo.FindByID(ctx, transition.ID)
		require.NoError(t, err)
		require.NotNil(t, found.Notes)
		assert.Equal(t, notes, *found.Notes)
	})
}

func TestGradeTransitionRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-find")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds existing transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-mappings")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds transition with mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		toClass := "2a"
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", &toClass)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1b", nil) // Graduate

		found, err := repo.FindByIDWithMappings(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, found.Mappings, 2)
	})
}

func TestGradeTransitionRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-update")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("updates transition notes", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-delete")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-list")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("lists all transitions", func(t *testing.T) {
		transition1 := testpkg.CreateTestGradeTransition(t, db, "2024-2025", account.ID)
		transition2 := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition1.ID, transition2.ID)

		transitions, total, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(transitions), 2)
	})
}

func TestGradeTransitionRepository_FindByAcademicYear(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-year")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds transitions by academic year", func(t *testing.T) {
		transition1 := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		transition2 := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		transition3 := testpkg.CreateTestGradeTransition(t, db, "2024-2025", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition1.ID, transition2.ID, transition3.ID)

		transitions, err := repo.FindByAcademicYear(ctx, "2025-2026")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(transitions), 2)

		for _, tr := range transitions {
			assert.Equal(t, "2025-2026", tr.AcademicYear)
		}
	})
}

func TestGradeTransitionRepository_FindByStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-status")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("finds transitions by status", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		transitions, err := repo.FindByStatus(ctx, educationModels.TransitionStatusDraft)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(transitions), 1)

		for _, tr := range transitions {
			assert.Equal(t, educationModels.TransitionStatusDraft, tr.Status)
		}
	})
}

func TestGradeTransitionRepository_MappingOperations(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-mapping-ops")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("creates and retrieves mapping", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	account := testpkg.CreateTestAccount(t, db, "transition-history-ops")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("creates and retrieves history", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	t.Run("returns distinct classes from students", func(t *testing.T) {
		// Create test students with different classes
		student1 := testpkg.CreateTestStudent(t, db, "Test1", "Student1", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Test2", "Student2", "1b")
		student3 := testpkg.CreateTestStudent(t, db, "Test3", "Student3", "2a")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, student3.ID)

		classes, err := repo.GetDistinctClasses(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(classes), 3)
		assert.Contains(t, classes, "1a")
		assert.Contains(t, classes, "1b")
		assert.Contains(t, classes, "2a")
	})
}

func TestGradeTransitionRepository_GetStudentCountByClass(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	t.Run("counts students in class", func(t *testing.T) {
		// Create unique class name using UUID to ensure test isolation
		className := fmt.Sprintf("test-count-%s", uuid.Must(uuid.NewV4()).String()[:8])
		student1 := testpkg.CreateTestStudent(t, db, "Count1", "Student1", className)
		student2 := testpkg.CreateTestStudent(t, db, "Count2", "Student2", className)
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := education.NewGradeTransitionRepository(db)
	ctx := context.Background()

	t.Run("gets students by classes", func(t *testing.T) {
		// Create unique class names using UUID to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		class1 := fmt.Sprintf("test-get-class1-%s", suffix)
		class2 := fmt.Sprintf("test-get-class2-%s", suffix)
		student1 := testpkg.CreateTestStudent(t, db, "Get1", "Student1", class1)
		student2 := testpkg.CreateTestStudent(t, db, "Get2", "Student2", class2)
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

		students, err := repo.GetStudentsByClasses(ctx, []string{class1, class2})
		require.NoError(t, err)
		assert.Equal(t, 2, len(students))
	})
}
