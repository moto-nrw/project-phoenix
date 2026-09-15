package education_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deletedTransitionID returns the id of a draft that no longer exists, so the
// not-found paths are exercised against a real id the tenant once had.
func deletedTransitionID(t *testing.T, ctx context.Context, f *transitionFixture, wf *gradetransition.Workflow) int64 {
	t.Helper()
	id := f.createDraft(t, ctx, "2025-2026")
	require.NoError(t, wf.DeleteDraft(ctx, id))
	return id
}

func TestGradeTransitionWorkflow_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("create transition without mappings", func(t *testing.T) {
		transition, err := wf.CreateDraft(ctx, gradetransition.Draft{AcademicYear: "2025-2026"})
		require.NoError(t, err)

		assert.Equal(t, "2025-2026", transition.AcademicYear)
		assert.Equal(t, schoolstructure.TransitionStatusDraft, transition.Status)
		assert.Equal(t, f.actorID, transition.CreatedBy)
		assert.Empty(t, transition.Mappings)
	})

	t.Run("create transition with mappings", func(t *testing.T) {
		transition, err := wf.CreateDraft(ctx, gradetransition.Draft{
			AcademicYear: "2026-2027",
			Mappings:     []gradetransition.Mapping{promote("1a", "2a"), graduate("4a")},
		})
		require.NoError(t, err)

		assert.Equal(t, "2026-2027", transition.AcademicYear)
		assert.Len(t, transition.Mappings, 2)
	})

	t.Run("create transition with notes", func(t *testing.T) {
		notes := "Test notes for transition"
		transition, err := wf.CreateDraft(ctx, gradetransition.Draft{AcademicYear: "2027-2028", Notes: &notes})
		require.NoError(t, err)

		require.NotNil(t, transition.Notes)
		assert.Equal(t, notes, *transition.Notes)
	})

	t.Run("create transition fails with empty academic year", func(t *testing.T) {
		_, err := wf.CreateDraft(ctx, gradetransition.Draft{})
		require.ErrorIs(t, err, gradetransition.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "academic_year is required")
	})

	t.Run("create transition fails with invalid mapping", func(t *testing.T) {
		_, err := wf.CreateDraft(ctx, gradetransition.Draft{
			AcademicYear: "2028-2029",
			Mappings:     []gradetransition.Mapping{promote("1a", "1a")}, // same class
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be the same")
	})
}

func TestGradeTransitionWorkflow_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("update academic year", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026")

		newYear := "2026-2027"
		updated, err := wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{AcademicYear: &newYear})
		require.NoError(t, err)
		assert.Equal(t, "2026-2027", updated.AcademicYear)
	})

	t.Run("update notes", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026")

		notes := "Updated notes"
		updated, err := wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{Notes: &notes})
		require.NoError(t, err)
		require.NotNil(t, updated.Notes)
		assert.Equal(t, "Updated notes", *updated.Notes)
	})

	t.Run("update mappings", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026", promote("1a", "2a"))

		updated, err := wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{
			Mappings: []gradetransition.Mapping{promote("2a", "3a"), promote("3a", "4a")},
		})
		require.NoError(t, err)
		assert.Len(t, updated.Mappings, 2)
	})

	t.Run("cannot update applied transition", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		id := f.createDraft(t, ctx, "2025-2026",
			promote(fmt.Sprintf("1upd-%s", suffix), fmt.Sprintf("2upd-%s", suffix)))
		_, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)

		newYear := "2026-2027"
		_, err = wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{AcademicYear: &newYear})
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotDraft)
		assert.Contains(t, err.Error(), "cannot modify transition")
	})

	t.Run("update non-existent transition", func(t *testing.T) {
		_, err := wf.UpdateDraft(ctx, deletedTransitionID(t, ctx, f, wf), gradetransition.DraftPatch{})
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionWorkflow_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("delete draft transition", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026")

		require.NoError(t, wf.DeleteDraft(ctx, id))

		// Verify deletion
		_, err := wf.FindTransition(ctx, id)
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
	})

	t.Run("cannot delete applied transition", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		id := f.createDraft(t, ctx, "2025-2026",
			promote(fmt.Sprintf("1del-%s", suffix), fmt.Sprintf("2del-%s", suffix)))
		_, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)

		err = wf.DeleteDraft(ctx, id)
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotDraft)
		assert.Contains(t, err.Error(), "cannot delete transition")
	})

	t.Run("delete non-existent transition", func(t *testing.T) {
		err := wf.DeleteDraft(ctx, deletedTransitionID(t, ctx, f, wf))
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionWorkflow_Get(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("get transition with mappings", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026", promote("1a", "2a"), promote("2a", "3a"))

		result, err := wf.FindTransition(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, result.ID)
		assert.Equal(t, "2025-2026", result.AcademicYear)
		assert.Len(t, result.Mappings, 2)
	})

	t.Run("get non-existent transition", func(t *testing.T) {
		_, err := wf.FindTransition(ctx, deletedTransitionID(t, ctx, f, wf))
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
	})
}

func TestGradeTransitionWorkflow_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("list transitions with pagination", func(t *testing.T) {
		// Create multiple transitions
		f.createDraft(t, ctx, "2025-2026")
		f.createDraft(t, ctx, "2026-2027")
		f.createDraft(t, ctx, "2027-2028")

		transitions, total, err := wf.ListTransitions(ctx, gradetransition.ListFilter{Page: 1, PageSize: 2})
		require.NoError(t, err)
		assert.Len(t, transitions, 2)
		assert.GreaterOrEqual(t, total, 3)
	})

	t.Run("list transitions with filter", func(t *testing.T) {
		f.createDraft(t, ctx, "2029-2030")

		transitions, _, err := wf.ListTransitions(ctx, gradetransition.ListFilter{AcademicYear: "2029-2030"})
		require.NoError(t, err)
		require.NotEmpty(t, transitions)
		for _, tr := range transitions {
			assert.Equal(t, "2029-2030", tr.AcademicYear)
		}
	})
}

func TestGradeTransitionWorkflow_Preview(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("preview with students", func(t *testing.T) {
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		class1 := fmt.Sprintf("1a-%s", suffix)
		class2 := fmt.Sprintf("2a-%s", suffix)
		class4 := fmt.Sprintf("4a-%s", suffix)

		// Create students in specific classes
		testpkg.CreateTestStudent(t, db, "Preview", "Student1", class1)
		testpkg.CreateTestStudent(t, db, "Preview", "Student2", class1)
		testpkg.CreateTestStudent(t, db, "Preview", "Student3", class4)

		id := f.createDraft(t, ctx, "2025-2026", promote(class1, class2), graduate(class4))

		preview, err := wf.Preview(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, id, preview.TransitionID)
		assert.Equal(t, 3, preview.TotalStudents)
		assert.Equal(t, 2, preview.ToPromote)
		assert.Equal(t, 1, preview.ToGraduate)
		assert.Len(t, preview.ByMapping, 2)

		actions := make(map[string]string, len(preview.ByMapping))
		counts := make(map[string]int, len(preview.ByMapping))
		for _, mapping := range preview.ByMapping {
			actions[mapping.FromClass] = mapping.Action
			counts[mapping.FromClass] = mapping.StudentCount
		}
		assert.Equal(t, schoolstructure.TransitionActionPromoted, actions[class1])
		assert.Equal(t, schoolstructure.TransitionActionGraduated, actions[class4])
		assert.Equal(t, 2, counts[class1])
		assert.Equal(t, 1, counts[class4])
	})

	t.Run("preview shows unmapped classes", func(t *testing.T) {
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		unmappedClass := fmt.Sprintf("3b-%s", suffix)
		mappedClass := fmt.Sprintf("1a-%s", suffix)
		targetClass := fmt.Sprintf("2a-%s", suffix)

		// Create student in unmapped class
		testpkg.CreateTestStudent(t, db, "Unmapped", "Student", unmappedClass)

		// Create transition without mapping for unmappedClass
		id := f.createDraft(t, ctx, "2025-2026", promote(mappedClass, targetClass))

		preview, err := wf.Preview(ctx, id)
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
		_, err := wf.Preview(ctx, deletedTransitionID(t, ctx, f, wf))
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionWorkflow_Apply(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("apply transition promotes students", func(t *testing.T) {
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1a-%s", suffix)
		toClass := fmt.Sprintf("2a-%s", suffix)

		// Create students in fromClass
		student1 := testpkg.CreateTestStudent(t, db, "Apply", "Student1", fromClass)
		testpkg.CreateTestStudent(t, db, "Apply", "Student2", fromClass)

		id := f.createDraft(t, ctx, "2025-2026", promote(fromClass, toClass))

		result, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)
		assert.Equal(t, schoolstructure.TransitionStatusApplied, result.Status)
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

		id := f.createDraft(t, ctx, "2025-2026", promote(fromClass, toClass))

		_, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)

		// Verify history was created
		history, err := f.deps.Structure.ListTransitionHistory(ctx, id)
		require.NoError(t, err)
		assert.NotEmpty(t, history)

		found := false
		for _, h := range history {
			if h.StudentID == student.ID {
				assert.Equal(t, fromClass, h.FromClass)
				assert.NotNil(t, h.ToClass)
				assert.Equal(t, toClass, *h.ToClass)
				assert.Equal(t, schoolstructure.TransitionActionPromoted, h.Action)
				found = true
				break
			}
		}
		assert.True(t, found, "Expected history record for student")
	})

	t.Run("cannot apply already applied transition", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		id := f.createDraft(t, ctx, "2025-2026",
			promote(fmt.Sprintf("9z-%s", suffix), fmt.Sprintf("10z-%s", suffix)))

		// First apply
		_, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)

		// Second apply should fail
		_, err = wf.Apply(ctx, id, "")
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotDraft)
		assert.Contains(t, err.Error(), "already been applied")
	})

	t.Run("cannot apply transition without mappings", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026")

		_, err := wf.Apply(ctx, id, "")
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotDraft)
		assert.Contains(t, err.Error(), "must be in draft status with mappings")
	})
}

func TestGradeTransitionWorkflow_Revert(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("revert applied transition", func(t *testing.T) {
		// Create unique class names to ensure test isolation
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1c-%s", suffix)
		toClass := fmt.Sprintf("2c-%s", suffix)

		// Create students
		student := testpkg.CreateTestStudent(t, db, "Revert", "Student", fromClass)

		// Create and apply transition
		id := f.createDraft(t, ctx, "2025-2026", promote(fromClass, toClass))

		_, err := wf.Apply(ctx, id, "")
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
		result, err := wf.Revert(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, schoolstructure.TransitionStatusReverted, result.Status)
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
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		id := f.createDraft(t, ctx, "2025-2026",
			promote(fmt.Sprintf("5x-%s", suffix), fmt.Sprintf("6x-%s", suffix)))

		_, err := wf.Revert(ctx, id)
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotApplied)
		assert.Contains(t, err.Error(), "has not been applied yet")
	})

	t.Run("cannot revert already reverted transition", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		id := f.createDraft(t, ctx, "2025-2026",
			promote(fmt.Sprintf("6y-%s", suffix), fmt.Sprintf("7y-%s", suffix)))

		// Apply then revert
		_, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)
		_, err = wf.Revert(ctx, id)
		require.NoError(t, err)

		// Try to revert again
		_, err = wf.Revert(ctx, id)
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotApplied)
		assert.Contains(t, err.Error(), "already been reverted")
	})
}

func TestGradeTransitionWorkflow_SuggestMappings(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("suggests promotion for lower grades", func(t *testing.T) {
		testpkg.CreateTestStudent(t, db, "Suggest", "Student", "1a")

		suggestions, err := wf.SuggestMappings(ctx)
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
		testpkg.CreateTestStudent(t, db, "Graduate", "Student", "4b")

		suggestions, err := wf.SuggestMappings(ctx)
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
		testpkg.CreateTestStudent(t, db, "Prefixed", "Student", "Klasse 1a")

		suggestions, err := wf.SuggestMappings(ctx)
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
		testpkg.CreateTestStudent(t, db, "PrefixedGrad", "Student", "Klasse 4b")

		suggestions, err := wf.SuggestMappings(ctx)
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
		testpkg.CreateTestStudent(t, db, "DigitOnly", "Student", "2")

		suggestions, err := wf.SuggestMappings(ctx)
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
		testpkg.CreateTestStudent(t, db, "NonStd", "Student", "special")

		suggestions, err := wf.SuggestMappings(ctx)
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

func TestGradeTransitionWorkflow_ListClasses(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("returns distinct classes", func(t *testing.T) {
		// Create students in different classes
		testpkg.CreateTestStudent(t, db, "Class", "Test1", "ClassA")
		testpkg.CreateTestStudent(t, db, "Class", "Test2", "ClassA") // duplicate class
		testpkg.CreateTestStudent(t, db, "Class", "Test3", "ClassB")

		classes, err := wf.ListClasses(ctx)
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

func TestGradeTransitionWorkflow_History(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("returns history after apply", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1d-%s", suffix)
		toClass := fmt.Sprintf("2d-%s", suffix)

		student := testpkg.CreateTestStudent(t, db, "History", "Test", fromClass)

		id := f.createDraft(t, ctx, "2025-2026", promote(fromClass, toClass))

		// Apply transition
		_, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)

		// Get history
		history, err := f.deps.Structure.ListTransitionHistory(ctx, id)
		require.NoError(t, err)
		assert.NotEmpty(t, history)

		// Verify history record
		var found bool
		for _, h := range history {
			if h.StudentID == student.ID {
				found = true
				assert.Equal(t, id, h.TransitionID)
				assert.Equal(t, fromClass, h.FromClass)
				assert.NotNil(t, h.ToClass)
				assert.Equal(t, toClass, *h.ToClass)
				assert.Contains(t, h.PersonName, "History")
			}
		}
		assert.True(t, found, "Expected history for student")
	})

	t.Run("empty history for transition without apply", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026")

		history, err := f.deps.Structure.ListTransitionHistory(ctx, id)
		require.NoError(t, err)
		assert.Empty(t, history)
	})
}

// ============================================================================
// Additional Edge Case Tests for the workflow
// ============================================================================

func TestGradeTransitionWorkflow_Apply_RevertedTransition(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("cannot apply reverted transition", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1z-%s", suffix)
		toClass := fmt.Sprintf("2z-%s", suffix)

		id := f.createDraft(t, ctx, "2025-2026", promote(fromClass, toClass))

		// Apply then revert
		_, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)
		_, err = wf.Revert(ctx, id)
		require.NoError(t, err)

		// Try to apply again - should fail
		_, err = wf.Apply(ctx, id, "")
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotDraft)
		assert.Contains(t, err.Error(), "has been reverted")
	})
}

func TestGradeTransitionWorkflow_Create_InvalidAcademicYearFormat(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("create fails with invalid academic year format", func(t *testing.T) {
		_, err := wf.CreateDraft(ctx, gradetransition.Draft{AcademicYear: "invalid-year"})
		require.ErrorIs(t, err, gradetransition.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "format")
	})

	t.Run("create fails with partial academic year", func(t *testing.T) {
		_, err := wf.CreateDraft(ctx, gradetransition.Draft{AcademicYear: "2025"})
		require.ErrorIs(t, err, gradetransition.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "format")
	})
}

func TestGradeTransitionWorkflow_Update_InvalidAcademicYearFormat(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("update fails with invalid academic year format", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026")

		invalidYear := "bad-format"
		_, err := wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{AcademicYear: &invalidYear})
		require.ErrorIs(t, err, gradetransition.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "format")
	})
}

func TestGradeTransitionWorkflow_Update_InvalidMapping(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("update fails with invalid mapping (same from and to)", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026", promote("1a", "2a"))

		notes := "must not be saved"
		_, err := wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{
			Notes:    &notes,
			Mappings: []gradetransition.Mapping{promote("1a", "1a")},
		})
		require.ErrorIs(t, err, gradetransition.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "cannot be the same")

		unchanged, err := wf.FindTransition(ctx, id)
		require.NoError(t, err)
		assert.Nil(t, unchanged.Notes)
		require.Len(t, unchanged.Mappings, 1)
		assert.Equal(t, "1a", unchanged.Mappings[0].FromClass)
		require.NotNil(t, unchanged.Mappings[0].ToClass)
		assert.Equal(t, "2a", *unchanged.Mappings[0].ToClass)
	})

	t.Run("update fails with empty from_class", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026")

		_, err := wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{
			Mappings: []gradetransition.Mapping{promote("", "2a")},
		})
		require.ErrorIs(t, err, gradetransition.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "from_class")
	})
}

func TestGradeTransitionWorkflow_Create_InvalidMapping(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("create fails with empty from_class", func(t *testing.T) {
		academicYear := "2025-2026"
		_, err := wf.CreateDraft(ctx, gradetransition.Draft{
			AcademicYear: academicYear,
			Mappings:     []gradetransition.Mapping{promote("", "2a")},
		})
		require.ErrorIs(t, err, gradetransition.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "from_class")

		count, err := db.NewSelect().
			Model((*education.GradeTransition)(nil)).
			Where("academic_year = ?", academicYear).
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Count(ctx)
		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("create fails with duplicate source classes", func(t *testing.T) {
		academicYear := "2026-2027"
		_, err := wf.CreateDraft(ctx, gradetransition.Draft{
			AcademicYear: academicYear,
			Mappings:     []gradetransition.Mapping{promote("1a", "2a"), promote(" 1a ", "2b")},
		})
		require.ErrorIs(t, err, gradetransition.ErrInvalidTransitionData)
		assert.Contains(t, err.Error(), "duplicate mapping")

		count, err := db.NewSelect().
			Model((*education.GradeTransition)(nil)).
			Where("academic_year = ?", academicYear).
			Where("tenant_id = ?", testpkg.Tenant(t)).
			Count(ctx)
		require.NoError(t, err)
		assert.Zero(t, count)
	})
}

func TestGradeTransitionWorkflow_Revert_NonExistentTransition(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("revert non-existent transition", func(t *testing.T) {
		_, err := wf.Revert(ctx, deletedTransitionID(t, ctx, f, wf))
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionWorkflow_Apply_NonExistentTransition(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("apply non-existent transition", func(t *testing.T) {
		_, err := wf.Apply(ctx, deletedTransitionID(t, ctx, f, wf), "")
		require.ErrorIs(t, err, gradetransition.ErrTransitionNotFound)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestGradeTransitionWorkflow_SuggestMappings_EmptyResult(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("suggest mappings returns sorted results", func(t *testing.T) {
		suggestions, err := wf.SuggestMappings(ctx)
		require.NoError(t, err)
		// Results should be sorted alphabetically by FromClass
		for i := 1; i < len(suggestions); i++ {
			assert.LessOrEqual(t, suggestions[i-1].FromClass, suggestions[i].FromClass)
		}
	})
}

func TestGradeTransitionWorkflow_Apply_GraduateStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("apply transition graduates students and creates warning", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		graduateClass := fmt.Sprintf("4grad-%s", suffix)

		// Create student to be graduated (soft-deactivated as alumnus)
		student := testpkg.CreateTestStudent(t, db, "Graduate", "Student", graduateClass)

		id := f.createDraft(t, ctx, "2025-2026", graduate(graduateClass))

		result, err := wf.Apply(ctx, id, "")
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

// TestGradeTransitionWorkflow_Apply_CascadingGraduation guards the ordering
// bug where a promotion moves students INTO a class that is graduated in the
// same transition. E.g. "3a -> 4a" (promote) together with "4a -> graduate":
// the 3a children must NOT be graduated just because they land in 4a. Only the
// original 4a members graduate; the promoted-in 3a children stay active.
func TestGradeTransitionWorkflow_Apply_CascadingGraduation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	lower := fmt.Sprintf("3c-%s", suffix) // promoted into `upper`
	upper := fmt.Sprintf("4c-%s", suffix) // graduated

	promoted := testpkg.CreateTestStudent(t, db, "Promoted", "Kid", lower)
	graduating := testpkg.CreateTestStudent(t, db, "Graduating", "Kid", upper)

	id := f.createDraft(t, ctx, "2025-2026",
		promote(lower, upper), // promote 3c -> 4c
		graduate(upper),       // graduate 4c
	)

	result, err := wf.Apply(ctx, id, "")
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
	_, err = wf.Revert(ctx, id)
	require.NoError(t, err)

	pClass, pStatus = readStudent(promoted.ID)
	assert.Equal(t, lower, pClass, "promoted child returns to 3c")
	assert.Equal(t, string(users.StudentStatusActive), pStatus)

	gClass, gStatus = readStudent(graduating.ID)
	assert.Equal(t, upper, gClass)
	assert.Equal(t, string(users.StudentStatusActive), gStatus, "graduated child reactivated")
}

func TestGradeTransitionWorkflow_Revert_WithGraduatedStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("revert restores graduated students to active", func(t *testing.T) {
		// Create unique class names
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		graduateClass := fmt.Sprintf("4revert-%s", suffix)

		// Create student to be graduated (soft-deactivated, restorable)
		student := testpkg.CreateTestStudent(t, db, "GradRevert", "Student", graduateClass)

		// Create and apply transition with graduate
		id := f.createDraft(t, ctx, "2025-2026", graduate(graduateClass))

		_, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)

		// Revert - graduates are restored to active (soft delete is reversible)
		result, err := wf.Revert(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, schoolstructure.TransitionStatusReverted, result.Status)
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
func TestGradeTransitionWorkflow_Revert_ReconcilesOnlyReactivatedStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	reconciler := &recordingRosterReconciler{}
	f.deps.Rosters = reconciler
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	graduateClass := fmt.Sprintf("4partial-%s", suffix)

	restorable := testpkg.CreateTestStudent(t, db, "Zurueck", "Kommt", graduateClass)
	handChanged := testpkg.CreateTestStudent(t, db, "Bleibt", "Weg", graduateClass)

	id := f.createDraft(t, ctx, "2025-2026", graduate(graduateClass))

	_, err := wf.Apply(ctx, id, "")
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{restorable.ID, handChanged.ID}, reconciler.removed,
		"both graduates leave the future rosters on apply")

	// An admin decides one departed child is simply inactive, not an alumnus.
	_, err = db.NewRaw(`UPDATE users.students SET status = 'inactive' WHERE id = ?`, handChanged.ID).
		Exec(ctx)
	require.NoError(t, err)

	result, err := wf.Revert(ctx, id)
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
func TestGradeTransitionWorkflow_Revert_SkipsRosterReplayForNonActiveRestores(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	reconciler := &recordingRosterReconciler{}
	f.deps.Rosters = reconciler
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	graduateClass := fmt.Sprintf("4pend-%s", suffix)

	activeChild := testpkg.CreateTestStudent(t, db, "Aktiv", "Zurueck", graduateClass)
	pendingChild := testpkg.CreateTestStudent(t, db, "Wartet", "Noch", graduateClass)

	// A future enrollment that never started: the child graduates with
	// from_status = pending and must come back as exactly that.
	_, err := db.NewRaw(`UPDATE users.students SET status = 'pending' WHERE id = ?`, pendingChild.ID).
		Exec(ctx)
	require.NoError(t, err)

	id := f.createDraft(t, ctx, "2025-2026", graduate(graduateClass))

	_, err = wf.Apply(ctx, id, "")
	require.NoError(t, err)

	result, err := wf.Revert(ctx, id)
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

func TestGradeTransitionWorkflow_Preview_NoMappings(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("preview with no mappings shows zero totals", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026")

		preview, err := wf.Preview(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 0, preview.TotalStudents)
		assert.Equal(t, 0, preview.ToPromote)
		assert.Equal(t, 0, preview.ToGraduate)
		assert.Empty(t, preview.ByMapping)
	})
}

func TestGradeTransitionWorkflow_List_ZeroFilter(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("list with the zero filter", func(t *testing.T) {
		f.createDraft(t, ctx, "2025-2026")

		transitions, total, err := wf.ListTransitions(ctx, gradetransition.ListFilter{})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 1)
		assert.NotEmpty(t, transitions)
	})
}

func TestGradeTransitionWorkflow_Update_ClearMappings(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	t.Run("update with empty mappings clears existing", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026", promote("1a", "2a"))

		// Verify mapping exists
		initial, err := wf.FindTransition(ctx, id)
		require.NoError(t, err)
		assert.Len(t, initial.Mappings, 1)

		// Update with empty mappings
		updated, err := wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{
			Mappings: []gradetransition.Mapping{},
		})
		require.NoError(t, err)
		assert.Empty(t, updated.Mappings)
	})
}

// TestGradeTransitionWorkflow_AlumniExcluded verifies that students already
// marked as alumnus (graduated in a previous transition) are invisible to
// preview counts, suggestions, and a subsequent apply — otherwise every next
// school year would re-count last year's leavers.
func TestGradeTransitionWorkflow_AlumniExcluded(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class := fmt.Sprintf("3alum-%s", suffix)

	testpkg.CreateTestStudent(t, db, "Active", "Kid", class)
	alumnus := testpkg.CreateTestStudent(t, db, "Former", "Kid", class)

	// Mark one student as alumnus directly (as a previous transition would)
	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(users.StudentStatusAlumnus)).
		Where("id = ?", alumnus.ID).
		Exec(ctx)
	require.NoError(t, err)

	t.Run("preview counts exclude alumni", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026", graduate(class))

		preview, err := wf.Preview(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, 1, preview.ToGraduate, "alumnus must not be counted")
		assert.Equal(t, 1, preview.TotalStudents)
	})

	t.Run("suggestions exclude alumni from counts", func(t *testing.T) {
		suggestions, err := wf.SuggestMappings(ctx)
		require.NoError(t, err)
		for _, s := range suggestions {
			if s.FromClass == class {
				assert.Equal(t, 1, s.StudentCount, "alumnus must not be counted in suggestion")
			}
		}
	})

	t.Run("apply graduates only non-alumni", func(t *testing.T) {
		id := f.createDraft(t, ctx, "2025-2026", graduate(class))

		result, err := wf.Apply(ctx, id, "")
		require.NoError(t, err)
		assert.Equal(t, 1, result.StudentsGraduated, "only the active student graduates")
	})
}

// TestGradeTransitionWorkflow_PromotionSkipsAlumni verifies the bulk promotion
// UPDATE does not drag alumni into the next class.
func TestGradeTransitionWorkflow_PromotionSkipsAlumni(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 10*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	fromClass := fmt.Sprintf("2promo-%s", suffix)
	toClass := fmt.Sprintf("3promo-%s", suffix)

	alumnus := testpkg.CreateTestStudent(t, db, "FormerPromo", "Kid", fromClass)

	_, err := db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(users.StudentStatusAlumnus)).
		Where("id = ?", alumnus.ID).
		Exec(ctx)
	require.NoError(t, err)

	id := f.createDraft(t, ctx, "2025-2026", promote(fromClass, toClass))

	result, err := wf.Apply(ctx, id, "")
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
