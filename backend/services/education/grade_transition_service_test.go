package education_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupGradeTransitionServiceTest creates service and returns cleanup function
func setupGradeTransitionServiceTest(t *testing.T) (*educationService.GradeTransitionService, *bun.DB, func()) {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	transitionRepo := educationRepo.NewGradeTransitionRepository(db)
	studentRepo := usersRepo.NewStudentRepository(db)
	personRepo := usersRepo.NewPersonRepository(db)

	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo:   transitionRepo,
		StudentRepo:      studentRepo,
		PersonRepo:       personRepo,
		ClassTeacherRepo: educationRepo.NewClassTeacherRepository(db),
		StaffRepo:        usersRepo.NewStaffRepository(db),
		DB:               db,
	})

	cleanup := func() {
	}

	return service, db, cleanup
}

func TestGradeTransitionService_Create(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
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
				{FromClass: "1a", ToClass: testpkg.StrPtr("2a")},
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
				{FromClass: "1a", ToClass: testpkg.StrPtr("1a")}, // same class
			},
		}

		_, err := service.Create(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be the same")
	})
}

func TestGradeTransitionService_Update(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
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
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", testpkg.StrPtr("2a"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		req := educationService.UpdateTransitionRequest{
			Mappings: []educationService.MappingRequest{
				{FromClass: "2a", ToClass: testpkg.StrPtr("3a")},
				{FromClass: "3a", ToClass: testpkg.StrPtr("4a")},
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
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
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
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-getter@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("get transition with mappings", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", testpkg.StrPtr("2a"))
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "2a", testpkg.StrPtr("3a"))
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
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
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
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-preview@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("preview with students", func(t *testing.T) {
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		class1 := fmt.Sprintf("1a-%s", suffix)
		class2 := fmt.Sprintf("2a-%s", suffix)
		class4 := fmt.Sprintf("4a-%s", suffix)

		// Create students in specific classes
		student1 := testpkg.CreateTestStudent(t, db, "Preview", "Student1", class1)
		student2 := testpkg.CreateTestStudent(t, db, "Preview", "Student2", class1)
		student3 := testpkg.CreateTestStudent(t, db, "Preview", "Student3", class4)
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID, student3.ID)

		// Create transition with mappings
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class4, nil) // graduate
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
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		unmappedClass := fmt.Sprintf("3b-%s", suffix)
		mappedClass := fmt.Sprintf("1a-%s", suffix)
		targetClass := fmt.Sprintf("2a-%s", suffix)

		// Create student in unmapped class
		student := testpkg.CreateTestStudent(t, db, "Unmapped", "Student", unmappedClass)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create transition without mapping for unmappedClass
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, mappedClass, testpkg.StrPtr(targetClass))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		preview, err := service.Preview(ctx, transition.ID)
		require.NoError(t, err)

		// Should have unmapped class warning
		found := false
		for _, uc := range preview.UnmappedClasses {
			if uc.ClassName == unmappedClass {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected %s in unmapped classes", unmappedClass)
		assert.NotEmpty(t, preview.Warnings)
	})

	t.Run("preview non-existent transition", func(t *testing.T) {
		_, err := service.Preview(ctx, 999999)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionService_Apply(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-applier@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("apply transition promotes students", func(t *testing.T) {
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1a-%s", suffix)
		toClass := fmt.Sprintf("2a-%s", suffix)

		// Create students in fromClass
		student1 := testpkg.CreateTestStudent(t, db, "Apply", "Student1", fromClass)
		student2 := testpkg.CreateTestStudent(t, db, "Apply", "Student2", fromClass)
		defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student2.ID)

		// Create transition
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, testpkg.StrPtr(toClass))
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
		assert.Equal(t, toClass, updatedStudent1.SchoolClass)
	})

	t.Run("apply transition creates history", func(t *testing.T) {
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("2b-%s", suffix)
		toClass := fmt.Sprintf("3b-%s", suffix)

		student := testpkg.CreateTestStudent(t, db, "History", "Student", fromClass)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, testpkg.StrPtr(toClass))
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
				assert.Equal(t, fromClass, h.FromClass)
				assert.NotNil(t, h.ToClass)
				assert.Equal(t, toClass, *h.ToClass)
				assert.Equal(t, education.ActionPromoted, h.Action)
				found = true
				break
			}
		}
		assert.True(t, found, "Expected history record for student")
	})

	t.Run("cannot apply already applied transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "9z", testpkg.StrPtr("10z"))
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
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-reverter@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("revert applied transition", func(t *testing.T) {
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1c-%s", suffix)
		toClass := fmt.Sprintf("2c-%s", suffix)

		// Create students
		student := testpkg.CreateTestStudent(t, db, "Revert", "Student", fromClass)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create and apply transition
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, testpkg.StrPtr(toClass))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)

		// Verify student is in toClass
		var classAfterApply string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("school_class").
			Where("id = ?", student.ID).
			Scan(ctx, &classAfterApply)
		require.NoError(t, err)
		assert.Equal(t, toClass, classAfterApply)

		// Revert
		result, err := service.Revert(ctx, transition.ID, account.ID)
		require.NoError(t, err)
		assert.Equal(t, education.TransitionStatusReverted, result.Status)
		assert.False(t, result.CanRevert)

		// Verify student is back in fromClass
		var classAfterRevert string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("school_class").
			Where("id = ?", student.ID).
			Scan(ctx, &classAfterRevert)
		require.NoError(t, err)
		assert.Equal(t, fromClass, classAfterRevert)
	})

	t.Run("cannot revert draft transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "5x", testpkg.StrPtr("6x"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		_, err := service.Revert(ctx, transition.ID, account.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has not been applied yet")
	})

	t.Run("cannot revert already reverted transition", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "6y", testpkg.StrPtr("7y"))
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
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
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

	t.Run("prefixed class names suggest increment with prefix kept", func(t *testing.T) {
		// "Klasse 1a" style names are common in German schools — the grade
		// number inside must be incremented while the prefix is preserved.
		student := testpkg.CreateTestStudent(t, db, "Prefixed", "Student", "Klasse 1a")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		suggestions, err := service.SuggestMappings(ctx)
		require.NoError(t, err)

		found := false
		for _, s := range suggestions {
			if s.FromClass == "Klasse 1a" {
				found = true
				require.NotNil(t, s.ToClass)
				assert.Equal(t, "Klasse 2a", *s.ToClass)
				assert.False(t, s.IsGraduating)
				break
			}
		}
		assert.True(t, found, "Expected suggestion for class Klasse 1a")
	})

	t.Run("prefixed grade 4 suggests graduation", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "PrefixedGrad", "Student", "Klasse 4b")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		suggestions, err := service.SuggestMappings(ctx)
		require.NoError(t, err)

		found := false
		for _, s := range suggestions {
			if s.FromClass == "Klasse 4b" {
				found = true
				assert.Nil(t, s.ToClass)
				assert.True(t, s.IsGraduating)
				break
			}
		}
		assert.True(t, found, "Expected graduation suggestion for Klasse 4b")
	})

	t.Run("digit-only class names suggest increment", func(t *testing.T) {
		// Some schools name classes just "1", "2", ... — those must be
		// promoted numerically, not suggested as graduation.
		student := testpkg.CreateTestStudent(t, db, "DigitOnly", "Student", "2")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		suggestions, err := service.SuggestMappings(ctx)
		require.NoError(t, err)

		found := false
		for _, s := range suggestions {
			if s.FromClass == "2" {
				found = true
				require.NotNil(t, s.ToClass)
				assert.Equal(t, "3", *s.ToClass)
				assert.False(t, s.IsGraduating)
				break
			}
		}
		assert.True(t, found, "Expected suggestion for class 2")
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
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
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
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-history@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("returns history after apply", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, db, "History", "Test", "1d")
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1d", testpkg.StrPtr("2d"))
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

// ============================================================================
// Additional Edge Case Tests for Service
// ============================================================================

func TestGradeTransitionService_Apply_RevertedTransition(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-apply-reverted@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("cannot apply reverted transition", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1z-%s", suffix)
		toClass := fmt.Sprintf("2z-%s", suffix)

		// Create transition and mapping
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, testpkg.StrPtr(toClass))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		// Apply then revert
		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)
		_, err = service.Revert(ctx, transition.ID, account.ID)
		require.NoError(t, err)

		// Try to apply again - should fail
		_, err = service.Apply(ctx, transition.ID, account.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "has been reverted")
	})
}

func TestGradeTransitionService_Create_InvalidAcademicYearFormat(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-invalid-year@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("create fails with invalid academic year format", func(t *testing.T) {
		req := educationService.CreateTransitionRequest{
			AcademicYear: "invalid-year",
			CreatedBy:    account.ID,
		}

		_, err := service.Create(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "format")
	})

	t.Run("create fails with partial academic year", func(t *testing.T) {
		req := educationService.CreateTransitionRequest{
			AcademicYear: "2025",
			CreatedBy:    account.ID,
		}

		_, err := service.Create(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "format")
	})
}

func TestGradeTransitionService_Update_InvalidAcademicYearFormat(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-update-invalid@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("update fails with invalid academic year format", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		invalidYear := "bad-format"
		req := educationService.UpdateTransitionRequest{
			AcademicYear: &invalidYear,
		}

		_, err := service.Update(ctx, transition.ID, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "format")
	})
}

func TestGradeTransitionService_Update_InvalidMapping(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-update-invalid-map@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("update fails with invalid mapping (same from and to)", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", testpkg.StrPtr("2a"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		notes := "must not be saved"
		req := educationService.UpdateTransitionRequest{
			Notes: &notes,
			Mappings: []educationService.MappingRequest{
				{FromClass: "1a", ToClass: testpkg.StrPtr("1a")},
			},
		}

		_, err := service.Update(ctx, transition.ID, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be the same")

		unchanged, err := service.GetByID(ctx, transition.ID)
		require.NoError(t, err)
		assert.Nil(t, unchanged.Notes)
		require.Len(t, unchanged.Mappings, 1)
		assert.Equal(t, "1a", unchanged.Mappings[0].FromClass)
		require.NotNil(t, unchanged.Mappings[0].ToClass)
		assert.Equal(t, "2a", *unchanged.Mappings[0].ToClass)
	})

	t.Run("update fails with empty from_class", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		req := educationService.UpdateTransitionRequest{
			Mappings: []educationService.MappingRequest{
				{FromClass: "", ToClass: testpkg.StrPtr("2a")},
			},
		}

		_, err := service.Update(ctx, transition.ID, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_class")
	})
}

// Deliberately NOT parallel: the rollback check counts grade transitions by
// academic year alone, with no tenant filter, so it also sees the rows of
// tests running beside it.
func TestGradeTransitionService_Create_InvalidMapping(t *testing.T) {
	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-create-invalid-map@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("create fails with empty from_class", func(t *testing.T) {
		req := educationService.CreateTransitionRequest{
			AcademicYear: "2025-2026",
			CreatedBy:    account.ID,
			Mappings: []educationService.MappingRequest{
				{FromClass: "", ToClass: testpkg.StrPtr("2a")},
			},
		}

		_, err := service.Create(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "from_class")

		count, err := db.NewSelect().
			Model((*education.GradeTransition)(nil)).
			Where("academic_year = ?", req.AcademicYear).
			Count(ctx)
		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("create fails with duplicate source classes", func(t *testing.T) {
		req := educationService.CreateTransitionRequest{
			AcademicYear: "2026-2027",
			CreatedBy:    account.ID,
			Mappings: []educationService.MappingRequest{
				{FromClass: "1a", ToClass: testpkg.StrPtr("2a")},
				{FromClass: " 1a ", ToClass: testpkg.StrPtr("2b")},
			},
		}

		_, err := service.Create(ctx, req)
		require.ErrorIs(t, err, educationService.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "duplicate mapping")

		count, err := db.NewSelect().
			Model((*education.GradeTransition)(nil)).
			Where("academic_year = ?", req.AcademicYear).
			Count(ctx)
		require.NoError(t, err)
		assert.Zero(t, count)
	})
}

func TestGradeTransitionService_Revert_NonExistentTransition(t *testing.T) {
	t.Parallel()

	service, _, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("revert non-existent transition", func(t *testing.T) {
		_, err := service.Revert(ctx, 999999, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionService_Apply_NonExistentTransition(t *testing.T) {
	t.Parallel()

	service, _, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("apply non-existent transition", func(t *testing.T) {
		_, err := service.Apply(ctx, 999999, 1)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionService_SuggestMappings_EmptyResult(t *testing.T) {
	t.Parallel()

	service, _, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("suggest mappings returns sorted results", func(t *testing.T) {
		suggestions, err := service.SuggestMappings(ctx)
		require.NoError(t, err)
		// Results should be sorted alphabetically by FromClass
		for i := 1; i < len(suggestions); i++ {
			assert.LessOrEqual(t, suggestions[i-1].FromClass, suggestions[i].FromClass)
		}
	})
}

func TestGradeTransitionService_Apply_GraduateStudents(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-graduate@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("apply transition graduates students and creates warning", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		graduateClass := fmt.Sprintf("4grad-%s", suffix)

		// Create student to be graduated (soft-deactivated as alumnus)
		student := testpkg.CreateTestStudent(t, db, "Graduate", "Student", graduateClass)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create transition with graduate mapping
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, graduateClass, nil)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		result, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, result.StudentsGraduated)
		assert.NotEmpty(t, result.Warnings)

		// Verify warning mentions the alumnus soft-deactivation, not deletion
		foundWarning := false
		for _, w := range result.Warnings {
			if strings.Contains(w, "marked as alumni") {
				foundWarning = true
				break
			}
		}
		assert.True(t, foundWarning, "expected a 'marked as alumni' warning, got %v", result.Warnings)

		// Verify student was NOT deleted — row kept with status alumnus
		var status string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("status").
			Where("id = ?", student.ID).
			Scan(ctx, &status)
		require.NoError(t, err)
		assert.Equal(t, string(users.StudentStatusAlumnus), status)
	})
}

// TestGradeTransitionService_Apply_CascadingGraduation guards the ordering
// bug where a promotion moves students INTO a class that is graduated in the
// same transition. E.g. "3a -> 4a" (promote) together with "4a -> graduate":
// the 3a children must NOT be graduated just because they land in 4a. Only the
// original 4a members graduate; the promoted-in 3a children stay active.
func TestGradeTransitionService_Apply_CascadingGraduation(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-cascade@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	lower := fmt.Sprintf("3c-%s", suffix) // promoted into `upper`
	upper := fmt.Sprintf("4c-%s", suffix) // graduated

	promoted := testpkg.CreateTestStudent(t, db, "Promoted", "Kid", lower)
	graduating := testpkg.CreateTestStudent(t, db, "Graduating", "Kid", upper)
	defer testpkg.CleanupActivityFixtures(t, db, promoted.ID, graduating.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, lower, &upper) // promote 3c -> 4c
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, upper, nil)    // graduate 4c
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	result, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.StudentsPromoted, "only the 3c child is promoted")
	assert.Equal(t, 1, result.StudentsGraduated, "only the original 4c child graduates")

	readStudent := func(id int64) (string, string) {
		var class, status string
		err := db.NewSelect().
			TableExpr(`users.students`).
			ColumnExpr("school_class").
			ColumnExpr("status").
			Where("id = ?", id).
			Scan(ctx, &class, &status)
		require.NoError(t, err)
		return class, status
	}

	// The promoted child now sits in 4c but must remain active.
	pClass, pStatus := readStudent(promoted.ID)
	assert.Equal(t, upper, pClass)
	assert.Equal(t, string(users.StudentStatusActive), pStatus,
		"promoted child must not be graduated by landing in the graduated class")

	// The original 4c child is the alumnus.
	gClass, gStatus := readStudent(graduating.ID)
	assert.Equal(t, upper, gClass)
	assert.Equal(t, string(users.StudentStatusAlumnus), gStatus)

	// Revert restores both cleanly.
	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	pClass, pStatus = readStudent(promoted.ID)
	assert.Equal(t, lower, pClass, "promoted child returns to 3c")
	assert.Equal(t, string(users.StudentStatusActive), pStatus)

	gClass, gStatus = readStudent(graduating.ID)
	assert.Equal(t, upper, gClass)
	assert.Equal(t, string(users.StudentStatusActive), gStatus, "graduated child reactivated")
}

func TestGradeTransitionService_Revert_WithGraduatedStudents(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-revert-grad@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("revert restores graduated students to active", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		graduateClass := fmt.Sprintf("4revert-%s", suffix)

		// Create student to be graduated (soft-deactivated, restorable)
		student := testpkg.CreateTestStudent(t, db, "GradRevert", "Student", graduateClass)
		defer testpkg.CleanupActivityFixtures(t, db, student.ID)

		// Create and apply transition with graduate
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, graduateClass, nil)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		_, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)

		// Revert - graduates are restored to active (soft delete is reversible)
		result, err := service.Revert(ctx, transition.ID, account.ID)
		require.NoError(t, err)
		assert.Equal(t, education.TransitionStatusReverted, result.Status)
		assert.Equal(t, 1, result.StudentsGraduated, "revert should count restored graduates")

		// No unrecoverable-graduates warning anymore
		for _, w := range result.Warnings {
			assert.NotContains(t, w, "cannot be restored")
		}

		// Verify student status is back to active
		var status string
		err = db.NewSelect().
			TableExpr(`users.students`).
			Column("status").
			Where("id = ?", student.ID).
			Scan(ctx, &status)
		require.NoError(t, err)
		assert.Equal(t, string(users.StudentStatusActive), status)
	})
}

// recordingRosterReconciler captures the student ids each reconciliation pass
// is handed, so a test can pin WHICH children a revert puts back on the
// timetable — not merely that it called the reconciler.
type recordingRosterReconciler struct {
	removed  []int64
	restored []int64
}

func (r *recordingRosterReconciler) RemoveStudentsFromFutureRosters(
	_ context.Context, _ int64, studentIDs []int64,
) error {
	r.removed = append(r.removed, studentIDs...)
	return nil
}

func (r *recordingRosterReconciler) RestoreStudentsToFutureRosters(
	_ context.Context, _ int64, studentIDs []int64, _ *int64,
) error {
	r.restored = append(r.restored, studentIDs...)
	return nil
}

func (r *recordingRosterReconciler) CurrentRosterBaseline(_ context.Context) (int64, error) {
	return 0, nil
}

// A graduate whose lifecycle status was changed by hand after the apply is
// deliberately NOT reactivated by the revert (the UPDATE only matches rows still
// in alumnus status) and is reported as a warning. Roster reconciliation must
// follow that decision: replaying the archive for such a child would put an
// inactive student back on upcoming timetables — reverting half of a change the
// admin never asked to revert (#405 review).
func TestGradeTransitionService_Revert_ReconcilesOnlyReactivatedStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	reconciler := &recordingRosterReconciler{}
	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo:   educationRepo.NewGradeTransitionRepository(db),
		StudentRepo:      usersRepo.NewStudentRepository(db),
		PersonRepo:       usersRepo.NewPersonRepository(db),
		RosterReconciler: reconciler,
		DB:               db,
	})

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-revert-partial@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	graduateClass := fmt.Sprintf("4partial-%s", suffix)

	restorable := testpkg.CreateTestStudent(t, db, "Zurueck", "Kommt", graduateClass)
	handChanged := testpkg.CreateTestStudent(t, db, "Bleibt", "Weg", graduateClass)
	defer testpkg.CleanupActivityFixtures(t, db, restorable.ID, handChanged.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, graduateClass, nil)
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{restorable.ID, handChanged.ID}, reconciler.removed,
		"both graduates leave the future rosters on apply")

	// An admin decides one departed child is simply inactive, not an alumnus.
	_, err = db.NewRaw(`UPDATE users.students SET status = 'inactive' WHERE id = ?`, handChanged.ID).
		Exec(ctx)
	require.NoError(t, err)

	result, err := service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.StudentsGraduated, "only the still-alumnus child is restored")
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "could not be restored")

	assert.Equal(t, []int64{restorable.ID}, reconciler.restored,
		"the hand-changed child must not be replayed onto future rosters")

	var status string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", handChanged.ID).Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusInactive), status,
		"the manual status decision survives the revert")
}

// A graduate whose recorded from_status was pending or inactive IS restored by
// the revert — to exactly that status — but must NOT be replayed onto future
// rosters: being off actionable rosters is what those lifecycle states mean,
// and the apply's roster removal is the correct end state for them (#405
// review).
func TestGradeTransitionService_Revert_SkipsRosterReplayForNonActiveRestores(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	reconciler := &recordingRosterReconciler{}
	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo:   educationRepo.NewGradeTransitionRepository(db),
		StudentRepo:      usersRepo.NewStudentRepository(db),
		PersonRepo:       usersRepo.NewPersonRepository(db),
		RosterReconciler: reconciler,
		DB:               db,
	})

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-revert-pending@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	graduateClass := fmt.Sprintf("4pend-%s", suffix)

	activeChild := testpkg.CreateTestStudent(t, db, "Aktiv", "Zurueck", graduateClass)
	pendingChild := testpkg.CreateTestStudent(t, db, "Wartet", "Noch", graduateClass)
	defer testpkg.CleanupActivityFixtures(t, db, activeChild.ID, pendingChild.ID)

	// A future enrollment that never started: the child graduates with
	// from_status = pending and must come back as exactly that.
	_, err := db.NewRaw(`UPDATE users.students SET status = 'pending' WHERE id = ?`, pendingChild.ID).
		Exec(ctx)
	require.NoError(t, err)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, graduateClass, nil)
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err = service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	result, err := service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, result.StudentsGraduated, "both graduates are restored")
	assert.Empty(t, result.Warnings)

	assert.Equal(t, []int64{activeChild.ID}, reconciler.restored,
		"only the child restored as active is replayed onto future rosters")

	var status string
	require.NoError(t, db.NewSelect().TableExpr(`users.students`).Column("status").
		Where("id = ?", pendingChild.ID).Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusPending), status,
		"the pending child returns to pending, not active")
}

func TestGradeTransitionService_Preview_NoMappings(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-preview-none@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("preview with no mappings shows zero totals", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		preview, err := service.Preview(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, 0, preview.TotalStudents)
		assert.Equal(t, 0, preview.ToPromote)
		assert.Equal(t, 0, preview.ToGraduate)
		assert.Empty(t, preview.ByMapping)
	})
}

func TestGradeTransitionService_List_NilOptions(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-list-nil@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("list with nil options", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		transitions, total, err := service.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)
		assert.NotEmpty(t, transitions)
	})
}

func TestGradeTransitionService_Update_ClearMappings(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-clear-map@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	t.Run("update with empty mappings clears existing", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, "1a", testpkg.StrPtr("2a"))
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		// Verify mapping exists
		initial, err := service.GetByID(ctx, transition.ID)
		require.NoError(t, err)
		assert.Len(t, initial.Mappings, 1)

		// Update with empty mappings
		emptyMappings := []educationService.MappingRequest{}
		req := educationService.UpdateTransitionRequest{
			Mappings: emptyMappings,
		}

		updated, err := service.Update(ctx, transition.ID, req)
		require.NoError(t, err)
		assert.Empty(t, updated.Mappings)
	})
}

// TestGradeTransitionService_AlumniExcluded verifies that students already
// marked as alumnus (graduated in a previous transition) are invisible to
// preview counts, suggestions, and a subsequent apply — otherwise every next
// school year would re-count last year's leavers.
func TestGradeTransitionService_AlumniExcluded(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-alumni-excl@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class := fmt.Sprintf("3alum-%s", suffix)

	active := testpkg.CreateTestStudent(t, db, "Active", "Kid", class)
	alumnus := testpkg.CreateTestStudent(t, db, "Former", "Kid", class)
	defer testpkg.CleanupActivityFixtures(t, db, active.ID, alumnus.ID)

	// Mark one student as alumnus directly (as a previous transition would)
	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(users.StudentStatusAlumnus)).
		Where("id = ?", alumnus.ID).
		Exec(ctx)
	require.NoError(t, err)

	t.Run("preview counts exclude alumni", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class, nil)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		preview, err := service.Preview(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, preview.ToGraduate, "alumnus must not be counted")
		assert.Equal(t, 1, preview.TotalStudents)
	})

	t.Run("suggestions exclude alumni from counts", func(t *testing.T) {
		suggestions, err := service.SuggestMappings(ctx)
		require.NoError(t, err)
		for _, s := range suggestions {
			if s.FromClass == class {
				assert.Equal(t, 1, s.StudentCount, "alumnus must not be counted in suggestion")
			}
		}
	})

	t.Run("apply graduates only non-alumni", func(t *testing.T) {
		transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class, nil)
		defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

		result, err := service.Apply(ctx, transition.ID, account.ID)
		require.NoError(t, err)
		assert.Equal(t, 1, result.StudentsGraduated, "only the active student graduates")
	})
}

// TestGradeTransitionService_PromotionSkipsAlumni verifies the bulk promotion
// UPDATE does not drag alumni into the next class.
func TestGradeTransitionService_PromotionSkipsAlumni(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-alumni-promo@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	fromClass := fmt.Sprintf("2promo-%s", suffix)
	toClass := fmt.Sprintf("3promo-%s", suffix)

	alumnus := testpkg.CreateTestStudent(t, db, "FormerPromo", "Kid", fromClass)
	defer testpkg.CleanupActivityFixtures(t, db, alumnus.ID)

	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(users.StudentStatusAlumnus)).
		Where("id = ?", alumnus.ID).
		Exec(ctx)
	require.NoError(t, err)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, &toClass)
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	result, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, result.StudentsPromoted)

	// Alumnus keeps the old class name
	var currentClass string
	err = db.NewSelect().
		TableExpr(`users.students`).
		Column("school_class").
		Where("id = ?", alumnus.ID).
		Scan(ctx, &currentClass)
	require.NoError(t, err)
	assert.Equal(t, fromClass, currentClass)
}
