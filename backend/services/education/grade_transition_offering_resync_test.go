package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orderRecordingReconciler and the resync closure below append to one shared
// log so the test can pin the archive/resync ordering (#2147 review round 16):
// the offering-source resync deletes sourced still-planned rows of the
// graduates WITHOUT archiving, so the archive pass must run first on apply —
// and on revert the archive replay must run first, so the resync finds the
// replayed rows (room, note, non-booking markers intact) and retains them
// instead of recreating plain expected rows.
type orderRecordingReconciler struct {
	log *[]string
}

func (r *orderRecordingReconciler) RemoveStudentsFromFutureRosters(_ context.Context, _ int64, _ []int64) error {
	*r.log = append(*r.log, "archive")
	return nil
}

func (r *orderRecordingReconciler) RestoreStudentsToFutureRosters(_ context.Context, _ int64, _ []int64, _ *int64) error {
	*r.log = append(*r.log, "restore")
	return nil
}

func (r *orderRecordingReconciler) CurrentRosterBaseline(_ context.Context) (int64, error) {
	*r.log = append(*r.log, "baseline")
	return 0, nil
}

func TestGradeTransitionWorkflow_ApplyAndRevert_ArchiveBracketsOfferingResync(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	log := make([]string, 0, 5)
	f := newTransitionFixture(t, db)
	f.deps.Rosters = &orderRecordingReconciler{log: &log}
	f.deps.ResyncOfferingRosters = func(_ context.Context, _ string) error {
		log = append(log, "resync")
		return nil
	}
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4order-%s", suffix)

	testpkg.CreateTestStudent(t, db, "Order", "Child", gradClass)

	transitionID := f.createDraft(t, ctx, "2025-2026", graduate(gradClass))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"archive", "resync", "baseline"}, log,
		"apply must archive the graduates' planned rows BEFORE the offering-source resync deletes them unarchived")

	log = log[:0]
	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)
	assert.Equal(t, []string{"restore", "resync"}, log,
		"revert must replay the archived rows BEFORE the resync, so it retains them instead of recreating plain expected rows")
}

// TestGradeTransitionWorkflow_ApplyAndRevert_ResyncOfferingSourcedRosters pins
// the resync itself (#2137 review): promotions rewrite school classes, so apply
// AND revert must re-reconcile the Jahrgang-filtered sourced rosters from the
// workflow's calendar day.
func TestGradeTransitionWorkflow_ApplyAndRevert_ResyncOfferingSourcedRosters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	fromClass := fmt.Sprintf("2resync-%s", suffix)
	toClass := fmt.Sprintf("3resync-%s", suffix)

	testpkg.CreateTestStudent(t, db, "Resync", "Child", fromClass)

	transitionID := f.createDraft(t, ctx, "2025-2026", promote(fromClass, toClass))

	_, err := wf.Apply(ctx, transitionID, "")
	require.NoError(t, err)
	require.Len(t, f.resyncCalls, 1,
		"apply must resync offering-sourced rosters after rewriting school classes")
	assert.Equal(t, timezone.NewDate(2026, 8, 24), f.resyncCalls[0])

	_, err = wf.Revert(ctx, transitionID)
	require.NoError(t, err)
	require.Len(t, f.resyncCalls, 2,
		"revert must resync in the opposite direction")
	assert.Equal(t, timezone.NewDate(2026, 8, 24), f.resyncCalls[1])
}
