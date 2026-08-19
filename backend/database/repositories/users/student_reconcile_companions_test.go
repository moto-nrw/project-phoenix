package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// These tests pin the SHARED departure-plan write path (#1694 review): every
// caller of StudentRepository.Update — not just the HTTP student flow — must
// leave the "läuft mit" edges consistent with the plan it persists. Enrollment
// approval and imports replace AllowedDepartureModes directly on the
// repository, so the reconciliation has to live here, not in the handler.

// giveAccompaniedPlan writes an "Anderes Kind" plan (plus the required note)
// for the given weekdays through the repository, mirroring what the HTTP flow
// would have stored.
func giveAccompaniedPlan(t *testing.T, db *bun.DB, ctx context.Context, studentID int64, days ...string) {
	t.Helper()
	repo := repositories.NewFactory(db).Student

	student, err := repo.FindByID(ctx, studentID)
	require.NoError(t, err)
	require.NotNil(t, student)

	note := "Nachbarskind"
	student.AllowedDepartureModes = users.WithAccompaniedDays(student.AllowedDepartureModes, days)
	student.DepartureCompanionNote = &note
	require.NoError(t, repo.Update(ctx, student))
}

// clearStoredCompanionNote drops the free-text note straight in SQL, leaving a
// child whose "mit wem" is answered only by the structured link — the state a
// note-carrying child reaches once it is linked and the note is removed.
func clearStoredCompanionNote(t *testing.T, db *bun.DB, studentID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.students").
		Set("departure_companion_note = NULL").
		Where("id = ?", studentID).
		Exec(context.Background())
	require.NoError(t, err)
}

// TestStudentRepository_Update_TrimsCompanionEdgesToPlan simulates a direct
// repository writer (enrollment approval) replacing the departure plan: the
// Tuesday edge loses its basis and must be dropped, symmetrically, while the
// Monday edge survives.
func TestStudentRepository_Update_TrimsCompanionEdgesToPlan(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	factory := repositories.NewFactory(db)

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "Trim", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "Trim", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupStudentCompanions(t, db, subject.ID, companion.ID)

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon", "tue")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon", "tue")
	require.NoError(t, factory.StudentCompanion.ReplaceForStudent(ctx, subject.ID, []*users.StudentCompanion{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
		newCompanionEdge(t, subject.ID, companion.ID, 2),
	}))

	// ACT — a writer that knows nothing about companions replaces the plan
	// with one that only allows "Anderes Kind" on Monday.
	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = users.AllowedDepartureModes{
		"mon": {users.DepartureAccompanied},
		"tue": {users.DepartureBus},
	}
	require.NoError(t, factory.Student.Update(ctx, loaded))

	// ASSERT — only the Monday edge remains, from both children's view.
	edges, err := factory.StudentCompanion.ListForStudent(ctx, subject.ID)
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.Equal(t, 1, edges[0].Weekday)

	fromCompanion, err := factory.StudentCompanion.ListForStudent(ctx, companion.ID)
	require.NoError(t, err)
	require.Len(t, fromCompanion, 1)
	assert.Equal(t, 1, fromCompanion[0].Weekday)
}

// TestStudentRepository_Update_DropsAllEdgesWhenPlanLosesAccompanied pins the
// clear path: a plan without any "Anderes Kind" day leaves no basis for any
// link, so all edges go — legal here because the far child still carries the
// free-text note that answers "mit wem".
func TestStudentRepository_Update_DropsAllEdgesWhenPlanLosesAccompanied(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	factory := repositories.NewFactory(db)

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "Clear", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "Clear", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupStudentCompanions(t, db, subject.ID, companion.ID)

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon")
	require.NoError(t, factory.StudentCompanion.ReplaceForStudent(ctx, subject.ID, []*users.StudentCompanion{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
	}))

	// ACT — the new plan has no accompanied day at all.
	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = users.AllowedDepartureModes{
		"mon": {users.DepartureBus},
	}
	require.NoError(t, factory.Student.Update(ctx, loaded))

	edges, err := factory.StudentCompanion.ListForStudent(ctx, subject.ID)
	require.NoError(t, err)
	assert.Empty(t, edges, "a plan without an accompanied day must not keep any edge")
}

// TestStudentRepository_Update_RefusesStrandingCompanionWeekday pins that the
// stranding check of the shared write path is per weekday: an edge the pair
// keeps on Monday does not answer for the far child's accompanied Tuesday when
// the Tuesday edge is being dropped, so the update is refused even though the
// pair stays linked.
func TestStudentRepository_Update_RefusesStrandingCompanionWeekday(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	factory := repositories.NewFactory(db)

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "StrandDay", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "StrandDay", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupStudentCompanions(t, db, subject.ID, companion.ID)

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon", "tue")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon", "tue")
	require.NoError(t, factory.StudentCompanion.ReplaceForStudent(ctx, subject.ID, []*users.StudentCompanion{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
		newCompanionEdge(t, subject.ID, companion.ID, 2),
	}))
	// Both of the companion's accompanied days are answered ONLY by the edges.
	clearStoredCompanionNote(t, db, companion.ID)

	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = users.AllowedDepartureModes{
		"mon": {users.DepartureAccompanied},
		"tue": {users.DepartureBus},
	}

	// ACT + ASSERT — refused; both edges survive.
	err = factory.Student.Update(ctx, loaded)
	require.ErrorIs(t, err, users.ErrCompanionWouldLoseDeparture)

	edges, listErr := factory.StudentCompanion.ListForStudent(ctx, subject.ID)
	require.NoError(t, listErr)
	assert.Len(t, edges, 2, "a refused update must not have dropped the Tuesday edge")
}

// TestStudentRepository_Update_BatchAllowsCoordinatedCompanionRemoval pins the
// coordinated multi-child edit (enrollment change-request approval): two
// children linked only to each other, both dropping the accompanied mode in the
// SAME approval, are a valid change. Applied child by child, the first write
// sees the second still carrying its old accompanied plan and would refuse — so
// inside an open batch the verdict is deferred and decided against the final
// plans, where nobody claims "Anderes Kind" anymore.
func TestStudentRepository_Update_BatchAllowsCoordinatedCompanionRemoval(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	factory := repositories.NewFactory(db)

	first := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "BatchOK", "1a")
	second := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "BatchOK", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, first.ID, second.ID)
	defer cleanupStudentCompanions(t, db, first.ID, second.ID)

	giveAccompaniedPlan(t, db, ctx, first.ID, "mon")
	giveAccompaniedPlan(t, db, ctx, second.ID, "mon")
	require.NoError(t, factory.StudentCompanion.ReplaceForStudent(ctx, first.ID, []*users.StudentCompanion{
		newCompanionEdge(t, first.ID, second.ID, 1),
	}))
	// Neither child has any other "mit wem" detail than the shared edge.
	clearStoredCompanionNote(t, db, first.ID)
	clearStoredCompanionNote(t, db, second.ID)

	// ACT — one approval, applied child after the other.
	batchCtx, _ := users.ContextWithCompanionStrandingBatch(ctx)
	for _, id := range []int64{first.ID, second.ID} {
		loaded, err := factory.Student.FindByID(batchCtx, id)
		require.NoError(t, err)
		loaded.AllowedDepartureModes = users.AllowedDepartureModes{
			"mon": {users.DepartureBus},
		}
		require.NoError(t, factory.Student.Update(batchCtx, loaded),
			"a coordinated removal must not be refused against a half-applied batch")
	}

	// ASSERT — the batch verdict passes and the shared edge is gone.
	require.NoError(t, factory.Student.VerifyCompanionStrandingBatch(batchCtx))

	edges, err := factory.StudentCompanion.ListForStudent(ctx, first.ID)
	require.NoError(t, err)
	assert.Empty(t, edges, "the edge lost its basis on both sides and must be gone")
}

// TestStudentRepository_Update_BatchStillRefusesStrandingCompanion pins that
// deferring is not weakening: a child the batch leaves with an accompanied day,
// no note and no remaining link still refuses the whole coordinated edit — just
// at the batch verdict instead of at the individual write.
func TestStudentRepository_Update_BatchStillRefusesStrandingCompanion(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	factory := repositories.NewFactory(db)

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "BatchStrand", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "BatchStrand", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupStudentCompanions(t, db, subject.ID, companion.ID)

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon")
	require.NoError(t, factory.StudentCompanion.ReplaceForStudent(ctx, subject.ID, []*users.StudentCompanion{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
	}))
	// The companion keeps its accompanied Monday, answered ONLY by the link.
	clearStoredCompanionNote(t, db, companion.ID)

	batchCtx, _ := users.ContextWithCompanionStrandingBatch(ctx)
	loaded, err := factory.Student.FindByID(batchCtx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = users.AllowedDepartureModes{
		"mon": {users.DepartureBus},
	}

	// ACT — the individual write no longer decides; the batch verdict does.
	require.NoError(t, factory.Student.Update(batchCtx, loaded))
	assert.ErrorIs(t, factory.Student.VerifyCompanionStrandingBatch(batchCtx),
		users.ErrCompanionWouldLoseDeparture)
}

// TestStudentRepository_Update_RefusesStrandingCompanion pins the guard rail:
// when the dropped edge is the far child's ONLY "mit wem" detail (no note, no
// other link), the whole update is refused with the shared sentinel — exactly
// like the service-level removal check — instead of silently leaving that
// child with a self-contradicting plan.
func TestStudentRepository_Update_RefusesStrandingCompanion(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)
	factory := repositories.NewFactory(db)

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "Strand", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "Strand", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupStudentCompanions(t, db, subject.ID, companion.ID)

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon")
	require.NoError(t, factory.StudentCompanion.ReplaceForStudent(ctx, subject.ID, []*users.StudentCompanion{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
	}))
	// The companion's "mit wem" is now answered ONLY by the link.
	clearStoredCompanionNote(t, db, companion.ID)

	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = users.AllowedDepartureModes{
		"mon": {users.DepartureBus},
	}

	// ACT + ASSERT — refused with the shared sentinel; the edge survives.
	err = factory.Student.Update(ctx, loaded)
	require.ErrorIs(t, err, users.ErrCompanionWouldLoseDeparture)

	edges, listErr := factory.StudentCompanion.ListForStudent(ctx, subject.ID)
	require.NoError(t, listErr)
	assert.Len(t, edges, 1, "a refused update must not have dropped the edge")
}
