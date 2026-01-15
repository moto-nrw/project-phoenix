package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestTimeframeRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Timeframe
	ctx := context.Background()

	t.Run("creates timeframe with valid data", func(t *testing.T) {
		now := time.Now()
		endTime := now.Add(2 * time.Hour)
		timeframe := &schedule.Timeframe{
			StartTime:   now,
			EndTime:     &endTime,
			IsActive:    true,
			Description: "Test timeframe",
		}

		err := repo.Create(ctx, timeframe)
		require.NoError(t, err)
		assert.NotZero(t, timeframe.ID)

		testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)
	})

	t.Run("creates open-ended timeframe", func(t *testing.T) {
		now := time.Now()
		timeframe := &schedule.Timeframe{
			StartTime:   now,
			IsActive:    false,
			Description: "Open-ended timeframe",
		}

		err := repo.Create(ctx, timeframe)
		require.NoError(t, err)
		assert.NotZero(t, timeframe.ID)
		assert.Nil(t, timeframe.EndTime)

		testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)
	})

	t.Run("create with nil timeframe should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestTimeframeRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Timeframe
	ctx := context.Background()

	t.Run("finds existing timeframe", func(t *testing.T) {
		now := time.Now()
		timeframe := &schedule.Timeframe{
			StartTime:   now,
			IsActive:    true,
			Description: "FindByID test",
		}
		err := repo.Create(ctx, timeframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)

		found, err := repo.FindByID(ctx, timeframe.ID)
		require.NoError(t, err)
		assert.Equal(t, timeframe.ID, found.ID)
		assert.Equal(t, "FindByID test", found.Description)
	})

	t.Run("returns error for non-existent timeframe", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestTimeframeRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Timeframe
	ctx := context.Background()

	t.Run("updates timeframe", func(t *testing.T) {
		now := time.Now()
		timeframe := &schedule.Timeframe{
			StartTime:   now,
			IsActive:    true,
			Description: "Update test",
		}
		err := repo.Create(ctx, timeframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)

		timeframe.Description = "Updated description"
		timeframe.IsActive = false
		err = repo.Update(ctx, timeframe)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, timeframe.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", found.Description)
		assert.False(t, found.IsActive)
	})
}

func TestTimeframeRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Timeframe
	ctx := context.Background()

	t.Run("deletes existing timeframe", func(t *testing.T) {
		now := time.Now()
		timeframe := &schedule.Timeframe{
			StartTime:   now,
			IsActive:    true,
			Description: "Delete test",
		}
		err := repo.Create(ctx, timeframe)
		require.NoError(t, err)

		err = repo.Delete(ctx, timeframe.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, timeframe.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestTimeframeRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Timeframe
	ctx := context.Background()

	t.Run("lists all timeframes", func(t *testing.T) {
		now := time.Now()
		timeframe := &schedule.Timeframe{
			StartTime:   now,
			IsActive:    true,
			Description: "List test",
		}
		err := repo.Create(ctx, timeframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)

		timeframes, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, timeframes)
	})
}

func TestTimeframeRepository_FindActive(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Timeframe
	ctx := context.Background()

	t.Run("finds only active timeframes", func(t *testing.T) {
		now := time.Now()
		activeTimeframe := &schedule.Timeframe{
			StartTime:   now,
			IsActive:    true,
			Description: "Active timeframe",
		}
		inactiveTimeframe := &schedule.Timeframe{
			StartTime:   now,
			IsActive:    false,
			Description: "Inactive timeframe",
		}

		err := repo.Create(ctx, activeTimeframe)
		require.NoError(t, err)
		err = repo.Create(ctx, inactiveTimeframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.timeframes", activeTimeframe.ID, inactiveTimeframe.ID)

		timeframes, err := repo.FindActive(ctx)
		require.NoError(t, err)

		// All returned timeframes should be active
		for _, tf := range timeframes {
			assert.True(t, tf.IsActive)
		}

		// Our active timeframe should be in the results
		var found bool
		for _, tf := range timeframes {
			if tf.ID == activeTimeframe.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestTimeframeRepository_FindByTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Timeframe
	ctx := context.Background()

	t.Run("finds timeframes overlapping with range", func(t *testing.T) {
		now := time.Now()
		endTime := now.Add(2 * time.Hour)
		timeframe := &schedule.Timeframe{
			StartTime:   now,
			EndTime:     &endTime,
			IsActive:    true,
			Description: "Time range test",
		}
		err := repo.Create(ctx, timeframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)

		// Search range that overlaps
		searchStart := now.Add(-1 * time.Hour)
		searchEnd := now.Add(1 * time.Hour)

		timeframes, err := repo.FindByTimeRange(ctx, searchStart, searchEnd)
		require.NoError(t, err)
		assert.NotEmpty(t, timeframes)

		var found bool
		for _, tf := range timeframes {
			if tf.ID == timeframe.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestTimeframeRepository_FindByDescription(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Timeframe
	ctx := context.Background()

	t.Run("finds timeframes by description", func(t *testing.T) {
		now := time.Now()
		timeframe := &schedule.Timeframe{
			StartTime:   now,
			IsActive:    true,
			Description: "Unique description for search test",
		}
		err := repo.Create(ctx, timeframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.timeframes", timeframe.ID)

		timeframes, err := repo.FindByDescription(ctx, "unique description")
		require.NoError(t, err)
		assert.NotEmpty(t, timeframes)

		var found bool
		for _, tf := range timeframes {
			if tf.ID == timeframe.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}
