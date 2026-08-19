package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FindByStudentAndActiveGroupIDs returns only the student's visits whose
// active_group_id matches one of the given IDs. Empty slice must short-circuit.
func TestVisitRepository_FindByStudentAndActiveGroupIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	now := time.Now()
	v1 := testpkg.CreateTestVisit(t, db, data.Student1.ID, data.ActiveGroup.ID, now.Add(-2*time.Hour), nil)
	v2 := testpkg.CreateTestVisit(t, db, data.Student2.ID, data.ActiveGroup.ID, now.Add(-1*time.Hour), nil)

	defer func() {
		for _, id := range []int64{v1.ID, v2.ID} {
			testpkg.CleanupTableRecords(t, db, "active.visits", id)
		}
	}()

	t.Run("empty id slice short-circuits", func(t *testing.T) {
		results, err := repo.FindByStudentAndActiveGroupIDs(ctx, data.Student1.ID, nil)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("returns only matching student's visits", func(t *testing.T) {
		results, err := repo.FindByStudentAndActiveGroupIDs(ctx, data.Student1.ID, []int64{data.ActiveGroup.ID})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, v1.ID, results[0].ID,
			"must not include student2's visit in the same group")
	})

	t.Run("non-matching active group id returns empty", func(t *testing.T) {
		results, err := repo.FindByStudentAndActiveGroupIDs(ctx, data.Student1.ID, []int64{data.ActiveGroup.ID + 9999})
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("different tenant context returns empty", func(t *testing.T) {
		ctxT2 := testpkg.TenantContext(2)
		results, err := repo.FindByStudentAndActiveGroupIDs(ctxT2, data.Student1.ID, []int64{data.ActiveGroup.ID})
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestVisitRepository_FindByStudentAndTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := repositories.NewFactory(db).ActiveVisit
	ctx := testpkg.Ctx(t)
	data := createVisitTestData(t, db)
	defer cleanupVisitTestData(t, db, data)

	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	// Create visits across different times
	exit1 := twoDaysAgo.Add(2 * time.Hour)
	v1 := testpkg.CreateTestVisit(t, db, data.Student1.ID, data.ActiveGroup.ID, twoDaysAgo, &exit1)

	exit2 := yesterday.Add(3 * time.Hour)
	v2 := testpkg.CreateTestVisit(t, db, data.Student1.ID, data.ActiveGroup.ID, yesterday, &exit2)

	exit3 := now.Add(-30 * time.Minute)
	v3 := testpkg.CreateTestVisit(t, db, data.Student1.ID, data.ActiveGroup.ID, now.Add(-2*time.Hour), &exit3)

	// Student2 visit — for isolation check
	exit4 := now.Add(-1 * time.Hour)
	v4 := testpkg.CreateTestVisit(t, db, data.Student2.ID, data.ActiveGroup.ID, now.Add(-3*time.Hour), &exit4)

	defer func() {
		for _, id := range []int64{v1.ID, v2.ID, v3.ID, v4.ID} {
			testpkg.CleanupTableRecords(t, db, "active.visits", id)
		}
	}()

	t.Run("returns_all_visits_in_range", func(t *testing.T) {
		start := twoDaysAgo.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		results, err := repo.FindByStudentAndTimeRange(ctx, data.Student1.ID, start, end)
		require.NoError(t, err)
		assert.Len(t, results, 3, "should return all 3 visits for student1")
	})

	t.Run("isolates_by_student_id", func(t *testing.T) {
		start := twoDaysAgo.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		results, err := repo.FindByStudentAndTimeRange(ctx, data.Student2.ID, start, end)
		require.NoError(t, err)
		assert.Len(t, results, 1, "should only return student2's visit")
	})

	t.Run("narrows_range", func(t *testing.T) {
		// Only yesterday and today's visits
		start := yesterday.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		results, err := repo.FindByStudentAndTimeRange(ctx, data.Student1.ID, start, end)
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("returns_empty_for_no_matches", func(t *testing.T) {
		start := now.Add(24 * time.Hour)
		end := now.Add(48 * time.Hour)

		results, err := repo.FindByStudentAndTimeRange(ctx, data.Student1.ID, start, end)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("populates_active_group_and_room", func(t *testing.T) {
		start := now.Add(-3 * time.Hour)
		end := now.Add(1 * time.Hour)

		results, err := repo.FindByStudentAndTimeRange(ctx, data.Student1.ID, start, end)
		require.NoError(t, err)
		require.Len(t, results, 1)

		visit := results[0]
		require.NotNil(t, visit.ActiveGroup, "ActiveGroup should be populated via explicit JOIN")
		assert.Equal(t, data.Room, visit.ActiveGroup.RoomID, "RoomID should match the active group's room")
		require.NotNil(t, visit.ActiveGroup.Room, "Room should be populated via explicit JOIN")
		assert.NotEmpty(t, visit.ActiveGroup.Room.Name, "Room name should be loaded")
	})

	t.Run("ordered_entry_time_asc", func(t *testing.T) {
		start := twoDaysAgo.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)

		results, err := repo.FindByStudentAndTimeRange(ctx, data.Student1.ID, start, end)
		require.NoError(t, err)
		require.Len(t, results, 3)
		// Ascending order: oldest first
		assert.True(t, results[0].EntryTime.Before(results[1].EntryTime), "results should be ordered entry_time ASC")
		assert.True(t, results[1].EntryTime.Before(results[2].EntryTime), "results should be ordered entry_time ASC")
	})
}
