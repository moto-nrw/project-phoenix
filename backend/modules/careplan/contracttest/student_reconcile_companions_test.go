package contracttest_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/api"

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
	repo := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db)).Student

	student, err := repo.FindByID(ctx, studentID)
	require.NoError(t, err)
	require.NotNil(t, student)

	note := "Nachbarskind"
	student.AllowedDepartureModes = peopleValues.WithAccompaniedDays(student.AllowedDepartureModes, days)
	student.DepartureCompanionNote = &note
	require.NoError(t, repo.Update(ctx, student))
}

// TestStudentRepository_Update_TrimsCompanionEdgesToPlan simulates a direct
// repository writer (enrollment approval) replacing the departure plan: the
// Tuesday edge loses its basis and must be dropped, symmetrically, while the
// Monday edge survives.
func TestStudentRepository_Update_TrimsCompanionEdgesToPlan(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "Trim", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "Trim", "1a")

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon", "tue")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon", "tue")
	require.NoError(t, api.ReplacePeopleRepositorySuiteCompanions(ctx, api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()), subject.ID, []*careplan.CompanionEdge{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
		newCompanionEdge(t, subject.ID, companion.ID, 2),
	}))

	// ACT — a writer that knows nothing about companions replaces the plan
	// with one that only allows "Anderes Kind" on Monday.
	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = peopleValues.AllowedDepartureModes{
		"mon": {peopleValues.DepartureAccompanied},
		"tue": {peopleValues.DepartureBus},
	}
	require.NoError(t, factory.Student.Update(ctx, loaded))

	// ASSERT — only the Monday edge remains, from both children's view.
	edges, err := api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()).ListForStudent(ctx, subject.ID)
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.Equal(t, 1, edges[0].Weekday)

	fromCompanion, err := api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()).ListForStudent(ctx, companion.ID)
	require.NoError(t, err)
	require.Len(t, fromCompanion, 1)
	assert.Equal(t, 1, fromCompanion[0].Weekday)
}

// TestStudentRepository_Update_DropsAllEdgesWhenPlanLosesAccompanied pins the
// clear path: a plan without any "Anderes Kind" day leaves no basis for any
// link, so all edges go — legal here because the far child still carries the
// free-text note that answers "mit wem".
func TestStudentRepository_Update_DropsAllEdgesWhenPlanLosesAccompanied(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "Clear", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "Clear", "1a")

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon")
	require.NoError(t, api.ReplacePeopleRepositorySuiteCompanions(ctx, api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()), subject.ID, []*careplan.CompanionEdge{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
	}))

	// ACT — the new plan has no accompanied day at all.
	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = peopleValues.AllowedDepartureModes{
		"mon": {peopleValues.DepartureBus},
	}
	require.NoError(t, factory.Student.Update(ctx, loaded))

	edges, err := api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()).ListForStudent(ctx, subject.ID)
	require.NoError(t, err)
	assert.Empty(t, edges, "a plan without an accompanied day must not keep any edge")
}

// TestStudentRepository_Update_RefusesStrandingCompanionWeekday pins that the
// stranding check of the shared write path is per weekday: an edge the pair
// keeps on Monday does not answer for the far child's accompanied Tuesday when
// the Tuesday edge is being dropped, so the update is refused even though the
// pair stays linked.
func TestStudentRepository_Update_RefusesStrandingCompanionWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "StrandDay", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "StrandDay", "1a")

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon", "tue")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon", "tue")
	require.NoError(t, api.ReplacePeopleRepositorySuiteCompanions(ctx, api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()), subject.ID, []*careplan.CompanionEdge{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
		newCompanionEdge(t, subject.ID, companion.ID, 2),
	}))
	// Both of the companion's accompanied days are answered ONLY by the edges.
	testpkg.ClearTestStudentCompanionNote(t, db, companion.ID)

	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = peopleValues.AllowedDepartureModes{
		"mon": {peopleValues.DepartureAccompanied},
		"tue": {peopleValues.DepartureBus},
	}

	// ACT + ASSERT — refused; both edges survive.
	err = factory.Student.Update(ctx, loaded)
	require.ErrorIs(t, err, peopleValues.TranslateStudentWriteError(peopleValues.ErrCompanionWouldLoseDeparture))

	edges, listErr := api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()).ListForStudent(ctx, subject.ID)
	require.NoError(t, listErr)
	assert.Len(t, edges, 2, "a refused update must not have dropped the Tuesday edge")
}

// TestStudentRepository_Update_RefusesStrandingCompanion pins the guard rail:
// when the dropped edge is the far child's ONLY "mit wem" detail (no note, no
// other link), the whole update is refused with the shared sentinel — exactly
// like the service-level removal check — instead of silently leaving that
// child with a self-contradicting plan.
func TestStudentRepository_Update_RefusesStrandingCompanion(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	factory := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))

	subject := testpkg.CreateTestStudent(t, db, "ReconcileSubject", "Strand", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ReconcileCompanion", "Strand", "1a")

	giveAccompaniedPlan(t, db, ctx, subject.ID, "mon")
	giveAccompaniedPlan(t, db, ctx, companion.ID, "mon")
	require.NoError(t, api.ReplacePeopleRepositorySuiteCompanions(ctx, api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()), subject.ID, []*careplan.CompanionEdge{
		newCompanionEdge(t, subject.ID, companion.ID, 1),
	}))
	// The companion's "mit wem" is now answered ONLY by the link.
	testpkg.ClearTestStudentCompanionNote(t, db, companion.ID)

	loaded, err := factory.Student.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	loaded.AllowedDepartureModes = peopleValues.AllowedDepartureModes{
		"mon": {peopleValues.DepartureBus},
	}

	// ACT + ASSERT — refused with the shared sentinel; the edge survives.
	err = factory.Student.Update(ctx, loaded)
	require.ErrorIs(t, err, peopleValues.TranslateStudentWriteError(peopleValues.ErrCompanionWouldLoseDeparture))

	edges, listErr := api.NewPeopleRepositorySuiteCompanions(factory.CarePlan()).ListForStudent(ctx, subject.ID)
	require.NoError(t, listErr)
	assert.Len(t, edges, 1, "a refused update must not have dropped the edge")
}
