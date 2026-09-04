package activities_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
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

	ctx := testpkg.Ctx(t)
	schedule := &activities.Schedule{
		ActivityGroupID: groupID,
		Weekday:         weekday,
		TimeframeID:     timeframeID,
	}
	schedule.SetTenantID(testpkg.Tenant(t))

	err := db.NewInsert().
		Model(schedule).
		ModelTableExpr(`activities.schedules AS "schedule"`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test schedule")

	return schedule
}

func scheduleRepository(t *testing.T, db *bun.DB) activities.ScheduleRepository {
	t.Helper()
	factory := repositories.NewFactory(db)
	factory.BindTimetable(timetabletest.New(t, db))
	return factory.ActivitySchedule
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestScheduleRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates schedule with valid data", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "ScheduleGroup")

		schedule := &activities.Schedule{
			ActivityGroupID: group.ID,
			Weekday:         2, // Tuesday
		}

		err := repo.Create(ctx, schedule)
		require.NoError(t, err)
		assert.NotZero(t, schedule.ID)

	})

	t.Run("creates schedule without timeframe", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "NoTimeframeGroup")

		schedule := &activities.Schedule{
			ActivityGroupID: group.ID,
			Weekday:         3,
			TimeframeID:     nil, // No timeframe
		}

		err := repo.Create(ctx, schedule)
		require.NoError(t, err)
		assert.NotZero(t, schedule.ID)
		assert.Nil(t, schedule.TimeframeID)

	})
}

func TestScheduleRepository_Create_WithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when schedule is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestScheduleRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds existing schedule", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "FindByID")

		schedule := createSchedule(t, db, group.ID, 1, nil)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates schedule weekday", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Update")

		schedule := createSchedule(t, db, group.ID, 1, nil)

		schedule.Weekday = 5
		err := repo.Update(ctx, schedule)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, schedule.ID)
		require.NoError(t, err)
		assert.Equal(t, 5, found.Weekday)
		assert.Equal(t, found.UpdatedAt, schedule.UpdatedAt)
	})
}

func TestScheduleRepository_Update_WithNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when schedule is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestScheduleRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes existing schedule", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "Delete")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all schedules", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "List")

		createSchedule(t, db, group.ID, 1, nil)

		schedules, err := repo.FindByGroupIDs(ctx, []int64{group.ID})
		require.NoError(t, err)
		assert.NotEmpty(t, schedules)
	})

	t.Run("empty group IDs return no schedules", func(t *testing.T) {
		schedules, err := repo.FindByGroupIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, schedules)
	})
}

func TestScheduleRepository_FindByGroupID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds schedules for a specific group", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "GroupSchedules")

		schedule1 := createSchedule(t, db, group.ID, 1, nil) // Monday
		schedule2 := createSchedule(t, db, group.ID, 3, nil) // Wednesday

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

		schedules, err := repo.FindByGroupID(ctx, group.ID)
		require.NoError(t, err)
		assert.Empty(t, schedules)
	})
}

func TestScheduleRepository_FindByWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds schedules for a specific weekday", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, db, "WeekdaySchedules")

		schedule := createSchedule(t, db, group.ID, 4, nil) // Thursday

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

// ============================================================================
// Edge Cases
// ============================================================================

func TestScheduleRepository_Delete_NonExistent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("does not error when deleting non-existent schedule", func(t *testing.T) {
		err := repo.Delete(ctx, int64(999999))
		require.NoError(t, err)
	})
}

// ============================================================================
// FindTemplateStartTimesByGroupIDs (WP-B13)
// ============================================================================

// createTestTimeframe inserts a schedule.timeframes row with the given
// timezone-free wall-clock start. Returns the model with its assigned ID.
// The tenant is 1.
func createTestTimeframe(t *testing.T, db *bun.DB, startHour, startMin int) *scheduleModels.Timeframe {
	t.Helper()
	start := time.Date(2000, 1, 1, startHour, startMin, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	tf := &scheduleModels.Timeframe{
		StartTime:   start,
		EndTime:     &end,
		IsActive:    true,
		Description: fmt.Sprintf("tf-%02d%02d-%d", startHour, startMin, time.Now().UnixNano()),
	}
	tf.SetTenantID(testpkg.Tenant(t))

	ctx := testpkg.Ctx(t)
	_, err := db.NewInsert().
		Model(tf).
		ModelTableExpr(`schedule.timeframes AS "timeframe"`).
		Returning("id").
		Exec(ctx)
	require.NoError(t, err)
	return tf
}

func TestScheduleRepository_FindTemplateStartTimesByGroupIDs_ReturnsTimes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	group1 := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("B13TplA-%d", time.Now().UnixNano()))
	group2 := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("B13TplB-%d", time.Now().UnixNano()+1))

	tf14 := createTestTimeframe(t, db, 14, 0)
	tf15 := createTestTimeframe(t, db, 15, 30)
	tf10 := createTestTimeframe(t, db, 10, 0)

	// group1 Monday 14:00, group1 Friday 15:30, group2 Monday 10:00, plus one
	// schedule without a timeframe (must be skipped).
	createSchedule(t, db, group1.ID, scheduleModels.WeekdayMonday, &tf14.ID)
	createSchedule(t, db, group1.ID, scheduleModels.WeekdayFriday, &tf15.ID)
	createSchedule(t, db, group2.ID, scheduleModels.WeekdayMonday, &tf10.ID)
	createSchedule(t, db, group1.ID, scheduleModels.WeekdayTuesday, nil)

	rows, err := repo.FindTemplateStartTimesByGroupIDs(ctx, []int64{group1.ID, group2.ID})
	require.NoError(t, err)

	type key struct {
		GID     int64
		Weekday int
	}
	got := make(map[key][]time.Time, len(rows))
	for _, r := range rows {
		got[key{r.ActivityGroupID, r.Weekday}] = append(got[key{r.ActivityGroupID, r.Weekday}], r.StartTime)
	}

	require.Len(t, got[key{group1.ID, scheduleModels.WeekdayMonday}], 1)
	assert.Equal(t, 14, got[key{group1.ID, scheduleModels.WeekdayMonday}][0].Hour())

	require.Len(t, got[key{group1.ID, scheduleModels.WeekdayFriday}], 1)
	assert.Equal(t, 15, got[key{group1.ID, scheduleModels.WeekdayFriday}][0].Hour())
	assert.Equal(t, 30, got[key{group1.ID, scheduleModels.WeekdayFriday}][0].Minute())

	require.Len(t, got[key{group2.ID, scheduleModels.WeekdayMonday}], 1)
	assert.Equal(t, 10, got[key{group2.ID, scheduleModels.WeekdayMonday}][0].Hour())

	_, hasTuesday := got[key{group1.ID, scheduleModels.WeekdayTuesday}]
	assert.False(t, hasTuesday, "schedule without timeframe must be skipped by the JOIN")
}

func TestScheduleRepository_FindTemplateStartTimesByGroupIDs_EmptyInput(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	rows, err := repo.FindTemplateStartTimesByGroupIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestScheduleRepository_FindTemplateStartTimesByGroupIDs_AmbiguousWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)
	ctx := testpkg.Ctx(t)

	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("B13Amb-%d", time.Now().UnixNano()))

	tfMorning := createTestTimeframe(t, db, 8, 0)
	tfAfternoon := createTestTimeframe(t, db, 14, 0)

	createSchedule(t, db, group.ID, scheduleModels.WeekdayMonday, &tfMorning.ID)
	createSchedule(t, db, group.ID, scheduleModels.WeekdayMonday, &tfAfternoon.ID)

	rows, err := repo.FindTemplateStartTimesByGroupIDs(ctx, []int64{group.ID})
	require.NoError(t, err)

	mondayRows := 0
	for _, r := range rows {
		if r.ActivityGroupID == group.ID && r.Weekday == scheduleModels.WeekdayMonday {
			mondayRows++
		}
	}
	assert.Equal(t, 2, mondayRows, "both schedules for the same weekday must be returned — caller disambiguates")
}

func TestScheduleRepository_FindTemplateStartTimesByGroupIDs_TenantScoped(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepository(t, db)

	group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("B13Iso-%d", time.Now().UnixNano()))

	tf := createTestTimeframe(t, db, 14, 0)

	createSchedule(t, db, group.ID, scheduleModels.WeekdayMonday, &tf.ID)

	// Tenant 1 sees the row.
	rows, err := repo.FindTemplateStartTimesByGroupIDs(testpkg.Ctx(t), []int64{group.ID})
	require.NoError(t, err)
	assert.NotEmpty(t, rows)

	// Tenant 2 must not.
	rows2, err := repo.FindTemplateStartTimesByGroupIDs(testpkg.TenantContext(2), []int64{group.ID})
	require.NoError(t, err)
	assert.Empty(t, rows2, "schedules from tenant 1 must not leak to tenant 2")
}
