package education_test

import (
	"context"
	"testing"
	"time"

	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupGradeTransitionServiceTest creates service and returns cleanup function
func setupGradeTransitionServiceTest(t *testing.T) (educationService.GradeTransitionService, *bun.DB, func()) {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	transitionRepo := educationRepo.NewGradeTransitionRepository(db)
	studentRepo := usersRepo.NewStudentRepository(db)
	personRepo := usersRepo.NewPersonRepository(db)

	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo: transitionRepo,
		StudentRepo:    studentRepo,
		PersonRepo:     personRepo,
		DB:             db,
	})

	cleanup := func() {
		_ = db.Close()
	}

	return service, db, cleanup
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}

func TestGradeTransitionService_Create(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a test account for created_by
	account := testpkg.CreateTestAccount(t, db, "transition-creator@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("create transition without mappings", func(t *testing.T) {
		req := educationService.CreateTransitionRequest{
			AcademicYear: "2025-2026",
			CreatedBy:    account.ID,
		}

		transition, err := service.Create(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, transition)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		assert.Equal(t, "2025-2026", transition.AcademicYear)
		assert.Equal(t, education.TransitionStatusDraft, transition.Status)
		assert.Equal(t, account.ID, transition.CreatedBy)
		assert.Empty(t, transition.Mappings)
	})

	t.Run("create transition with mappings", func(t *testing.T) {
		req := educationService.CreateTransitionRequest{
			AcademicYear: "2026-2027",
			CreatedBy:    account.ID,
			Mappings: []educationService.MappingRequest{
				{FromClass: "1a", ToClass: strPtr("2a")},
				{FromClass: "4a", ToClass: nil}, // graduate
			},
		}

		transition, err := service.Create(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, transition)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		assert.Equal(t, "2026-2027", transition.AcademicYear)
		assert.Len(t, transition.Mappings, 2)
	})

	t.Run("create transition with notes", func(t *testing.T) {
		notes := "Test notes for transition"
		req := educationService.CreateTransitionRequest{
			AcademicYear: "2027-2028",
			CreatedBy:    account.ID,
			Notes:        &notes,
		}

		transition, err := service.Create(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, transition)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		require.NotNil(t, transition.Notes)
		assert.Equal(t, notes, *transition.Notes)
	})

	t.Run("create transition fails with empty academic year", func(t *testing.T) {
		req := educationService.CreateTransitionRequest{
			AcademicYear: "",
			CreatedBy:    account.ID,
		}

		_, err := service.Create(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "academic_year is required")
	})

	t.Run("create transition fails with invalid mapping", func(t *testing.T) {
		req := educationService.CreateTransitionRequest{
			AcademicYear: "2028-2029",
			CreatedBy:    account.ID,
			Mappings: []educationService.MappingRequest{
				{FromClass: "1a", ToClass: strPtr("1a")}, // same class
			},
		}

		_, err := service.Create(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be the same")
	})
}

func TestGradeTransitionService_Update(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-updater@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("update academic year", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		newYear := "2026-2027"
		req := educationService.UpdateTransitionRequest{
			AcademicYear: &newYear,
		}

		updated, err := service.Update(ctx, transition.ID, req)
		require.NoError(t, err)
		assert.Equal(t, "2026-2027", updated.AcademicYear)
	})

	t.Run("update notes", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		notes := "Updated notes"
		req := educationService.UpdateTransitionRequest{
			Notes: &notes,
		}

		updated, err := service.Update(ctx, transition.ID, req)
		require.NoError(t, err)
		require.NotNil(t, updated.Notes)
		assert.Equal(t, "Updated notes", *updated.Notes)
	})

	t.Run("update mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", strPtr("2a"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		req := educationService.UpdateTransitionRequest{
			Mappings: []educationService.MappingRequest{
				{FromClass: "2a", ToClass: strPtr("3a")},
				{FromClass: "3a", ToClass: strPtr("4a")},
			},
		}

		updated, err := service.Update(ctx, transition.ID, req)
		require.NoError(t, err)
		assert.Len(t, updated.Mappings, 2)
	})

	t.Run("cannot update applied transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		// Mark as applied
		now := time.Now()
		transition.Status = education.TransitionStatusApplied
		transition.AppliedAt = &now
		transition.AppliedBy = &account.ID
		_, err := db.NewUpdate().
			Model(transition).
			ModelTableExpr(`education.grade_transitions`).
			Column("status", "applied_at", "applied_by").
			Where("id = ?", transition.ID).
			Exec(ctx)
		require.NoError(t, err)

		newYear := "2026-2027"
		req := educationService.UpdateTransitionRequest{
			AcademicYear: &newYear,
		}

		_, err = service.Update(ctx, transition.ID, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot modify transition")
	})

	t.Run("update non-existent transition", func(t *testing.T) {
		req := educationService.UpdateTransitionRequest{}
		_, err := service.Update(ctx, 999999, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionService_Delete(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-deleter@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("delete draft transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

		err := service.Delete(ctx, transition.ID)
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetByID(ctx, transition.ID)
		require.Error(t, err)
	})

	t.Run("cannot delete applied transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		// Mark as applied
		now := time.Now()
		transition.Status = education.TransitionStatusApplied
		transition.AppliedAt = &now
		transition.AppliedBy = &account.ID
		_, err := db.NewUpdate().
			Model(transition).
			ModelTableExpr(`education.grade_transitions`).
			Column("status", "applied_at", "applied_by").
			Where("id = ?", transition.ID).
			Exec(ctx)
		require.NoError(t, err)

		err = service.Delete(ctx, transition.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot delete transition")
	})

	t.Run("delete non-existent transition", func(t *testing.T) {
		err := service.Delete(ctx, 999999)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionService_GetByID(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-getter@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("get transition with mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", strPtr("2a"))
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "2a", strPtr("3a"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		result, err := service.GetByID(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, transition.ID, result.ID)
		assert.Equal(t, "2025-2026", result.AcademicYear)
		assert.Len(t, result.Mappings, 2)
	})

	t.Run("get non-existent transition", func(t *testing.T) {
		_, err := service.GetByID(ctx, 999999)
		require.Error(t, err)
	})
}

func TestGradeTransitionService_List(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-lister@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("list transitions with pagination", func(t *testing.T) {
		// Create multiple transitions
		t1 := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		t2 := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
		t3 := testpkg.CreateTestGradeTransition(t, db, "2027-2028", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, t1.ID, t2.ID, t3.ID)

		options := base.NewQueryOptions()
		options.WithPagination(1, 2)

		transitions, total, err := service.List(ctx, options)
		require.NoError(t, err)
		assert.Len(t, transitions, 2)
		assert.GreaterOrEqual(t, total, 3)
	})

	t.Run("list transitions with filter", func(t *testing.T) {
		t1 := testpkg.CreateTestGradeTransition(t, db, "2029-2030", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, t1.ID)

		options := base.NewQueryOptions()
		filter := base.NewFilter()
		filter.Equal("academic_year", "2029-2030")
		options.Filter = filter

		transitions, _, err := service.List(ctx, options)
		require.NoError(t, err)
		for _, tr := range transitions {
			assert.Equal(t, "2029-2030", tr.AcademicYear)
		}
	})
}

func TestGradeTransitionService_Preview(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-preview@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("preview with students", func(t *testing.T) {
		// Create students in specific classes
		student1 := testpkg.CreateTestStudent(t, db, "Preview", "Student1", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Preview", "Student2", "1a")
		student3 := testpkg.CreateTestStudent(t, db, "Preview", "Student3", "4a")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, student3.ID)

		// Create transition with mappings
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", strPtr("2a"))
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "4a", nil) // graduate
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		preview, err := service.Preview(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, transition.ID, preview.TransitionID)
		assert.Equal(t, 3, preview.TotalStudents)
		assert.Equal(t, 2, preview.ToPromote)
		assert.Equal(t, 1, preview.ToGraduate)
		assert.Len(t, preview.ByMapping, 2)
	})

	t.Run("preview shows unmapped classes", func(t *testing.T) {
		// Create student in unmapped class
		student := testpkg.CreateTestStudent(t, db, "Unmapped", "Student", "3b")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create transition without mapping for 3b
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", strPtr("2a"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		preview, err := service.Preview(ctx, transition.ID)
		require.NoError(t, err)

		// Should have unmapped class warning
		found := false
		for _, uc := range preview.UnmappedClasses {
			if uc.ClassName == "3b" {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected 3b in unmapped classes")
		assert.NotEmpty(t, preview.Warnings)
	})

	t.Run("preview non-existent transition", func(t *testing.T) {
		_, err := service.Preview(ctx, 999999)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionService_Apply(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-applier@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("apply transition promotes students", func(t *testing.T) {
		// Create students in class 1a
		student1 := testpkg.CreateTestStudent(t, db, "Apply", "Student1", "1a")
		student2 := testpkg.CreateTestStudent(t, db, "Apply", "Student2", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

		// Create transition
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", strPtr("2a"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		result, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)
		assert.Equal(t, education.TransitionStatusApplied, result.Status)
		assert.Equal(t, 2, result.StudentsPromoted)
		assert.True(t, result.CanRevert)

		// Verify students were promoted
		var updatedStudent1 struct {
			SchoolClass string `bun:"school_class"`
		}
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("school_class").
			Where("id = ?", student1.ID).
			Scan(ctx, &updatedStudent1)
		require.NoError(t, err)
		assert.Equal(t, "2a", updatedStudent1.SchoolClass)
	})

	t.Run("apply transition creates history", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "History", "Student", "2b")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "2b", strPtr("3b"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)

		// Verify history was created
		history, err := service.GetHistory(ctx, transition.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, history)

		found := false
		for _, h := range history {
			if h.StudentID == student.ID {
				assert.Equal(t, "2b", h.FromClass)
				assert.NotNil(t, h.ToClass)
				assert.Equal(t, "3b", *h.ToClass)
				assert.Equal(t, education.ActionPromoted, h.Action)
				found = true
				break
			}
		}
		assert.True(t, found, "Expected history record for student")
	})

	t.Run("cannot apply already applied transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "9z", strPtr("10z"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		// First apply
		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)

		// Second apply should fail
		_, err = service.Apply(ctx, transition.ID, account.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already been applied")
	})

	t.Run("cannot apply transition without mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be in draft status with mappings")
	})
}

func TestGradeTransitionService_Revert(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-reverter@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("revert applied transition", func(t *testing.T) {
		// Create students
		student := testpkg.CreateTestStudent(t, db, "Revert", "Student", "1c")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create and apply transition
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1c", strPtr("2c"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)

		// Verify student is in 2c
		var classAfterApply string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("school_class").
			Where("id = ?", student.ID).
			Scan(ctx, &classAfterApply)
		require.NoError(t, err)
		assert.Equal(t, "2c", classAfterApply)

		// Revert
		result, err := service.Revert(ctx, transition.ID, account.ID)
		require.NoError(t, err)
		assert.Equal(t, education.TransitionStatusReverted, result.Status)
		assert.False(t, result.CanRevert)

		// Verify student is back in 1c
		var classAfterRevert string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("school_class").
			Where("id = ?", student.ID).
			Scan(ctx, &classAfterRevert)
		require.NoError(t, err)
		assert.Equal(t, "1c", classAfterRevert)
	})

	t.Run("cannot revert draft transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "5x", strPtr("6x"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		_, err := service.Revert(ctx, transition.ID, account.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has not been applied yet")
	})

	t.Run("cannot revert already reverted transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "6y", strPtr("7y"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		// Apply then revert
		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)
		_, err = service.Revert(ctx, transition.ID, account.ID)
		require.NoError(t, err)

		// Try to revert again
		_, err = service.Revert(ctx, transition.ID, account.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already been reverted")
	})
}

func TestGradeTransitionService_SuggestMappings(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("suggests promotion for lower grades", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Suggest", "Student", "1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		suggestions, err := service.SuggestMappings(ctx)
		require.NoError(t, err)

		// Find 1a suggestion
		found := false
		for _, s := range suggestions {
			if s.FromClass == "1a" {
				found = true
				assert.NotNil(t, s.ToClass)
				assert.Equal(t, "2a", *s.ToClass)
				assert.False(t, s.IsGraduating)
				break
			}
		}
		assert.True(t, found, "Expected suggestion for class 1a")
	})

	t.Run("suggests graduation for grade 4+", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "Graduate", "Student", "4b")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		suggestions, err := service.SuggestMappings(ctx)
		require.NoError(t, err)

		// Find 4b suggestion
		found := false
		for _, s := range suggestions {
			if s.FromClass == "4b" {
				found = true
				assert.Nil(t, s.ToClass)
				assert.True(t, s.IsGraduating)
				break
			}
		}
		assert.True(t, found, "Expected suggestion for class 4b")
	})

	t.Run("non-standard class names suggest graduation", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "NonStd", "Student", "special")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		suggestions, err := service.SuggestMappings(ctx)
		require.NoError(t, err)

		// Find special class suggestion
		found := false
		for _, s := range suggestions {
			if s.FromClass == "special" {
				found = true
				assert.Nil(t, s.ToClass)
				assert.True(t, s.IsGraduating)
				break
			}
		}
		assert.True(t, found, "Expected suggestion for class special")
	})
}

func TestGradeTransitionService_GetDistinctClasses(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("returns distinct classes", func(t *testing.T) {
		// Create students in different classes
		s1 := testpkg.CreateTestStudent(t, db, "Class", "Test1", "ClassA")
		s2 := testpkg.CreateTestStudent(t, db, "Class", "Test2", "ClassA") // duplicate class
		s3 := testpkg.CreateTestStudent(t, db, "Class", "Test3", "ClassB")
		defer testpkg.CleanupActivityFixtures(t, db, s1.ID, s2.ID, s3.ID)

		classes, err := service.GetDistinctClasses(ctx)
		require.NoError(t, err)

		// Should contain ClassA and ClassB
		classSet := make(map[string]bool)
		for _, c := range classes {
			classSet[c] = true
		}
		assert.True(t, classSet["ClassA"])
		assert.True(t, classSet["ClassB"])
	})
}

func TestGradeTransitionService_GetHistory(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-history@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("returns history after apply", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "History", "Test", "1d")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1d", strPtr("2d"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		// Apply transition
		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)

		// Get history
		history, err := service.GetHistory(ctx, transition.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, history)

		// Verify history record
		var found bool
		for _, h := range history {
			if h.StudentID == student.ID {
				found = true
				assert.Equal(t, transition.ID, h.TransitionID)
				assert.Equal(t, "1d", h.FromClass)
				assert.NotNil(t, h.ToClass)
				assert.Equal(t, "2d", *h.ToClass)
				assert.Contains(t, h.PersonName, "History")
			}
		}
		assert.True(t, found, "Expected history for student")
	})

	t.Run("empty history for transition without apply", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		history, err := service.GetHistory(ctx, transition.ID)
		require.NoError(t, err)
		assert.Empty(t, history)
	})
}
