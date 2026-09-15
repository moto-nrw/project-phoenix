package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
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
