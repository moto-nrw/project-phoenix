package peopledirectory_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The coordinated multi-child write of the shared departure-plan write path
// (#1694 review). These two suites open the owner's stranding batch, which
// only this package may reach; the remaining reconciliation suites run in the
// People Directory contract suite (#3966).

// giveAccompaniedMonday writes an "Anderes Kind" Monday (plus the required
// note) through the repository, mirroring what the HTTP flow would have
// stored: the accompanied mode widens the plan, it never narrows it.
func giveAccompaniedMonday(t *testing.T, ctx context.Context, factory *api.PeopleRepositorySuiteFactory, studentID int64) {
	t.Helper()
	student, err := factory.Student.FindByID(ctx, studentID)
	require.NoError(t, err)
	require.NotNil(t, student)

	note := "Nachbarskind"
	student.AllowedDepartureModes = student.AllowedDepartureModes.Merge(departure.AllowedDepartureModes{
		"mon": {departure.DepartureAccompanied},
	})
	student.DepartureCompanionNote = &note
	require.NoError(t, factory.Student.Update(ctx, student))
}

// companionEdge validates one edge through the suite composition. This suite
// may not name the retained companion model, so the slice type is inferred
// from the constructor.
func companionEdge[E any](t *testing.T, build func(int64, int64, int) (E, error), studentID, companionID int64, weekday int) []E {
	t.Helper()
	edge, err := build(studentID, companionID, weekday)
	require.NoError(t, err)
	return []E{edge}
}

// TestStudentRepository_Update_BatchAllowsCoordinatedCompanionRemoval pins the
// coordinated multi-child edit (enrollment change-request approval): two
// children linked only to each other, both dropping the accompanied mode in the
// SAME approval, are a valid change. Applied child by child, the first write
// sees the second still carrying its old accompanied plan and would refuse — so
// inside an open batch the verdict is deferred and decided against the final
// plans, where nobody claims "Anderes Kind" anymore.
func TestStudentRepository_Update_BatchAllowsCoordinatedCompanionRemoval(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	factoryCare := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	first := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "BatchOK", "1a")
	second := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "BatchOK", "1a")

	giveAccompaniedMonday(t, ctx, factory, first.ID)
	giveAccompaniedMonday(t, ctx, factory, second.ID)
	require.NoError(t, api.ReplacePeopleRepositorySuiteCompanions(ctx, api.NewPeopleRepositorySuiteCompanions(factoryCare.CarePlan()), first.ID,
		companionEdge(t, api.NewPeopleRepositorySuiteCompanionEdge, first.ID, second.ID, 1)))
	// Neither child has any other "mit wem" detail than the shared edge.
	testpkg.ClearTestStudentCompanionNote(t, db, first.ID)
	testpkg.ClearTestStudentCompanionNote(t, db, second.ID)

	// ACT — one approval, applied child after the other.
	batchCtx, _ := departure.ContextWithStrandingBatch(ctx)
	for _, id := range []int64{first.ID, second.ID} {
		loaded, err := factory.Student.FindByID(batchCtx, id)
		require.NoError(t, err)
		loaded.AllowedDepartureModes = departure.AllowedDepartureModes{
			"mon": {departure.DepartureBus},
		}
		require.NoError(t, factory.Student.Update(batchCtx, loaded),
			"a coordinated removal must not be refused against a half-applied batch")
	}

	// ASSERT — the batch verdict passes and the shared edge is gone.
	require.NoError(t, factory.Student.VerifyCompanionStrandingBatch(batchCtx))

	edges, err := api.NewPeopleRepositorySuiteCompanions(factoryCare.CarePlan()).ListForStudent(ctx, first.ID)
	require.NoError(t, err)
	assert.Empty(t, edges, "the edge lost its basis on both sides and must be gone")
}

// TestStudentRepository_Update_BatchStillRefusesStrandingCompanion pins that
// deferring is not weakening: a child the batch leaves with an accompanied day,
// no note and no remaining link still refuses the whole coordinated edit — just
// at the batch verdict instead of at the individual write.
func TestStudentRepository_Update_BatchStillRefusesStrandingCompanion(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	factoryCare := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "BatchStrand", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "BatchStrand", "1a")

	giveAccompaniedMonday(t, ctx, factory, subject.ID)
	giveAccompaniedMonday(t, ctx, factory, companion.ID)
	require.NoError(t, api.ReplacePeopleRepositorySuiteCompanions(ctx, api.NewPeopleRepositorySuiteCompanions(factoryCare.CarePlan()), subject.ID,
		companionEdge(t, api.NewPeopleRepositorySuiteCompanionEdge, subject.ID, companion.ID, 1)))
	// The companion keeps its accompanied Monday, answered ONLY by the link.
	testpkg.ClearTestStudentCompanionNote(t, db, companion.ID)

	batchCtx, _ := departure.ContextWithStrandingBatch(ctx)
	loaded, err := factory.Student.FindByID(batchCtx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = departure.AllowedDepartureModes{
		"mon": {departure.DepartureBus},
	}

	// ACT — the individual write no longer decides; the batch verdict does.
	require.NoError(t, factory.Student.Update(batchCtx, loaded))
	assert.ErrorIs(t, factory.Student.VerifyCompanionStrandingBatch(batchCtx),
		departure.ErrCompanionWouldLoseDeparture)
}
