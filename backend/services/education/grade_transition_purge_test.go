package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGradeTransitionWorkflow_History_StudentStates pins the classification the
// Abgänge view is built on. The ledger is append-only, so every one of these
// children reads as "graduated" in it — only the resolved state says which
// actions are still possible, and getting that wrong would offer "endgültig
// löschen" for a child a revert already brought back.
func TestGradeTransitionWorkflow_History_StudentStates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	graduateClass := fmt.Sprintf("4a-%s", suffix)

	stillAlumnus := testpkg.CreateTestStudent(t, db, "State", "Alumnus", graduateClass)
	restored := testpkg.CreateTestStudent(t, db, "State", "Restored", graduateClass)
	purged := testpkg.CreateTestStudent(t, db, "State", "Purged", graduateClass)

	id := f.createDraft(t, ctx, "2026-2027", graduate(graduateClass))

	_, err := wf.Apply(ctx, id, "")
	require.NoError(t, err)

	// Put one child back the way a revert does, and remove another the way the
	// purge route does — the two states the ledger cannot express.
	require.NoError(t, f.deps.UnitOfWork(ctx, func(txCtx context.Context) error {
		reactivated, err := f.deps.Membership.Reactivate(txCtx,
			[]int64{restored.ID}, string(users.StudentStatusActive), gradetransition.RevertChildQuotaCheck)
		if err != nil {
			return err
		}
		require.Equal(t, []int64{restored.ID}, reactivated)
		return nil
	}))

	_, err = db.NewDelete().
		Model((*struct{})(nil)).
		ModelTableExpr("users.student_profiles").
		Where("id = ?", purged.ID).
		Exec(ctx)
	require.NoError(t, err)

	history, err := wf.History(ctx, id)
	require.NoError(t, err)
	require.NotEmpty(t, history)

	states := make(map[int64]string, len(history))
	for _, entry := range history {
		states[entry.StudentID] = entry.StudentState
	}

	assert.Equal(t, gradetransition.GraduateStateAlumnus, states[stillAlumnus.ID],
		"a child still soft-deleted must stay revertable and purgeable")
	assert.Equal(t, gradetransition.GraduateStateRestored, states[restored.ID],
		"a reverted child must not be offered for endgültiges Löschen")
	assert.Equal(t, gradetransition.GraduateStatePurged, states[purged.ID],
		"a child with no row left is purged, not merely graduated")
}
