package schedule_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, p1)
		require.NoError(t, err)

		p2 := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("GetAll-B-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 1, 31),
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, active)
		require.NoError(t, err)

		inactive := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Inactive-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 1, 31),
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
		anchor := timezone.NewDate(2025, 9, 1)
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, first)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", first.ID)

		second := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 1, 31),
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
			StartDate:       timezone.NewDate(2026, 8, 1),
			EndDate:         timezone.NewDate(2025, 7, 31),
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

func TestCalendarPeriodService_SameTypeOverlapConflict(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	tenantID := int64(600000) + time.Now().UnixNano()%50000
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			Table("schedule.calendar_periods").
			Where("tenant_id = ?", tenantID).
			Exec(context.Background())
	})
	ctx := testpkg.TenantContext(tenantID)

	makePeriod := func(name, periodType string, start, end timezone.Date, active bool) *scheduleModels.CalendarPeriod {
		return &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
			PeriodType:      periodType,
			StartDate:       start,
			EndDate:         end,
			WeekCycleLength: 1,
			IsActive:        active,
		}
	}

	base := makePeriod("SameType-Basis", scheduleModels.PeriodTypeSemester,
		timezone.NewDate(2035, 8, 1), timezone.NewDate(2036, 1, 31), true)
	require.NoError(t, svc.CreatePeriod(ctx, base))

	t.Run("create rejects an active same-type overlap", func(t *testing.T) {
		overlapping := makePeriod("SameType-Kollision", scheduleModels.PeriodTypeSemester,
			timezone.NewDate(2035, 10, 1), timezone.NewDate(2036, 3, 31), true)
		err := svc.CreatePeriod(ctx, overlapping)
		require.Error(t, err)
		assert.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodOverlapConflict)

		// The typed error exposes the conflicting period so the handler can
		// name it in the 409 message.
		var overlapErr *scheduleModels.CalendarPeriodOverlapError
		require.ErrorAs(t, err, &overlapErr)
		require.Len(t, overlapErr.Overlaps, 1)
		assert.Equal(t, base.ID, overlapErr.Overlaps[0].ID)
		assert.Equal(t, base.Name, overlapErr.Overlaps[0].Name)
	})

	t.Run("create allows an inactive same-type overlap", func(t *testing.T) {
		inactive := makePeriod("SameType-Inaktiv", scheduleModels.PeriodTypeSemester,
			timezone.NewDate(2035, 10, 1), timezone.NewDate(2036, 3, 31), false)
		require.NoError(t, svc.CreatePeriod(ctx, inactive))
	})

	t.Run("create allows an active cross-type overlap", func(t *testing.T) {
		holiday := makePeriod("SameType-Ferien", scheduleModels.PeriodTypeHoliday,
			timezone.NewDate(2035, 10, 1), timezone.NewDate(2035, 10, 14), true)
		require.NoError(t, svc.CreatePeriod(ctx, holiday))
	})

	t.Run("create allows an adjacent same-type period", func(t *testing.T) {
		adjacent := makePeriod("SameType-Angrenzend", scheduleModels.PeriodTypeSemester,
			timezone.NewDate(2036, 2, 1), timezone.NewDate(2036, 7, 31), true)
		require.NoError(t, svc.CreatePeriod(ctx, adjacent))
	})

	t.Run("update rejects a date change into a same-type overlap", func(t *testing.T) {
		mover := makePeriod("SameType-Verschoben", scheduleModels.PeriodTypeSemester,
			timezone.NewDate(2037, 8, 1), timezone.NewDate(2038, 1, 31), true)
		require.NoError(t, svc.CreatePeriod(ctx, mover))

		mover.StartDate = timezone.NewDate(2035, 12, 1)
		mover.EndDate = timezone.NewDate(2036, 1, 15)
		err := svc.UpdatePeriod(ctx, mover)
		require.Error(t, err)
		assert.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodOverlapConflict)
	})

	t.Run("update rejects activating an overlapping same-type period", func(t *testing.T) {
		sleeper := makePeriod("SameType-Schlafend", scheduleModels.PeriodTypeSemester,
			timezone.NewDate(2035, 9, 1), timezone.NewDate(2035, 12, 31), false)
		require.NoError(t, svc.CreatePeriod(ctx, sleeper))

		sleeper.IsActive = true
		err := svc.UpdatePeriod(ctx, sleeper)
		require.Error(t, err)
		assert.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodOverlapConflict)
	})

	t.Run("rename-only update of a legacy overlapper stays allowed", func(t *testing.T) {
		// Seed the overlap at the repository level, bypassing the service
		// guard — this is the pre-rule legacy data shape.
		repo := repositories.NewFactory(db).CalendarPeriod
		legacy := makePeriod("SameType-Bestand", scheduleModels.PeriodTypeSemester,
			timezone.NewDate(2035, 9, 1), timezone.NewDate(2035, 11, 30), true)
		legacy.SetTenantID(tenantID)
		require.NoError(t, repo.Create(ctx, legacy))

		legacy.Name = fmt.Sprintf("SameType-Bestand-Umbenannt-%d", time.Now().UnixNano())
		require.NoError(t, svc.UpdatePeriod(ctx, legacy),
			"rename-only edits must not trip the overlap guard on pre-existing overlaps")
	})

	t.Run("update rejects a type change into a same-type overlap", func(t *testing.T) {
		// A holiday inside the base semester is a legal cross-type overlap …
		retyped := makePeriod("SameType-Umgetypt", scheduleModels.PeriodTypeHoliday,
			timezone.NewDate(2035, 11, 5), timezone.NewDate(2035, 11, 15), true)
		require.NoError(t, svc.CreatePeriod(ctx, retyped))

		// … but re-typing it to semester collides with the active base semester.
		retyped.PeriodType = scheduleModels.PeriodTypeSemester
		err := svc.UpdatePeriod(ctx, retyped)
		require.Error(t, err)
		assert.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodOverlapConflict)
	})

	t.Run("re-typing a legacy overlapper out of the conflict stays allowed", func(t *testing.T) {
		// Repo-seed a pre-rule same-type overlap with the base semester, then
		// resolve it by switching to a type with no active overlaps — the
		// guard checks the NEW type, so this repair path must keep working.
		repo := repositories.NewFactory(db).CalendarPeriod
		legacy := makePeriod("SameType-Reparatur", scheduleModels.PeriodTypeSemester,
			timezone.NewDate(2035, 12, 1), timezone.NewDate(2036, 1, 20), true)
		legacy.SetTenantID(tenantID)
		require.NoError(t, repo.Create(ctx, legacy))

		legacy.PeriodType = scheduleModels.PeriodTypeSchoolYear
		require.NoError(t, svc.UpdatePeriod(ctx, legacy),
			"re-typing out of a same-type conflict must resolve, not trip, the overlap guard")
	})
}

// TestCalendarPeriodService_ConcurrentCreateSameTypeOverlap guards the
// serialization of CreatePeriod: two concurrent creates of overlapping active
// same-type periods must yield exactly one row. Without the tenant recurrence
// gate both requests could pass the overlap check before either insert
// commits, silently violating the hard invariant.
func TestCalendarPeriodService_ConcurrentCreateSameTypeOverlap(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	tenantID, ctx := newBootstrapTenant(t, db)

	suffix := time.Now().UnixNano()
	makePeriod := func(name string) *scheduleModels.CalendarPeriod {
		return &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("%s-%d", name, suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2040, 8, 1),
			EndDate:         timezone.NewDate(2041, 1, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
	}

	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results <- svc.CreatePeriod(ctx, makePeriod(fmt.Sprintf("Concurrent-Create-%d", index)))
		}(i)
	}
	wg.Wait()
	close(results)

	conflicts := 0
	for err := range results {
		if err != nil {
			require.ErrorIs(t, err, scheduleModels.ErrCalendarPeriodOverlapConflict)
			conflicts++
		}
	}
	assert.Equal(t, 1, conflicts, "exactly one create must lose the overlap race")

	var rowCount int
	require.NoError(t, db.NewSelect().
		TableExpr("schedule.calendar_periods").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", tenantID).
		Scan(ctx, &rowCount))
	assert.Equal(t, 1, rowCount, "the losing create must not leave a second overlapping row")
}

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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		err := svc.CreatePeriod(ctx, first)
		require.NoError(t, err)

		second := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Second-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 1, 31),
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
// EnsureDefaultSchoolYear Tests (WP-B1)
// =============================================================================

// newBootstrapTenant provisions a fresh, unique tenant so the bootstrap tests
// never race with other tests (or parallel packages) that create calendar
// periods in the shared tenant 1. Calendar periods of the tenant are removed
// on cleanup; the school/org rows are idempotent fixtures and stay.
func newBootstrapTenant(t *testing.T, db *bun.DB) (int64, context.Context) {
	t.Helper()
	scope := testpkg.NewTenantScope(t, db)
	tenantID := scope.TenantID
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			Table("schedule.calendar_periods").
			Where("tenant_id = ?", tenantID).
			Exec(context.Background())
	})
	return tenantID, scope.Context()
}

func TestCalendarPeriodService_EnsureDefaultSchoolYear(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)

	t.Run("creates default school year when tenant has none", func(t *testing.T) {
		_, ctx := newBootstrapTenant(t, db)

		periods, created, err := svc.EnsureDefaultSchoolYear(ctx)

		require.NoError(t, err)
		assert.True(t, created)
		require.Len(t, periods, 1)
		p := periods[0]
		// Independent re-derivation of the expected school year — keeps the
		// test honest against the production helper.
		today := timezone.TodayDate()
		startYear := today.Year
		if today.Month < time.August {
			startYear--
		}
		assert.Equal(t, fmt.Sprintf("Schuljahr %d/%d", startYear, startYear+1), p.Name)
		assert.Equal(t, timezone.NewDate(startYear, time.August, 1), p.StartDate)
		assert.Equal(t, timezone.NewDate(startYear+1, time.July, 31), p.EndDate)
		assert.Equal(t, scheduleModels.PeriodTypeSchoolYear, p.PeriodType)
		assert.True(t, p.IsActive)
		assert.Equal(t, 1, p.WeekCycleLength)
	})

	t.Run("is idempotent on repeated calls", func(t *testing.T) {
		_, ctx := newBootstrapTenant(t, db)

		first, created, err := svc.EnsureDefaultSchoolYear(ctx)
		require.NoError(t, err)
		assert.True(t, created)
		require.Len(t, first, 1)

		second, createdAgain, err := svc.EnsureDefaultSchoolYear(ctx)
		require.NoError(t, err)
		assert.False(t, createdAgain)
		require.Len(t, second, 1)
		assert.Equal(t, first[0].ID, second[0].ID)
	})

	t.Run("no-op when any period already exists", func(t *testing.T) {
		_, ctx := newBootstrapTenant(t, db)

		existing := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Vorhandener-Zeitraum-%d", time.Now().UnixNano()),
			PeriodType:      scheduleModels.PeriodTypeCustom,
			StartDate:       timezone.NewDate(2030, time.January, 1),
			EndDate:         timezone.NewDate(2030, time.June, 30),
			WeekCycleLength: 1,
			IsActive:        false,
		}
		require.NoError(t, svc.CreatePeriod(ctx, existing))

		periods, created, err := svc.EnsureDefaultSchoolYear(ctx)

		require.NoError(t, err)
		assert.False(t, created, "must not create a school year next to an existing period")
		require.Len(t, periods, 1)
		assert.Equal(t, existing.ID, periods[0].ID)
	})

	t.Run("tenant isolation", func(t *testing.T) {
		tenantA, ctxA := newBootstrapTenant(t, db)
		tenantB, ctxB := newBootstrapTenant(t, db)
		require.NotEqual(t, tenantA, tenantB)

		periodsA, createdA, err := svc.EnsureDefaultSchoolYear(ctxA)
		require.NoError(t, err)
		assert.True(t, createdA)
		require.Len(t, periodsA, 1)

		// Tenant B starts empty even though A now has the same-named period.
		periodsB, createdB, err := svc.EnsureDefaultSchoolYear(ctxB)
		require.NoError(t, err)
		assert.True(t, createdB, "tenant B must get its own default period")
		require.Len(t, periodsB, 1)
		assert.NotEqual(t, periodsA[0].ID, periodsB[0].ID)

		// A's view is unchanged by B's bootstrap.
		again, created, err := svc.EnsureDefaultSchoolYear(ctxA)
		require.NoError(t, err)
		assert.False(t, created)
		require.Len(t, again, 1)
		assert.Equal(t, periodsA[0].ID, again[0].ID)
	})

	t.Run("concurrent calls yield one row and no error", func(t *testing.T) {
		tenantID, ctx := newBootstrapTenant(t, db)

		type outcome struct {
			periods []*scheduleModels.CalendarPeriod
			created bool
			err     error
		}
		results := make(chan outcome, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				periods, created, err := svc.EnsureDefaultSchoolYear(ctx)
				results <- outcome{periods: periods, created: created, err: err}
			}()
		}
		wg.Wait()
		close(results)

		createdCount := 0
		for res := range results {
			require.NoError(t, res.err)
			assert.Len(t, res.periods, 1)
			if res.created {
				createdCount++
			}
		}
		assert.Equal(t, 1, createdCount, "exactly one caller must win the insert")

		var rowCount int
		require.NoError(t, db.NewSelect().
			TableExpr("schedule.calendar_periods").
			ColumnExpr("COUNT(*)").
			Where("tenant_id = ?", tenantID).
			Scan(ctx, &rowCount))
		assert.Equal(t, 1, rowCount)
	})
}

// Hermetic DB test (no hardcoded IDs) verifying the recurrence-gate
// serialization of EnsureDefaultSchoolYear against CreatePeriod: a bootstrap
// racing an explicit create of an overlapping active same-type period must
// never insert the default school year next to it. Either the bootstrap sees
// the committed period (created=false) or the explicit create loses with the
// overlap conflict — exactly one row survives.
func TestCalendarPeriodService_ConcurrentBootstrapVsCreate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	tenantID, ctx := newBootstrapTenant(t, db)

	// Same bounds as the default school year so the two inserts overlap.
	today := timezone.TodayDate()
	startYear := today.Year
	if today.Month < time.August {
		startYear--
	}
	explicit := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Eigenes-Schuljahr-%d", time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       timezone.NewDate(startYear, time.August, 1),
		EndDate:         timezone.NewDate(startYear+1, time.July, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}

	var wg sync.WaitGroup
	var createErr error
	var bootstrapCreated bool
	var bootstrapErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		createErr = svc.CreatePeriod(ctx, explicit)
	}()
	go func() {
		defer wg.Done()
		_, bootstrapCreated, bootstrapErr = svc.EnsureDefaultSchoolYear(ctx)
	}()
	wg.Wait()

	require.NoError(t, bootstrapErr, "bootstrap must never fail in this race")
	if createErr != nil {
		require.ErrorIs(t, createErr, scheduleModels.ErrCalendarPeriodOverlapConflict)
		assert.True(t, bootstrapCreated, "create can only lose against a bootstrap that inserted first")
	} else {
		assert.False(t, bootstrapCreated, "bootstrap must not insert next to the committed period")
	}

	var rowCount int
	require.NoError(t, db.NewSelect().
		TableExpr("schedule.calendar_periods").
		ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", tenantID).
		Scan(ctx, &rowCount))
	assert.Equal(t, 1, rowCount, "the race must never leave two overlapping active school years")
}

// =============================================================================
// FindActiveOverlaps Tests (WP-B2)
// =============================================================================

func TestCalendarPeriodService_FindActiveOverlaps(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	svc := setupCalendarPeriodService(t, db)
	_, ctx := newBootstrapTenant(t, db)

	suffix := time.Now().UnixNano()
	active := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("Overlaps-Aktiv-%d", suffix),
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       timezone.NewDate(2030, time.August, 1),
		EndDate:         timezone.NewDate(2031, time.July, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	require.NoError(t, svc.CreatePeriod(ctx, active))

	t.Run("finds overlapping active period", func(t *testing.T) {
		candidate := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Overlaps-Kandidat-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2031, time.January, 1),
			EndDate:         timezone.NewDate(2031, time.December, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		require.NoError(t, svc.CreatePeriod(ctx, candidate))

		overlaps, err := svc.FindActiveOverlaps(ctx, candidate)

		require.NoError(t, err)
		require.Len(t, overlaps, 1)
		assert.Equal(t, active.ID, overlaps[0].ID)
	})

	t.Run("inactive candidate short-circuits to nil", func(t *testing.T) {
		candidate := &scheduleModels.CalendarPeriod{
			Name:       "wird nicht gespeichert",
			PeriodType: scheduleModels.PeriodTypeSemester,
			StartDate:  timezone.NewDate(2031, time.January, 1),
			EndDate:    timezone.NewDate(2031, time.December, 31),
			IsActive:   false,
		}

		overlaps, err := svc.FindActiveOverlaps(ctx, candidate)

		require.NoError(t, err)
		assert.Nil(t, overlaps)
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
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
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
