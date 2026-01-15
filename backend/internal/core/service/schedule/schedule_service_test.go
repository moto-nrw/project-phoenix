package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/internal/core/service/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupScheduleService creates a schedule service with real database connection.
func setupScheduleService(t *testing.T, db *bun.DB) scheduleSvc.Service {
	t.Helper()

	repoFactory := repositories.NewFactory(db)

	return scheduleSvc.NewService(
		repoFactory.Dateframe,
		repoFactory.Timeframe,
		repoFactory.RecurrenceRule,
		db,
	)
}

// ============================================================================
// Test Fixtures - Schedule Domain
// ============================================================================

// createTestDateframe creates a test dateframe in the database
func createTestDateframe(t *testing.T, db *bun.DB, name string, startDate, endDate time.Time) *schedule.Dateframe {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	df := &schedule.Dateframe{
		Name:        name,
		StartDate:   startDate,
		EndDate:     endDate,
		Description: "Test dateframe: " + name,
	}

	err := db.NewInsert().
		Model(df).
		ModelTableExpr(`schedule.dateframes`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test dateframe")

	return df
}

// createTestTimeframe creates a test timeframe in the database
func createTestTimeframe(t *testing.T, db *bun.DB, startTime time.Time, endTime *time.Time, isActive bool) *schedule.Timeframe {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tf := &schedule.Timeframe{
		StartTime:   startTime,
		EndTime:     endTime,
		IsActive:    isActive,
		Description: "Test timeframe",
	}

	err := db.NewInsert().
		Model(tf).
		ModelTableExpr(`schedule.timeframes`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test timeframe")

	return tf
}

// createTestRecurrenceRule creates a test recurrence rule in the database
func createTestRecurrenceRule(t *testing.T, db *bun.DB, frequency string, intervalCount int) *schedule.RecurrenceRule {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rr := &schedule.RecurrenceRule{
		Frequency:     frequency,
		IntervalCount: intervalCount,
	}

	err := db.NewInsert().
		Model(rr).
		ModelTableExpr(`schedule.recurrence_rules`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test recurrence rule")

	return rr
}

// cleanupScheduleFixtures removes schedule-related test fixtures
func cleanupScheduleFixtures(t *testing.T, db *bun.DB, dateframeIDs, timeframeIDs, recurrenceRuleIDs []int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, id := range dateframeIDs {
		_, _ = db.NewDelete().
			Model((*schedule.Dateframe)(nil)).
			ModelTableExpr(`schedule.dateframes`).
			Where("id = ?", id).
			Exec(ctx)
	}

	for _, id := range timeframeIDs {
		_, _ = db.NewDelete().
			Model((*schedule.Timeframe)(nil)).
			ModelTableExpr(`schedule.timeframes`).
			Where("id = ?", id).
			Exec(ctx)
	}

	for _, id := range recurrenceRuleIDs {
		_, _ = db.NewDelete().
			Model((*schedule.RecurrenceRule)(nil)).
			ModelTableExpr(`schedule.recurrence_rules`).
			Where("id = ?", id).
			Exec(ctx)
	}
}

// ============================================================================
// Dateframe Tests
// ============================================================================

func TestScheduleService_GetDateframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("returns dateframe for valid ID", func(t *testing.T) {
		// ARRANGE
		startDate := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		endDate := startDate.AddDate(0, 1, 0)
		df := createTestDateframe(t, db, "GetTest", startDate, endDate)
		defer cleanupScheduleFixtures(t, db, []int64{df.ID}, nil, nil)

		// ACT
		result, err := service.GetDateframe(ctx, df.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, df.ID, result.ID)
		assert.Equal(t, "GetTest", result.Name)
	})

	t.Run("returns error for non-existent dateframe", func(t *testing.T) {
		// ACT
		_, err := service.GetDateframe(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestScheduleService_CreateDateframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("creates dateframe successfully", func(t *testing.T) {
		// ARRANGE
		startDate := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		endDate := startDate.AddDate(0, 1, 0)
		df := &schedule.Dateframe{
			Name:        "CreateTest-" + time.Now().Format("20060102150405"),
			StartDate:   startDate,
			EndDate:     endDate,
			Description: "Test creation",
		}

		// ACT
		err := service.CreateDateframe(ctx, df)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, df.ID)

		defer cleanupScheduleFixtures(t, db, []int64{df.ID}, nil, nil)

		// Verify it was created
		retrieved, err := service.GetDateframe(ctx, df.ID)
		require.NoError(t, err)
		assert.Equal(t, df.Name, retrieved.Name)
	})

	t.Run("rejects dateframe with end before start", func(t *testing.T) {
		// ARRANGE
		startDate := time.Now().AddDate(0, 1, 0).Truncate(24 * time.Hour)
		endDate := time.Now().Truncate(24 * time.Hour) // End before start
		df := &schedule.Dateframe{
			Name:      "InvalidDateRange",
			StartDate: startDate,
			EndDate:   endDate,
		}

		// ACT
		err := service.CreateDateframe(ctx, df)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects dateframe with zero start date", func(t *testing.T) {
		// ARRANGE
		df := &schedule.Dateframe{
			Name:    "MissingStart",
			EndDate: time.Now().AddDate(0, 1, 0),
		}

		// ACT
		err := service.CreateDateframe(ctx, df)

		// ASSERT
		require.Error(t, err)
	})
}

func TestScheduleService_UpdateDateframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("updates dateframe successfully", func(t *testing.T) {
		// ARRANGE
		startDate := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		endDate := startDate.AddDate(0, 1, 0)
		df := createTestDateframe(t, db, "UpdateTest", startDate, endDate)
		defer cleanupScheduleFixtures(t, db, []int64{df.ID}, nil, nil)

		// Update description
		df.Description = "Updated description"

		// ACT
		err := service.UpdateDateframe(ctx, df)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		retrieved, err := service.GetDateframe(ctx, df.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", retrieved.Description)
	})
}

func TestScheduleService_DeleteDateframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("deletes dateframe successfully", func(t *testing.T) {
		// ARRANGE
		startDate := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		endDate := startDate.AddDate(0, 1, 0)
		df := createTestDateframe(t, db, "DeleteTest", startDate, endDate)
		dfID := df.ID

		// ACT
		err := service.DeleteDateframe(ctx, dfID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetDateframe(ctx, dfID)
		require.Error(t, err)
	})
}

func TestScheduleService_ListDateframes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("lists all dateframes", func(t *testing.T) {
		// ARRANGE
		startDate := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		df1 := createTestDateframe(t, db, "List1", startDate, startDate.AddDate(0, 1, 0))
		df2 := createTestDateframe(t, db, "List2", startDate.AddDate(0, 2, 0), startDate.AddDate(0, 3, 0))
		defer cleanupScheduleFixtures(t, db, []int64{df1.ID, df2.ID}, nil, nil)

		// ACT
		dateframes, err := service.ListDateframes(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, dateframes)

		// Verify our dateframes are in the list
		found1, found2 := false, false
		for _, df := range dateframes {
			if df.ID == df1.ID {
				found1 = true
			}
			if df.ID == df2.ID {
				found2 = true
			}
		}
		assert.True(t, found1, "df1 should be in list")
		assert.True(t, found2, "df2 should be in list")
	})
}

func TestScheduleService_FindDateframesByDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("finds dateframes containing a specific date", func(t *testing.T) {
		// ARRANGE
		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.AddDate(0, 0, -5)
		endDate := today.AddDate(0, 0, 5)
		df := createTestDateframe(t, db, "ContainsToday", startDate, endDate)
		defer cleanupScheduleFixtures(t, db, []int64{df.ID}, nil, nil)

		// ACT
		dateframes, err := service.FindDateframesByDate(ctx, today)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, dateframes)

		// Verify our dateframe is found
		found := false
		for _, d := range dateframes {
			if d.ID == df.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Dateframe should be found")
	})
}

func TestScheduleService_FindOverlappingDateframes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("finds overlapping dateframes", func(t *testing.T) {
		// ARRANGE
		today := time.Now().Truncate(24 * time.Hour)
		df := createTestDateframe(t, db, "Overlapping", today, today.AddDate(0, 1, 0))
		defer cleanupScheduleFixtures(t, db, []int64{df.ID}, nil, nil)

		// ACT - Find dateframes overlapping with a range that includes our dateframe
		searchStart := today.AddDate(0, 0, -5)
		searchEnd := today.AddDate(0, 0, 15)
		dateframes, err := service.FindOverlappingDateframes(ctx, searchStart, searchEnd)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, dateframes)
	})

	t.Run("rejects invalid date range", func(t *testing.T) {
		// ACT - End before start
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, 10)
		_, err := service.FindOverlappingDateframes(ctx, startDate, endDate)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})
}

// ============================================================================
// Timeframe Tests
// ============================================================================

func TestScheduleService_GetTimeframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("returns timeframe for valid ID", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime := startTime.Add(2 * time.Hour)
		tf := createTestTimeframe(t, db, startTime, &endTime, true)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf.ID}, nil)

		// ACT
		result, err := service.GetTimeframe(ctx, tf.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, tf.ID, result.ID)
	})

	t.Run("returns error for non-existent timeframe", func(t *testing.T) {
		// ACT
		_, err := service.GetTimeframe(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestScheduleService_CreateTimeframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("creates timeframe successfully", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime := startTime.Add(2 * time.Hour)
		tf := &schedule.Timeframe{
			StartTime:   startTime,
			EndTime:     &endTime,
			IsActive:    true,
			Description: "Test creation",
		}

		// ACT
		err := service.CreateTimeframe(ctx, tf)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, tf.ID)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf.ID}, nil)
	})

	t.Run("creates open-ended timeframe", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		tf := &schedule.Timeframe{
			StartTime: startTime,
			EndTime:   nil, // Open-ended
			IsActive:  true,
		}

		// ACT
		err := service.CreateTimeframe(ctx, tf)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, tf.ID)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf.ID}, nil)
	})

	t.Run("rejects timeframe with end before start", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(2 * time.Hour)
		endTime := time.Now().Add(1 * time.Hour) // End before start
		tf := &schedule.Timeframe{
			StartTime: startTime,
			EndTime:   &endTime,
		}

		// ACT
		err := service.CreateTimeframe(ctx, tf)

		// ASSERT
		require.Error(t, err)
	})
}

func TestScheduleService_UpdateTimeframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("updates timeframe successfully", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime := startTime.Add(2 * time.Hour)
		tf := createTestTimeframe(t, db, startTime, &endTime, false)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf.ID}, nil)

		// Update
		tf.IsActive = true
		tf.Description = "Updated"

		// ACT
		err := service.UpdateTimeframe(ctx, tf)

		// ASSERT
		require.NoError(t, err)

		// Verify
		retrieved, err := service.GetTimeframe(ctx, tf.ID)
		require.NoError(t, err)
		assert.True(t, retrieved.IsActive)
		assert.Equal(t, "Updated", retrieved.Description)
	})
}

func TestScheduleService_DeleteTimeframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("deletes timeframe successfully", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime := startTime.Add(2 * time.Hour)
		tf := createTestTimeframe(t, db, startTime, &endTime, true)
		tfID := tf.ID

		// ACT
		err := service.DeleteTimeframe(ctx, tfID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetTimeframe(ctx, tfID)
		require.Error(t, err)
	})
}

func TestScheduleService_ListTimeframes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("lists all timeframes", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime1 := startTime.Add(2 * time.Hour)
		endTime2 := startTime.Add(3 * time.Hour)
		tf1 := createTestTimeframe(t, db, startTime, &endTime1, true)
		tf2 := createTestTimeframe(t, db, startTime.Add(4*time.Hour), &endTime2, false)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf1.ID, tf2.ID}, nil)

		// ACT
		timeframes, err := service.ListTimeframes(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, timeframes)
	})
}

func TestScheduleService_FindActiveTimeframes(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("finds only active timeframes", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime := startTime.Add(2 * time.Hour)
		activeTF := createTestTimeframe(t, db, startTime, &endTime, true)
		inactiveTF := createTestTimeframe(t, db, startTime.Add(3*time.Hour), &endTime, false)
		defer cleanupScheduleFixtures(t, db, nil, []int64{activeTF.ID, inactiveTF.ID}, nil)

		// ACT
		timeframes, err := service.FindActiveTimeframes(ctx)

		// ASSERT
		require.NoError(t, err)

		// Active timeframe should be found
		foundActive := false
		for _, tf := range timeframes {
			if tf.ID == activeTF.ID {
				foundActive = true
				assert.True(t, tf.IsActive)
			}
			// Inactive should not be in list
			assert.NotEqual(t, inactiveTF.ID, tf.ID, "Inactive timeframe should not be in list")
		}
		assert.True(t, foundActive, "Active timeframe should be found")
	})
}

func TestScheduleService_FindTimeframesByTimeRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("finds timeframes in range", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime := startTime.Add(2 * time.Hour)
		tf := createTestTimeframe(t, db, startTime, &endTime, true)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf.ID}, nil)

		// ACT
		searchStart := startTime.Add(-1 * time.Hour)
		searchEnd := endTime.Add(1 * time.Hour)
		timeframes, err := service.FindTimeframesByTimeRange(ctx, searchStart, searchEnd)

		// ASSERT
		require.NoError(t, err)
		// Our timeframe should be in the results
		found := false
		for _, t := range timeframes {
			if t.ID == tf.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Timeframe should be found in range")
	})

	t.Run("rejects invalid time range", func(t *testing.T) {
		// ACT - End before start
		endTime := time.Now()
		startTime := endTime.Add(1 * time.Hour)
		_, err := service.FindTimeframesByTimeRange(ctx, startTime, endTime)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
	})
}

// ============================================================================
// RecurrenceRule Tests
// ============================================================================

func TestScheduleService_GetRecurrenceRule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("returns recurrence rule for valid ID", func(t *testing.T) {
		// ARRANGE
		rr := createTestRecurrenceRule(t, db, schedule.FrequencyWeekly, 1)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// ACT
		result, err := service.GetRecurrenceRule(ctx, rr.ID)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, rr.ID, result.ID)
		assert.Equal(t, schedule.FrequencyWeekly, result.Frequency)
	})

	t.Run("returns error for non-existent rule", func(t *testing.T) {
		// ACT
		_, err := service.GetRecurrenceRule(ctx, 999999999)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestScheduleService_CreateRecurrenceRule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("creates daily rule successfully", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 1,
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, rr.ID)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})
	})

	t.Run("creates weekly rule with weekdays", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"MON", "WED", "FRI"},
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, rr.ID)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})
	})

	t.Run("creates monthly rule with month days", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyMonthly,
			IntervalCount: 1,
			MonthDays:     []int{1, 15},
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, rr.ID)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})
	})

	t.Run("creates rule with count limit", func(t *testing.T) {
		// ARRANGE
		count := 10
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 1,
			Count:         &count,
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, rr.ID)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})
	})

	t.Run("creates rule with end date", func(t *testing.T) {
		// ARRANGE
		endDate := time.Now().AddDate(0, 1, 0)
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 1,
			EndDate:       &endDate,
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.NoError(t, err)
		assert.NotZero(t, rr.ID)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})
	})

	t.Run("rejects invalid frequency", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     "invalid",
			IntervalCount: 1,
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects zero interval count", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 0,
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects both count and end date", func(t *testing.T) {
		// ARRANGE
		count := 10
		endDate := time.Now().AddDate(0, 1, 0)
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 1,
			Count:         &count,
			EndDate:       &endDate,
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects invalid weekday", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"INVALID"},
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects invalid month day", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyMonthly,
			IntervalCount: 1,
			MonthDays:     []int{32}, // Invalid - max is 31
		}

		// ACT
		err := service.CreateRecurrenceRule(ctx, rr)

		// ASSERT
		require.Error(t, err)
	})
}

func TestScheduleService_UpdateRecurrenceRule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("updates recurrence rule successfully", func(t *testing.T) {
		// ARRANGE
		rr := createTestRecurrenceRule(t, db, schedule.FrequencyDaily, 1)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// Update
		rr.IntervalCount = 2

		// ACT
		err := service.UpdateRecurrenceRule(ctx, rr)

		// ASSERT
		require.NoError(t, err)

		// Verify
		retrieved, err := service.GetRecurrenceRule(ctx, rr.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, retrieved.IntervalCount)
	})
}

func TestScheduleService_DeleteRecurrenceRule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("deletes recurrence rule successfully", func(t *testing.T) {
		// ARRANGE
		rr := createTestRecurrenceRule(t, db, schedule.FrequencyDaily, 1)
		rrID := rr.ID

		// ACT
		err := service.DeleteRecurrenceRule(ctx, rrID)

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetRecurrenceRule(ctx, rrID)
		require.Error(t, err)
	})
}

func TestScheduleService_ListRecurrenceRules(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("lists all recurrence rules", func(t *testing.T) {
		// ARRANGE
		rr1 := createTestRecurrenceRule(t, db, schedule.FrequencyDaily, 1)
		rr2 := createTestRecurrenceRule(t, db, schedule.FrequencyWeekly, 2)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr1.ID, rr2.ID})

		// ACT
		rules, err := service.ListRecurrenceRules(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, rules)
	})
}

func TestScheduleService_FindRecurrenceRulesByFrequency(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("finds rules by frequency", func(t *testing.T) {
		// ARRANGE
		dailyRR := createTestRecurrenceRule(t, db, schedule.FrequencyDaily, 1)
		weeklyRR := createTestRecurrenceRule(t, db, schedule.FrequencyWeekly, 1)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{dailyRR.ID, weeklyRR.ID})

		// ACT
		rules, err := service.FindRecurrenceRulesByFrequency(ctx, schedule.FrequencyDaily)

		// ASSERT
		require.NoError(t, err)

		// Daily rule should be found, weekly should not
		foundDaily := false
		for _, r := range rules {
			if r.ID == dailyRR.ID {
				foundDaily = true
			}
			assert.NotEqual(t, weeklyRR.ID, r.ID, "Weekly rule should not be in daily results")
		}
		assert.True(t, foundDaily, "Daily rule should be found")
	})
}

func TestScheduleService_FindRecurrenceRulesByWeekday(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("finds rules by weekday", func(t *testing.T) {
		// ARRANGE - Create rule with weekdays via service
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"MON", "WED"},
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// ACT
		rules, err := service.FindRecurrenceRulesByWeekday(ctx, "MON")

		// ASSERT
		require.NoError(t, err)

		// Our rule should be found
		found := false
		for _, r := range rules {
			if r.ID == rr.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Rule with MON weekday should be found")
	})
}

// ============================================================================
// Advanced Operations Tests
// ============================================================================

func TestScheduleService_GenerateEvents(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("generates daily events", func(t *testing.T) {
		// ARRANGE
		rr := createTestRecurrenceRule(t, db, schedule.FrequencyDaily, 1)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		startDate := time.Now().Truncate(24 * time.Hour)
		endDate := startDate.AddDate(0, 0, 7) // 1 week

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, events, 8) // 8 days including start and end
	})

	t.Run("generates weekly events", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyWeekly,
			IntervalCount: 1,
			Weekdays:      []string{"MON", "WED", "FRI"},
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// Find a Monday to start
		startDate := time.Now().Truncate(24 * time.Hour)
		for startDate.Weekday() != time.Monday {
			startDate = startDate.AddDate(0, 0, 1)
		}
		endDate := startDate.AddDate(0, 0, 14) // 2 weeks

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		// Should have 6 events (3 per week x 2 weeks)
		assert.GreaterOrEqual(t, len(events), 6)
	})

	t.Run("generates monthly events", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyMonthly,
			IntervalCount: 1,
			MonthDays:     []int{1, 15},
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// Start from the 1st of a month
		now := time.Now()
		startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		endDate := startDate.AddDate(0, 2, 0) // 2 months

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		// Should have events on 1st and 15th of each month
		assert.GreaterOrEqual(t, len(events), 4)
	})

	t.Run("respects count limit", func(t *testing.T) {
		// ARRANGE
		count := 5
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 1,
			Count:         &count,
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		startDate := time.Now().Truncate(24 * time.Hour)
		endDate := startDate.AddDate(0, 0, 30) // 1 month

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		assert.LessOrEqual(t, len(events), 5)
	})

	t.Run("rejects invalid date range", func(t *testing.T) {
		// ARRANGE
		rr := createTestRecurrenceRule(t, db, schedule.FrequencyDaily, 1)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// ACT - End before start
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, 10)
		_, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns empty for expired rule", func(t *testing.T) {
		// ARRANGE
		endDate := time.Now().AddDate(0, 0, -10) // Ended 10 days ago
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 1,
			EndDate:       &endDate,
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// ACT - Search in future
		startDate := time.Now().AddDate(0, 0, 1)
		searchEnd := startDate.AddDate(0, 0, 7)
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, searchEnd)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, events)
	})

	t.Run("returns error for non-existent rule", func(t *testing.T) {
		// ACT
		startDate := time.Now()
		endDate := startDate.AddDate(0, 0, 7)
		_, err := service.GenerateEvents(ctx, 999999999, startDate, endDate)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("generates yearly events", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyYearly,
			IntervalCount: 1,
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// Use a specific date
		startDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		endDate := startDate.AddDate(3, 0, 0) // 3 years

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		// Should have 4 events (2024, 2025, 2026, 2027)
		assert.Len(t, events, 4)
	})

	t.Run("generates yearly events with leap year handling", func(t *testing.T) {
		// ARRANGE
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyYearly,
			IntervalCount: 1,
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// Start on Feb 29 of a leap year
		startDate := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)
		endDate := startDate.AddDate(4, 0, 0) // 4 years

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, events)
	})

	t.Run("generates events with interval > 1", func(t *testing.T) {
		// ARRANGE - Every 2 days
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 2,
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		startDate := time.Now().Truncate(24 * time.Hour)
		endDate := startDate.AddDate(0, 0, 10) // 10 days

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		// Every 2 days for 10 days = 6 events (day 0, 2, 4, 6, 8, 10)
		assert.Len(t, events, 6)
	})

	t.Run("generates weekly events with interval > 1", func(t *testing.T) {
		// ARRANGE - Every 2 weeks on Monday
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyWeekly,
			IntervalCount: 2,
			Weekdays:      []string{"MON"},
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		// Find a Monday to start
		startDate := time.Now().Truncate(24 * time.Hour)
		for startDate.Weekday() != time.Monday {
			startDate = startDate.AddDate(0, 0, 1)
		}
		endDate := startDate.AddDate(0, 0, 35) // 5 weeks

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		// Every 2 weeks = 3 events (week 0, 2, 4)
		assert.GreaterOrEqual(t, len(events), 2)
	})

	t.Run("generates events respecting rule end date", func(t *testing.T) {
		// ARRANGE
		ruleEndDate := time.Now().AddDate(0, 0, 5).Truncate(24 * time.Hour)
		rr := &schedule.RecurrenceRule{
			Frequency:     schedule.FrequencyDaily,
			IntervalCount: 1,
			EndDate:       &ruleEndDate,
		}
		err := service.CreateRecurrenceRule(ctx, rr)
		require.NoError(t, err)
		defer cleanupScheduleFixtures(t, db, nil, nil, []int64{rr.ID})

		startDate := time.Now().Truncate(24 * time.Hour)
		endDate := startDate.AddDate(0, 0, 30) // Request 30 days

		// ACT
		events, err := service.GenerateEvents(ctx, rr.ID, startDate, endDate)

		// ASSERT
		require.NoError(t, err)
		// Should stop at rule end date (5-6 events, not 30)
		assert.LessOrEqual(t, len(events), 7)
	})
}

func TestScheduleService_CheckConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("detects conflict with existing timeframe", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime := startTime.Add(2 * time.Hour)
		tf := createTestTimeframe(t, db, startTime, &endTime, true)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf.ID}, nil)

		// ACT - Check for conflict in overlapping range
		checkStart := startTime.Add(30 * time.Minute)
		checkEnd := startTime.Add(90 * time.Minute)
		hasConflict, conflicts, err := service.CheckConflict(ctx, checkStart, checkEnd)

		// ASSERT
		require.NoError(t, err)
		assert.True(t, hasConflict)
		assert.NotEmpty(t, conflicts)
	})

	t.Run("no conflict with non-overlapping time", func(t *testing.T) {
		// ARRANGE
		startTime := time.Now().Add(1 * time.Hour)
		endTime := startTime.Add(2 * time.Hour)
		tf := createTestTimeframe(t, db, startTime, &endTime, true)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf.ID}, nil)

		// ACT - Check for conflict outside the timeframe
		checkStart := endTime.Add(1 * time.Hour)
		checkEnd := checkStart.Add(1 * time.Hour)
		hasConflict, conflicts, err := service.CheckConflict(ctx, checkStart, checkEnd)

		// ASSERT
		require.NoError(t, err)
		assert.False(t, hasConflict)
		assert.Empty(t, conflicts)
	})

	t.Run("rejects invalid time range", func(t *testing.T) {
		// ACT
		endTime := time.Now()
		startTime := endTime.Add(1 * time.Hour)
		_, _, err := service.CheckConflict(ctx, startTime, endTime)

		// ASSERT
		require.Error(t, err)
	})
}

func TestScheduleService_FindAvailableSlots(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("finds available slots between timeframes", func(t *testing.T) {
		// ARRANGE - Create two timeframes with a gap
		baseTime := time.Now().Add(1 * time.Hour).Truncate(time.Hour)
		endTime1 := baseTime.Add(1 * time.Hour)
		startTime2 := baseTime.Add(3 * time.Hour)
		endTime2 := baseTime.Add(4 * time.Hour)

		tf1 := createTestTimeframe(t, db, baseTime, &endTime1, true)
		tf2 := createTestTimeframe(t, db, startTime2, &endTime2, true)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf1.ID, tf2.ID}, nil)

		// ACT - Find slots of at least 1 hour
		searchStart := baseTime.Add(-1 * time.Hour)
		searchEnd := baseTime.Add(5 * time.Hour)
		slots, err := service.FindAvailableSlots(ctx, searchStart, searchEnd, 1*time.Hour)

		// ASSERT
		require.NoError(t, err)
		// Should find slots in gaps
		assert.NotEmpty(t, slots)
	})

	t.Run("returns empty when no slots available", func(t *testing.T) {
		// ARRANGE - Create a continuous timeframe
		startTime := time.Now().Add(1 * time.Hour).Truncate(time.Hour)
		endTime := startTime.Add(10 * time.Hour)
		tf := createTestTimeframe(t, db, startTime, &endTime, true)
		defer cleanupScheduleFixtures(t, db, nil, []int64{tf.ID}, nil)

		// ACT - Find slots that don't fit
		searchStart := startTime
		searchEnd := endTime
		slots, err := service.FindAvailableSlots(ctx, searchStart, searchEnd, 5*time.Hour)

		// ASSERT
		require.NoError(t, err)
		// No slots of 5 hours available since the timeframe is occupied
		assert.Empty(t, slots)
	})

	t.Run("rejects invalid date range", func(t *testing.T) {
		// ACT
		endDate := time.Now()
		startDate := endDate.Add(1 * time.Hour)
		_, err := service.FindAvailableSlots(ctx, startDate, endDate, 1*time.Hour)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("rejects invalid duration", func(t *testing.T) {
		// ACT
		startDate := time.Now()
		endDate := startDate.Add(5 * time.Hour)
		_, err := service.FindAvailableSlots(ctx, startDate, endDate, 0)

		// ASSERT
		require.Error(t, err)
	})
}

func TestScheduleService_GetCurrentDateframe(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("returns current dateframe when exists", func(t *testing.T) {
		// ARRANGE - Create a dateframe that includes today
		today := time.Now().Truncate(24 * time.Hour)
		startDate := today.AddDate(0, 0, -5)
		endDate := today.AddDate(0, 0, 5)
		df := createTestDateframe(t, db, "Current", startDate, endDate)
		defer cleanupScheduleFixtures(t, db, []int64{df.ID}, nil, nil)

		// ACT
		result, err := service.GetCurrentDateframe(ctx)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
	})
}

// ============================================================================
// Transaction Tests
// ============================================================================

func TestScheduleService_WithTx(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupScheduleService(t, db)
	ctx := context.Background()

	t.Run("WithTx returns transactional service", func(t *testing.T) {
		// ARRANGE
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// ACT
		txService := service.WithTx(tx)

		// ASSERT
		require.NotNil(t, txService)
		_, ok := txService.(scheduleSvc.Service)
		assert.True(t, ok, "WithTx should return a Service interface")
	})
}
