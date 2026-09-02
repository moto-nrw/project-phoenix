package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetHistoryWithStudentStates pins the classification the Abgänge view is
// built on. The ledger is append-only, so every one of these children reads as
// "graduated" in it — only the resolved state says which actions are still
// possible, and getting that wrong would offer "endgültig löschen" for a child
// a revert already brought back.
func TestGetHistoryWithStudentStates(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-states@test.local")

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	graduateClass := fmt.Sprintf("4a-%s", suffix)

	stillAlumnus := testpkg.CreateTestStudent(t, db, "State", "Alumnus", graduateClass)
	restored := testpkg.CreateTestStudent(t, db, "State", "Restored", graduateClass)
	purged := testpkg.CreateTestStudent(t, db, "State", "Purged", graduateClass)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, graduateClass, nil)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	// Put one child back the way a revert does, and remove another the way the
	// purge route does — the two states the ledger cannot express.
	repo := newGradeTransitionRepository(t, db)
	reactivated, err := repo.ReactivateStudentsToStatus(ctx, []int64{restored.ID}, string(users.StudentStatusActive))
	require.NoError(t, err)
	require.Equal(t, []int64{restored.ID}, reactivated)

	_, err = db.NewDelete().
		Model((*struct{})(nil)).
		ModelTableExpr("users.students").
		Where("id = ?", purged.ID).
		Exec(ctx)
	require.NoError(t, err)

	history, states, err := service.GetHistoryWithStudentStates(ctx, transition.ID)
	require.NoError(t, err)
	require.NotEmpty(t, history)

	assert.Equal(t, educationService.GraduateStateAlumnus, states[stillAlumnus.ID],
		"a child still soft-deleted must stay revertable and purgeable")
	assert.Equal(t, educationService.GraduateStateRestored, states[restored.ID],
		"a reverted child must not be offered for endgültiges Löschen")
	assert.Equal(t, educationService.GraduateStatePurged, states[purged.ID],
		"a child with no row left is purged, not merely graduated")
}

// TestAnonymizePurgedGraduate covers the half of the deletion that has no
// foreign key to enforce it: grade_transition_history.person_name is a
// denormalized copy that outlives both the student and the person row, so
// without this step "endgültig löschen" would leave the name in the database.
func TestAnonymizePurgedGraduate(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-anonymize@test.local")

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	graduateClass := fmt.Sprintf("4b-%s", suffix)

	student := testpkg.CreateTestStudent(t, db, "Anonymize", "Candidate", graduateClass)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, graduateClass, nil)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	t.Run("refuses while the student row still exists", func(t *testing.T) {
		err := service.AnonymizePurgedGraduate(ctx, student.ID)
		require.ErrorIs(t, err, educationService.ErrGraduateStillPresent,
			"blanking the name of a child a revert can still restore breaks both the revert and the Abgänge list")

		_, states, err := service.GetHistoryWithStudentStates(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, educationService.GraduateStateAlumnus, states[student.ID])
	})

	t.Run("anonymizes once the row is gone", func(t *testing.T) {
		_, err := db.NewDelete().
			Model((*struct{})(nil)).
			ModelTableExpr("users.students").
			Where("id = ?", student.ID).
			Exec(ctx)
		require.NoError(t, err)

		require.NoError(t, service.AnonymizePurgedGraduate(ctx, student.ID))

		history, states, err := service.GetHistoryWithStudentStates(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, educationService.GraduateStatePurged, states[student.ID])

		var found bool
		for _, h := range history {
			if h.StudentID != student.ID {
				continue
			}
			found = true
			assert.Equal(t, educationRepo.PurgedStudentPlaceholder, h.PersonName,
				"the ledger must not keep naming a child whose deletion was reported as endgültig")
			assert.NotContains(t, h.PersonName, "Candidate")
		}
		assert.True(t, found, "the ledger row itself must survive so the transition's Abgänge count still adds up")
	})
}
