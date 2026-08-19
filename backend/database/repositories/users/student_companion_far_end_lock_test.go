package users_test

// Dropping a "läuft mit" edge is decided by reading the FAR child's other
// links: may this edge go, or is it that child's last answer to "mit wem geht
// es nach Hause"? Two writers can pass that read on each other's
// soon-to-be-deleted data — with A-B and C-B, both still see the other edge,
// both let their own go, and B ends up with an accompanied plan and no answer
// at all. The far end's row lock is what serializes them, and it has to be
// taken in the repository because enrollment approval, imports and the
// care-request flow write plans without ever passing the HTTP handler.
//
// Locking downward (a far end with a LOWER id than the child being edited)
// inverts the ascending order every companion writer follows, so it is taken
// without waiting and surfaces as the retriable ErrCompanionLockBusy.

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// holdStudentRowLock keeps the student's row locked from a second transaction
// for the rest of the test — "this child is being edited elsewhere".
func holdStudentRowLock(t *testing.T, db *bun.DB, studentID int64) {
	t.Helper()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	var locked int64
	require.NoError(t, tx.NewRaw(
		`SELECT id FROM users.students WHERE id = ? FOR UPDATE`, studentID,
	).Scan(context.Background(), &locked))
	require.Equal(t, studentID, locked)
}

// TestStudentRepository_Update_RefusesWhenFarEndLocked pins that a plan change
// which would drop a link waits for nobody it may not wait for: the far child
// is locked by another edit, so the update is refused as retriable — and, just
// as important, refuses BEFORE writing anything.
func TestStudentRepository_Update_RefusesWhenFarEndLocked(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	factory := repositories.NewFactory(db)

	// The companion is created FIRST so its id is the lower one — the direction
	// that cannot wait.
	companion := testpkg.CreateTestStudent(t, db, "FarEndLocked", "Companion", "1a")
	subject := testpkg.CreateTestStudent(t, db, "FarEndSubject", "Companion", "1a")
	require.Less(t, companion.ID, subject.ID, "fixture order no longer yields ascending ids")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupStudentCompanions(t, db, subject.ID, companion.ID)

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon", "tue")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon", "tue")
	require.NoError(t, factory.StudentCompanion.ReplaceForStudent(ctx, subject.ID, []*users.StudentCompanion{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
		newCompanionEdge(t, subject.ID, companion.ID, 2),
	}))

	holdStudentRowLock(t, db, companion.ID)

	// ACT — narrow the plan so the Tuesday edge loses its basis and has to be
	// dropped, which is what makes the far end's state relevant.
	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = users.AllowedDepartureModes{
		"mon": {users.DepartureAccompanied},
		"tue": {users.DepartureBus},
	}

	start := time.Now()
	err = factory.Student.Update(ctx, loaded)

	require.ErrorIs(t, err, users.ErrCompanionLockBusy)
	assert.Less(t, time.Since(start), 2*time.Second, "the downward lock must refuse, not queue")

	// Nothing was written: both edges survive and the plan is untouched, so the
	// retry the user is asked for starts from the same state.
	edges, err := factory.StudentCompanion.ListForStudent(ctx, subject.ID)
	require.NoError(t, err)
	assert.Len(t, edges, 2)

	stored, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	assert.Contains(t, stored.AllowedDepartureModes["tue"], users.DepartureAccompanied)
}

// TestStudentRepository_Update_UnaffectedWhenNoEdgeIsDropped guards the far more
// common path against the new locking: an update that drops no edge must not
// reach for the far end's row at all, or every widening plan change would start
// failing while a linked child happens to be open somewhere else.
func TestStudentRepository_Update_UnaffectedWhenNoEdgeIsDropped(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	factory := repositories.NewFactory(db)

	companion := testpkg.CreateTestStudent(t, db, "FarEndKept", "Companion", "1a")
	subject := testpkg.CreateTestStudent(t, db, "FarEndKeeper", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupStudentCompanions(t, db, subject.ID, companion.ID)

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon")
	require.NoError(t, factory.StudentCompanion.ReplaceForStudent(ctx, subject.ID, []*users.StudentCompanion{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
	}))

	holdStudentRowLock(t, db, companion.ID)

	// ACT — Tuesday gains the accompanied mode; the Monday edge keeps its basis.
	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = users.AllowedDepartureModes{
		"mon": {users.DepartureAccompanied},
		"tue": {users.DepartureAccompanied},
	}

	require.NoError(t, factory.Student.Update(ctx, loaded))

	edges, err := factory.StudentCompanion.ListForStudent(ctx, subject.ID)
	require.NoError(t, err)
	assert.Len(t, edges, 1)
}
