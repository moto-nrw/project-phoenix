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

func TestDateframeRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Dateframe
	ctx := context.Background()

	t.Run("creates dateframe with valid data", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "Full academic year",
			Description: "Complete year dateframe",
		}

		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		assert.NotZero(t, dateframe.ID)

		testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)
	})

	t.Run("creates dateframe with same start and end date", func(t *testing.T) {
		singleDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   singleDate,
			EndDate:     singleDate,
			Name:        "Single Day Event",
			Description: "One day only",
		}

		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		assert.NotZero(t, dateframe.ID)

		testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)
	})

	t.Run("create with nil dateframe should fail", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("create with invalid dates should fail", func(t *testing.T) {
		startDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate: startDate,
			EndDate:   endDate,
			Name:      "Invalid Range",
		}

		err := repo.Create(ctx, dateframe)
		assert.Error(t, err)
	})
}

func TestDateframeRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Dateframe
	ctx := context.Background()

	t.Run("finds existing dateframe", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "FindByID Test",
			Description: "Test dateframe",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		found, err := repo.FindByID(ctx, dateframe.ID)
		require.NoError(t, err)
		assert.Equal(t, dateframe.ID, found.ID)
		assert.Equal(t, "FindByID Test", found.Name)
	})

	t.Run("returns error for non-existent dateframe", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestDateframeRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Dateframe
	ctx := context.Background()

	t.Run("updates dateframe", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "Original Name",
			Description: "Original Description",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		dateframe.Name = "Updated Name"
		dateframe.Description = "Updated Description"
		err = repo.Update(ctx, dateframe)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, dateframe.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", found.Name)
		assert.Equal(t, "Updated Description", found.Description)
	})

	t.Run("update with nil dateframe should fail", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestDateframeRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Dateframe
	ctx := context.Background()

	t.Run("deletes existing dateframe", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "Delete Test",
			Description: "To be deleted",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)

		err = repo.Delete(ctx, dateframe.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, dateframe.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestDateframeRepository_List(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Dateframe
	ctx := context.Background()

	t.Run("lists all dateframes", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "List Test",
			Description: "Test listing",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		dateframes, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, dateframes)
	})

	t.Run("lists with no results returns empty slice", func(t *testing.T) {
		dateframes, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotNil(t, dateframes)
	})
}

func TestDateframeRepository_FindByName(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Dateframe
	ctx := context.Background()

	t.Run("finds dateframe by name", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		uniqueName := "Unique Test Name For FindByName"
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        uniqueName,
			Description: "Testing name search",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		found, err := repo.FindByName(ctx, uniqueName)
		require.NoError(t, err)
		assert.Equal(t, dateframe.ID, found.ID)
		assert.Equal(t, uniqueName, found.Name)
	})

	t.Run("finds dateframe by name case insensitive", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "CaseSensitive Test",
			Description: "Test case sensitivity",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		found, err := repo.FindByName(ctx, "casesensitive test")
		require.NoError(t, err)
		assert.Equal(t, dateframe.ID, found.ID)
	})

	t.Run("returns error for non-existent name", func(t *testing.T) {
		_, err := repo.FindByName(ctx, "NonExistentDateframeName12345")
		require.Error(t, err)
	})
}

func TestDateframeRepository_FindByDate(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Dateframe
	ctx := context.Background()

	t.Run("finds dateframes containing specific date", func(t *testing.T) {
		startDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 8, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "Summer Period",
			Description: "June to August",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		// Check a date in the middle
		checkDate := time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)
		dateframes, err := repo.FindByDate(ctx, checkDate)
		require.NoError(t, err)

		var found bool
		for _, df := range dateframes {
			if df.ID == dateframe.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("finds dateframes on boundary dates", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "January Period",
			Description: "Full month",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		// Check start date
		dateframes, err := repo.FindByDate(ctx, startDate)
		require.NoError(t, err)
		assert.NotEmpty(t, dateframes)

		// Check end date
		dateframes, err = repo.FindByDate(ctx, endDate)
		require.NoError(t, err)
		assert.NotEmpty(t, dateframes)
	})

	t.Run("returns empty for date outside range", func(t *testing.T) {
		startDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 8, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "Limited Period",
			Description: "Summer only",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		// Check a date way outside the range
		checkDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
		dateframes, err := repo.FindByDate(ctx, checkDate)
		require.NoError(t, err)

		var found bool
		for _, df := range dateframes {
			if df.ID == dateframe.ID {
				found = true
				break
			}
		}
		assert.False(t, found)
	})
}

func TestDateframeRepository_FindOverlapping(t *testing.T) {

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Dateframe
	ctx := context.Background()

	t.Run("finds overlapping dateframes", func(t *testing.T) {
		startDate := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 8, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "Summer Session",
			Description: "Overlap test",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		// Search for range that overlaps
		searchStart := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
		searchEnd := time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC)

		dateframes, err := repo.FindOverlapping(ctx, searchStart, searchEnd)
		require.NoError(t, err)

		var found bool
		for _, df := range dateframes {
			if df.ID == dateframe.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("finds dateframes with partial overlap", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "First Half",
			Description: "Partial overlap",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		// Search overlaps with end of dateframe
		searchStart := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		searchEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

		dateframes, err := repo.FindOverlapping(ctx, searchStart, searchEnd)
		require.NoError(t, err)

		var found bool
		for _, df := range dateframes {
			if df.ID == dateframe.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("does not find non-overlapping dateframes", func(t *testing.T) {
		startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		endDate := time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC)
		dateframe := &schedule.Dateframe{
			StartDate:   startDate,
			EndDate:     endDate,
			Name:        "Q1 Only",
			Description: "No overlap test",
		}
		err := repo.Create(ctx, dateframe)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.dateframes", dateframe.ID)

		// Search for completely different time range
		searchStart := time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)
		searchEnd := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

		dateframes, err := repo.FindOverlapping(ctx, searchStart, searchEnd)
		require.NoError(t, err)

		var found bool
		for _, df := range dateframes {
			if df.ID == dateframe.ID {
				found = true
				break
			}
		}
		assert.False(t, found)
	})
}
