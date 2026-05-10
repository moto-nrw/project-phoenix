package schedule_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupCalendarPeriodService(t *testing.T, db *bun.DB) schedule.CalendarPeriodService {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db, slog.Default())
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.CalendarPeriod
}

// =============================================================================
// GetAllPeriods Tests
// =============================================================================

func TestCalendarPeriodService_GetAllPeriods(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns all periods for tenant", func(t *testing.T) {
		suffix := time.Now().UnixNano()
		p1 := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("GetAll-A-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, p1)
		require.NoError(t, err)

		p2 := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("GetAll-B-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        false,
		}
		err = svc.CreatePeriod(ctx, p2)
		require.NoError(t, err)

		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", p1.ID, p2.ID)

		periods, err := svc.GetAllPeriods(ctx)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(periods), 2)
	})
}

// =============================================================================
// GetActivePeriods Tests
// =============================================================================

func TestCalendarPeriodService_GetActivePeriods(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns only active periods", func(t *testing.T) {
		suffix := time.Now().UnixNano()
		active := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Active-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, active)
		require.NoError(t, err)

		inactive := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Inactive-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        false,
		}
		err = svc.CreatePeriod(ctx, inactive)
		require.NoError(t, err)

		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", active.ID, inactive.ID)

		periods, err := svc.GetActivePeriods(ctx)

		require.NoError(t, err)
		for _, p := range periods {
			assert.True(t, p.IsActive, "all returned periods should be active")
		}
	})
}

// =============================================================================
// GetPeriodByID Tests
// =============================================================================

func TestCalendarPeriodService_GetPeriodByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns period by ID", func(t *testing.T) {
		name := fmt.Sprintf("GetByID-%d", time.Now().UnixNano())
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, period)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)

		found, err := svc.GetPeriodByID(ctx, period.ID)

		require.NoError(t, err)
		assert.Equal(t, period.ID, found.ID)
		assert.Equal(t, name, found.Name)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := svc.GetPeriodByID(ctx, int64(999999999))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "schedule error")
	})
}

// =============================================================================
// CreatePeriod Tests
// =============================================================================

func TestCalendarPeriodService_CreatePeriod(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("creates period successfully", func(t *testing.T) {
		name := fmt.Sprintf("Create-%d", time.Now().UnixNano())
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}

		err := svc.CreatePeriod(ctx, period)

		require.NoError(t, err)
		assert.Greater(t, period.ID, int64(0))
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
	})

	t.Run("creates period with week cycle", func(t *testing.T) {
		name := fmt.Sprintf("Create-Cycle-%d", time.Now().UnixNano())
		anchor := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 2,
			WeekCycleAnchor: &anchor,
			IsActive:        true,
		}

		err := svc.CreatePeriod(ctx, period)

		require.NoError(t, err)
		assert.Greater(t, period.ID, int64(0))
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
	})

	t.Run("rejects duplicate name", func(t *testing.T) {
		name := fmt.Sprintf("Duplicate-%d", time.Now().UnixNano())
		first := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, first)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", first.ID)

		second := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err = svc.CreatePeriod(ctx, second)

		require.Error(t, err)
		assert.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodNameConflict)
	})

	t.Run("rejects invalid period data", func(t *testing.T) {
		period := &scheduleModels.CalendarPeriod{
			Name:            "",
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
		}

		err := svc.CreatePeriod(ctx, period)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("rejects end date before start date", func(t *testing.T) {
		period := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("BadDates-%d", time.Now().UnixNano()),
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2025, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
		}

		err := svc.CreatePeriod(ctx, period)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "end_date must be after start_date")
	})
}

// =============================================================================
// UpdatePeriod Tests
// =============================================================================

func TestCalendarPeriodService_UpdatePeriod(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("updates period successfully", func(t *testing.T) {
		name := fmt.Sprintf("Update-%d", time.Now().UnixNano())
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, period)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)

		newName := fmt.Sprintf("Updated-%d", time.Now().UnixNano())
		period.Name = newName
		period.PeriodType = scheduleModels.PeriodTypeSemester
		period.IsActive = false

		err = svc.UpdatePeriod(ctx, period)

		require.NoError(t, err)

		found, err := svc.GetPeriodByID(ctx, period.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, found.Name)
		assert.Equal(t, scheduleModels.PeriodTypeSemester, found.PeriodType)
		assert.False(t, found.IsActive)
	})

	t.Run("allows updating with same name (self)", func(t *testing.T) {
		name := fmt.Sprintf("SameName-%d", time.Now().UnixNano())
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, period)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)

		period.IsActive = false
		err = svc.UpdatePeriod(ctx, period)

		require.NoError(t, err)
	})

	t.Run("rejects duplicate name from another period", func(t *testing.T) {
		suffix := time.Now().UnixNano()
		first := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("First-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, first)
		require.NoError(t, err)

		second := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Second-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err = svc.CreatePeriod(ctx, second)
		require.NoError(t, err)

		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", first.ID, second.ID)

		second.Name = first.Name
		err = svc.UpdatePeriod(ctx, second)

		require.Error(t, err)
		assert.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodNameConflict)
	})

	t.Run("rejects invalid update data", func(t *testing.T) {
		name := fmt.Sprintf("InvalidUpdate-%d", time.Now().UnixNano())
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, period)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)

		period.Name = ""
		err = svc.UpdatePeriod(ctx, period)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
}

// =============================================================================
// DeletePeriod Tests
// =============================================================================

func TestCalendarPeriodService_DeletePeriod(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes period successfully", func(t *testing.T) {
		name := fmt.Sprintf("Delete-%d", time.Now().UnixNano())
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC),
			EndDate:         time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, period)
		require.NoError(t, err)

		err = svc.DeletePeriod(ctx, period.ID)

		require.NoError(t, err)

		_, err = svc.GetPeriodByID(ctx, period.ID)
		assert.Error(t, err)
	})
}
