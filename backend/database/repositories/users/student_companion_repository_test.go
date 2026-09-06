package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Test Helpers
// ============================================================================

// newCompanionEdge builds a normalized edge or fails the test.
func newCompanionEdge(t *testing.T, studentID, companionID int64, weekday int) *users.StudentCompanion {
	t.Helper()
	edge, err := users.NewStudentCompanion(studentID, companionID, weekday)
	require.NoError(t, err)
	return edge
}

// companionIDsOf projects the far ends of the edges as seen from studentID.
func companionIDsOf(t *testing.T, studentID int64, edges []*users.StudentCompanion) []int64 {
	t.Helper()
	ids := make([]int64, 0, len(edges))
	for _, edge := range edges {
		other, ok := edge.Other(studentID)
		require.True(t, ok, "ListForStudent returned an edge not touching the student")
		ids = append(ids, other)
	}
	return ids
}

// ============================================================================
// ReplaceForStudent Tests
// ============================================================================

// TestStudentCompanionRepository_ReplaceForStudent_IsSymmetric pins the central
// invariant of the Laufgemeinschaft model: an edge is one undirected row, so
// linking Lina to Tom from Lina's card must make Lina appear on Tom's card too.
// If a future change ever stores a directed row per child, this fails.
func TestStudentCompanionRepository_ReplaceForStudent_IsSymmetric(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StudentCompanion
	ctx := testpkg.Ctx(t)

	studentA := testpkg.CreateTestStudent(t, db, "CompanionA", "Symmetric", "1a")
	studentB := testpkg.CreateTestStudent(t, db, "CompanionB", "Symmetric", "1a")

	// ACT — edited from A's card only.
	err := repo.ReplaceForStudent(ctx, studentA.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentA.ID, studentB.ID, 1),
	})
	require.NoError(t, err)

	// ASSERT — the edge is visible from BOTH endpoints.
	edgesA, err := repo.ListForStudent(ctx, studentA.ID)
	require.NoError(t, err)
	require.Len(t, edgesA, 1)
	assert.Equal(t, []int64{studentB.ID}, companionIDsOf(t, studentA.ID, edgesA))
	assert.Equal(t, 1, edgesA[0].Weekday)
	assert.Equal(t, testpkg.Tenant(t), edgesA[0].GetTenantID(), "tenant_id must be filled from context")

	edgesB, err := repo.ListForStudent(ctx, studentB.ID)
	require.NoError(t, err)
	require.Len(t, edgesB, 1, "the same row must be visible from the other child")
	assert.Equal(t, []int64{studentA.ID}, companionIDsOf(t, studentB.ID, edgesB))
	assert.Equal(t, edgesA[0].ID, edgesB[0].ID, "both children must see the SAME row, not a mirrored copy")
}

// TestStudentCompanionRepository_ReplaceForStudent_MultipleWeekdays checks that a
// pair walking together on several days is stored as one row per weekday and
// read back for both children.
func TestStudentCompanionRepository_ReplaceForStudent_MultipleWeekdays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StudentCompanion
	ctx := testpkg.Ctx(t)

	studentA := testpkg.CreateTestStudent(t, db, "CompanionA", "Weekdays", "2a")
	studentB := testpkg.CreateTestStudent(t, db, "CompanionB", "Weekdays", "2a")

	err := repo.ReplaceForStudent(ctx, studentA.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentA.ID, studentB.ID, 1),
		newCompanionEdge(t, studentB.ID, studentA.ID, 4), // reversed args, same pair
	})
	require.NoError(t, err)

	edges, err := repo.ListForStudent(ctx, studentB.ID)
	require.NoError(t, err)
	require.Len(t, edges, 2)
	assert.Equal(t, 1, edges[0].Weekday, "ListForStudent orders by weekday")
	assert.Equal(t, 4, edges[1].Weekday)
	for _, edge := range edges {
		assert.Less(t, edge.StudentLowID, edge.StudentHighID, "rows must stay in normalized low/high order")
	}
}

// TestStudentCompanionRepository_ReplaceForStudent_EmptyClears covers the
// "Laufgemeinschaft aufgelöst" path: submitting an empty list from the child
// detail view must remove every edge of that child — including from the other
// children's cards.
func TestStudentCompanionRepository_ReplaceForStudent_EmptyClears(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StudentCompanion
	ctx := testpkg.Ctx(t)

	studentA := testpkg.CreateTestStudent(t, db, "CompanionA", "Clear", "3a")
	studentB := testpkg.CreateTestStudent(t, db, "CompanionB", "Clear", "3a")

	require.NoError(t, repo.ReplaceForStudent(ctx, studentA.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentA.ID, studentB.ID, 2),
	}))

	// ACT — clear A's list.
	require.NoError(t, repo.ReplaceForStudent(ctx, studentA.ID, nil))

	edgesA, err := repo.ListForStudent(ctx, studentA.ID)
	require.NoError(t, err)
	assert.Empty(t, edgesA)

	edgesB, err := repo.ListForStudent(ctx, studentB.ID)
	require.NoError(t, err)
	assert.Empty(t, edgesB, "clearing one child's list must remove the shared row from the other child too")
}

// TestStudentCompanionRepository_ReplaceForStudent_LeavesUnrelatedEdges is the
// blast-radius test: ReplaceForStudent deletes only the edges TOUCHING the
// edited child. A delete that forgot its WHERE (or matched on tenant only)
// would wipe every other Laufgemeinschaft in the school — this catches that.
func TestStudentCompanionRepository_ReplaceForStudent_LeavesUnrelatedEdges(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StudentCompanion
	ctx := testpkg.Ctx(t)

	studentA := testpkg.CreateTestStudent(t, db, "CompanionA", "Unrelated", "4a")
	studentB := testpkg.CreateTestStudent(t, db, "CompanionB", "Unrelated", "4a")
	studentC := testpkg.CreateTestStudent(t, db, "CompanionC", "Unrelated", "4a")

	// A–B (edited later) and B–C (must survive).
	require.NoError(t, repo.ReplaceForStudent(ctx, studentA.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentA.ID, studentB.ID, 1),
	}))
	require.NoError(t, repo.ReplaceForStudent(ctx, studentC.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentC.ID, studentB.ID, 1),
	}))

	// ACT — clear A's list; B–C is none of A's business.
	require.NoError(t, repo.ReplaceForStudent(ctx, studentA.ID, nil))

	edgesA, err := repo.ListForStudent(ctx, studentA.ID)
	require.NoError(t, err)
	assert.Empty(t, edgesA)

	edgesC, err := repo.ListForStudent(ctx, studentC.ID)
	require.NoError(t, err)
	require.Len(t, edgesC, 1, "the unrelated B–C edge must survive")
	assert.Equal(t, []int64{studentB.ID}, companionIDsOf(t, studentC.ID, edgesC))

	edgesB, err := repo.ListForStudent(ctx, studentB.ID)
	require.NoError(t, err)
	require.Len(t, edgesB, 1, "B keeps exactly its link to C")
	assert.Equal(t, []int64{studentC.ID}, companionIDsOf(t, studentB.ID, edgesB))
}

// ============================================================================
// CompanionIDsForWeekday Tests
// ============================================================================

// TestStudentCompanionRepository_CompanionIDsForWeekday covers the bulk read
// behind the Kindersuche grouping: both directions of every edge are reported,
// and only for the requested weekday.
func TestStudentCompanionRepository_CompanionIDsForWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StudentCompanion
	ctx := testpkg.Ctx(t)

	studentA := testpkg.CreateTestStudent(t, db, "CompanionA", "Weekday", "5a")
	studentB := testpkg.CreateTestStudent(t, db, "CompanionB", "Weekday", "5a")
	studentC := testpkg.CreateTestStudent(t, db, "CompanionC", "Weekday", "5a")

	// A–B on Monday, A–C on Tuesday.
	require.NoError(t, repo.ReplaceForStudent(ctx, studentA.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentA.ID, studentB.ID, 1),
		newCompanionEdge(t, studentA.ID, studentC.ID, 2),
	}))

	ids := []int64{studentA.ID, studentB.ID, studentC.ID}

	t.Run("reports both directions for the requested weekday", func(t *testing.T) {
		result, err := repo.CompanionIDsForWeekday(ctx, ids, 1)
		require.NoError(t, err)

		assert.Equal(t, []int64{studentB.ID}, result[studentA.ID])
		assert.Equal(t, []int64{studentA.ID}, result[studentB.ID], "the far end must see the near end too")
		assert.NotContains(t, result, studentC.ID, "a child without a Monday link must be absent from the map")
	})

	t.Run("ignores edges on other weekdays", func(t *testing.T) {
		result, err := repo.CompanionIDsForWeekday(ctx, ids, 2)
		require.NoError(t, err)

		assert.Equal(t, []int64{studentC.ID}, result[studentA.ID])
		assert.Equal(t, []int64{studentA.ID}, result[studentC.ID])
		assert.NotContains(t, result, studentB.ID, "Monday's link must not leak into Tuesday")
	})

	t.Run("returns an empty map for a weekday without links", func(t *testing.T) {
		result, err := repo.CompanionIDsForWeekday(ctx, ids, 5)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns an empty map for an empty id list", func(t *testing.T) {
		result, err := repo.CompanionIDsForWeekday(ctx, nil, 1)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("rejects a weekday outside Mon..Fri", func(t *testing.T) {
		_, err := repo.CompanionIDsForWeekday(ctx, ids, 6)
		require.ErrorIs(t, err, users.ErrCompanionInvalidWeekday)
	})
}

// TestStudentCompanionRepository_CompanionIDsForWeekdayTransitive pins the
// connected-component semantics of the Kindersuche grouping: the chain
// A–B–C–D is ONE Laufgemeinschaft, so asking for A and D alone must still
// report every other member, including the two children bridging them. Direct
// neighbours only would let the page render one group as two.
func TestStudentCompanionRepository_CompanionIDsForWeekdayTransitive(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StudentCompanion
	ctx := testpkg.Ctx(t)

	studentA := testpkg.CreateTestStudent(t, db, "ChainA", "Reach", "5b")
	studentB := testpkg.CreateTestStudent(t, db, "ChainB", "Reach", "5b")
	studentC := testpkg.CreateTestStudent(t, db, "ChainC", "Reach", "5b")
	studentD := testpkg.CreateTestStudent(t, db, "ChainD", "Reach", "5b")

	// A–B, B–C, C–D on Monday.
	require.NoError(t, repo.ReplaceForStudent(ctx, studentB.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentB.ID, studentA.ID, 1),
		newCompanionEdge(t, studentB.ID, studentC.ID, 1),
	}))
	require.NoError(t, repo.ReplaceForStudent(ctx, studentD.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentD.ID, studentC.ID, 1),
	}))

	t.Run("reports the whole component for the ends of a chain", func(t *testing.T) {
		result, err := repo.CompanionIDsForWeekday(ctx, []int64{studentA.ID, studentD.ID}, 1)
		require.NoError(t, err)

		assert.ElementsMatch(t, []int64{studentB.ID, studentC.ID, studentD.ID}, result[studentA.ID])
		assert.ElementsMatch(t, []int64{studentA.ID, studentB.ID, studentC.ID}, result[studentD.ID])
	})

	t.Run("does not reach across weekdays", func(t *testing.T) {
		result, err := repo.CompanionIDsForWeekday(ctx, []int64{studentA.ID, studentD.ID}, 2)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// ============================================================================
// ListLinksForStudent Tests
// ============================================================================

// TestStudentCompanionRepository_ListLinksForStudent verifies the per-child
// read model: one entry per companion with the weekdays folded and the
// companion's name joined in, so the detail view renders without a second round
// trip.
func TestStudentCompanionRepository_ListLinksForStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StudentCompanion
	ctx := testpkg.Ctx(t)

	studentA := testpkg.CreateTestStudent(t, db, "LinkSource", "Companion", "6a")
	studentB := testpkg.CreateTestStudent(t, db, "LinkTarget", "Companion", "6a")

	require.NoError(t, repo.ReplaceForStudent(ctx, studentA.ID, []*users.StudentCompanion{
		newCompanionEdge(t, studentA.ID, studentB.ID, 1),
		newCompanionEdge(t, studentA.ID, studentB.ID, 3),
	}))

	links, err := repo.ListLinksForStudent(ctx, studentA.ID)
	require.NoError(t, err)
	require.Len(t, links, 1, "two weekdays with the same child must fold into ONE link")

	link := links[0]
	assert.Equal(t, studentB.ID, link.CompanionStudentID)
	assert.Equal(t, []string{users.PickupDayMonday, users.PickupDayWednesday}, link.Weekdays)
	assert.Equal(t, "LinkTarget", link.FirstName, "the companion's name must be joined in")
	assert.Equal(t, "Companion", link.LastName)

	t.Run("returns an empty list for a child without companions", func(t *testing.T) {
		lonely := testpkg.CreateTestStudent(t, db, "NoCompanion", "Child", "6b")

		got, err := repo.ListLinksForStudent(ctx, lonely.ID)
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

// ============================================================================
// ListLinksForStudents Tests
// ============================================================================

// TestStudentCompanionRepository_ListLinksForStudents pins the bulk read the
// offline lists (student export, class roster) depend on: every requested child
// gets exactly its OWN folded links, with names, out of the single edge query.
//
// The per-child bucketing is what makes that affordable. Folding the whole edge
// slice once per child rescans every edge of the school for every row of the
// export, which is quadratic at the sizes these documents are rendered at — the
// result below must therefore stay identical while the work stays linear.
func TestStudentCompanionRepository_ListLinksForStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).StudentCompanion
	ctx := testpkg.Ctx(t)

	first := testpkg.CreateTestStudent(t, db, "BulkFirst", "Companion", "7a")
	second := testpkg.CreateTestStudent(t, db, "BulkSecond", "Companion", "7a")
	third := testpkg.CreateTestStudent(t, db, "BulkThird", "Companion", "7a")
	lonely := testpkg.CreateTestStudent(t, db, "BulkLonely", "Companion", "7a")

	// A chain: first-second on Mon+Wed, second-third on Tue. The middle child
	// therefore carries links to BOTH ends and must not receive the other pair's
	// weekdays.
	require.NoError(t, repo.ReplaceForStudent(ctx, first.ID, []*users.StudentCompanion{
		newCompanionEdge(t, first.ID, second.ID, 1),
		newCompanionEdge(t, first.ID, second.ID, 3),
	}))
	require.NoError(t, repo.ReplaceForStudent(ctx, third.ID, []*users.StudentCompanion{
		newCompanionEdge(t, third.ID, second.ID, 2),
	}))

	byStudent, err := repo.ListLinksForStudents(ctx, []int64{first.ID, second.ID, third.ID, lonely.ID})
	require.NoError(t, err)

	require.Len(t, byStudent[first.ID], 1)
	assert.Equal(t, second.ID, byStudent[first.ID][0].CompanionStudentID)
	assert.Equal(t, []string{users.PickupDayMonday, users.PickupDayWednesday}, byStudent[first.ID][0].Weekdays)
	assert.Equal(t, "BulkSecond", byStudent[first.ID][0].FirstName, "the companion's name must be joined in")

	require.Len(t, byStudent[second.ID], 2, "the middle child keeps one link per end")
	linkedDays := map[int64][]string{}
	for _, link := range byStudent[second.ID] {
		linkedDays[link.CompanionStudentID] = link.Weekdays
	}
	assert.Equal(t, []string{users.PickupDayMonday, users.PickupDayWednesday}, linkedDays[first.ID])
	assert.Equal(t, []string{users.PickupDayTuesday}, linkedDays[third.ID])

	require.Len(t, byStudent[third.ID], 1)
	assert.Equal(t, second.ID, byStudent[third.ID][0].CompanionStudentID)
	assert.Equal(t, []string{users.PickupDayTuesday}, byStudent[third.ID][0].Weekdays)

	// A child without links is absent from the map, not present with an empty
	// slice — the callers test for presence.
	_, ok := byStudent[lonely.ID]
	assert.False(t, ok)
}
