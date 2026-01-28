package activities_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

// createSchedule is a helper to create a schedule
func createSchedule(t *testing.T, db *bun.DB, groupID int64, weekday int, timeframeID *int64) *activities.Schedule {
	t.Helper()

	ctx := context.Background()
	schedule := &activities.Schedule{
		ActivityGroupID: groupID,
		Weekday:         weekday,
		TimeframeID:     timeframeID,
	}

	err := db.NewInsert().
		Model(schedule).
		ModelTableExpr(`activities.schedules AS "schedule"`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test schedule")

	return schedule
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestScheduleRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("creates schedule with valid data", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "ScheduleGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule := &activities.Schedule{
			ActivityGroupID: group.ID,
			Weekday:         2, // Tuesday
		}

		err := repo.Create(ctx, schedule)
		require.NoError(t, err)
		assert.NotZero(t, schedule.ID)

		testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule.ID)
	})

	t.Run("creates schedule without timeframe", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "NoTimeframeGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule := &activities.Schedule{
			ActivityGroupID: group.ID,
			Weekday:         3,
			TimeframeID:     nil, // No timeframe
		}

		err := repo.Create(ctx, schedule)
		require.NoError(t, err)
		assert.NotZero(t, schedule.ID)
		assert.Nil(t, schedule.TimeframeID)

		testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule.ID)
	})
}

func TestScheduleRepository_Create_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("returns error when schedule is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestScheduleRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("finds existing schedule", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "FindByID")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule := createSchedule(t, db, group.ID, 1, nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule.ID)

		found, err := repo.FindByID(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, schedule.ID, found.ID)
		assert.Equal(t, schedule.Weekday, found.Weekday)
	})

	t.Run("returns error for non-existent schedule", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestScheduleRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("updates schedule weekday", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Update")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule := createSchedule(t, db, group.ID, 1, nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule.ID)

		schedule.Weekday = 5
		err := repo.Update(ctx, schedule)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, found.Weekday)
	})
}

func TestScheduleRepository_Update_WithNil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("returns error when schedule is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestScheduleRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("deletes existing schedule", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Delete")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule := createSchedule(t, db, group.ID, 1, nil)

		err := repo.Delete(ctx, schedule.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, schedule.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestScheduleRepository_List(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("lists all schedules", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "List")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule := createSchedule(t, db, group.ID, 1, nil)
		defer testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule.ID)

		schedules, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, schedules)
	})
}

func TestScheduleRepository_FindByGroupID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("finds schedules for a specific group", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "GroupSchedules")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule1 := createSchedule(t, db, group.ID, 1, nil) // Monday
		schedule2 := createSchedule(t, db, group.ID, 3, nil) // Wednesday
		defer testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule1.ID, schedule2.ID)

		schedules, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(schedules), 2)

		// Verify our schedules are in the results
		var foundIDs []int64
		for _, s := range schedules {
			if s.ID == schedule1.ID || s.ID == schedule2.ID {
				foundIDs = append(foundIDs, s.ID)
			}
		}
		assert.Len(t, foundIDs, 2)
	})

	t.Run("returns empty for group with no schedules", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "EmptySchedules")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedules, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, schedules)
	})
}

func TestScheduleRepository_FindByWeekday(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("finds schedules for a specific weekday", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "WeekdaySchedules")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule := createSchedule(t, db, group.ID, 4, nil) // Thursday
		defer testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule.ID)

		schedules, err := repo.FindByWeekday(ctx, "4")
		require.NoError(t, err)

		// Should find at least our schedule
		var found bool
		for _, s := range schedules {
			if s.ID == schedule.ID {
				found = true
				assert.Equal(t, 4, s.Weekday)
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty for weekday with no schedules", func(t *testing.T) {
		// Use a non-existent weekday pattern that is unlikely to have schedules
		schedules, err := repo.FindByWeekday(ctx, "999")
		require.NoError(t, err)
		assert.Empty(t, schedules)
	})
}

func TestScheduleRepository_FindByTimeframeID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("returns empty for timeframe with no schedules", func(t *testing.T) {
		// Use a non-existent timeframe ID (high value unlikely to exist)
		schedules, err := repo.FindByTimeframeID(ctx, int64(999999))
		require.NoError(t, err)
		assert.NotNil(t, schedules)
		assert.Empty(t, schedules)
	})

	t.Run("finds schedules for a specific timeframe", func(t *testing.T) {
		// Create a timeframe using the fixture helper
		timeframe := testpkg.CreateTestTimeframe(t, db, "TestTimeframe")
		defer testpkg.CleanupScheduleFixtures(t, db, timeframe.ID)

		// Create an activity group and schedule linked to the timeframe
		group := testpkg.CreateTestActivityGroup(t, db, "TimeframeScheduleGroup")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, 0, group.CategoryID, 0)
		defer testpkg.CleanupTableRecords(t, db, "activities.groups", group.ID)

		schedule := createSchedule(t, db, group.ID, 1, &timeframe.ID)
		defer testpkg.CleanupTableRecords(t, db, "activities.schedules", schedule.ID)

		// Query by timeframe ID
		schedules, err := repo.FindByTimeframeID(ctx, timeframe.ID)
		require.NoError(t, err)
		assert.NotNil(t, schedules)

		// Verify our schedule is in the results
		var found bool
		for _, s := range schedules {
			if s.ID == schedule.ID {
				found = true
				assert.Equal(t, &timeframe.ID, s.TimeframeID)
				break
			}
		}
		assert.True(t, found, "Schedule with timeframe should be found")
	})
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestScheduleRepository_Delete_NonExistent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).ActivitySchedule
	ctx := context.Background()

	t.Run("does not error when deleting non-existent schedule", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}
