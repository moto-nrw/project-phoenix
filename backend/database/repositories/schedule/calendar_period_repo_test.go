package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// =============================================================================
// CalendarPeriodRepository Tests
// =============================================================================

func createTestCalendarPeriod(t *testing.T, repo scheduleModels.CalendarPeriodRepository, name string) *scheduleModels.CalendarPeriod {
	t.Helper()
	ctx := testpkg.TenantContext(1)

	period := &scheduleModels.CalendarPeriod{
		Name:            name,
		PeriodType:      scheduleModels.PeriodTypeSchoolYear,
		StartDate:       timezone.NewDate(2025, 8, 1),
		EndDate:         timezone.NewDate(2026, 7, 31),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(1)

	err := repo.Create(ctx, period)
	require.NoError(t, err)
	require.Greater(t, period.ID, int64(0))
	return period
}

func TestCalendarPeriodRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("creates period successfully", func(t *testing.T) {
		name := fmt.Sprintf("Test-Create-%d", time.Now().UnixNano())
		period := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		period.SetTenantID(1)

		err := repo.Create(ctx, period)

		require.NoError(t, err)
		assert.Greater(t, period.ID, int64(0))
		assert.False(t, period.CreatedAt.IsZero())
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
	})

	t.Run("creates period with week cycle anchor", func(t *testing.T) {
		name := fmt.Sprintf("Test-Anchor-%d", time.Now().UnixNano())
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
		period.SetTenantID(1)

		err := repo.Create(ctx, period)

		require.NoError(t, err)
		assert.Greater(t, period.ID, int64(0))
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
	})

	t.Run("fails on nil period", func(t *testing.T) {
		err := repo.Create(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on missing name", func(t *testing.T) {
		period := &scheduleModels.CalendarPeriod{
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
		}
		period.SetTenantID(1)

		err := repo.Create(ctx, period)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("fails validation on invalid period type", func(t *testing.T) {
		period := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Test-InvalidType-%d", time.Now().UnixNano()),
			PeriodType:      "invalid_type",
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 1,
		}
		period.SetTenantID(1)

		err := repo.Create(ctx, period)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid period type")
	})

	t.Run("fails validation on end date before start date", func(t *testing.T) {
		period := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Test-BadDates-%d", time.Now().UnixNano()),
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2026, 8, 1),
			EndDate:         timezone.NewDate(2025, 7, 31),
			WeekCycleLength: 1,
		}
		period.SetTenantID(1)

		err := repo.Create(ctx, period)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "end_date must be after start_date")
	})

	t.Run("fails validation on week cycle without anchor", func(t *testing.T) {
		period := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Test-NoAnchor-%d", time.Now().UnixNano()),
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 7, 31),
			WeekCycleLength: 2,
		}
		period.SetTenantID(1)

		err := repo.Create(ctx, period)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "week_cycle_anchor is required")
	})
}

func TestCalendarPeriodRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("finds period by ID", func(t *testing.T) {
		name := fmt.Sprintf("Test-FindByID-%d", time.Now().UnixNano())
		created := createTestCalendarPeriod(t, repo, name)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", created.ID)

		found, err := repo.FindByID(ctx, created.ID)

		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, name, found.Name)
		assert.Equal(t, scheduleModels.PeriodTypeSchoolYear, found.PeriodType)
		assert.True(t, found.IsActive)
	})

	t.Run("returns error for non-existent ID", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999999))

		require.Error(t, err)
	})
}

func TestCalendarPeriodRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("updates period successfully", func(t *testing.T) {
		name := fmt.Sprintf("Test-Update-%d", time.Now().UnixNano())
		period := createTestCalendarPeriod(t, repo, name)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)

		period.Name = fmt.Sprintf("Updated-%d", time.Now().UnixNano())
		period.PeriodType = scheduleModels.PeriodTypeSemester
		period.IsActive = false

		err := repo.Update(ctx, period)

		require.NoError(t, err)

		updated, err := repo.FindByID(ctx, period.ID)
		require.NoError(t, err)
		assert.Equal(t, period.Name, updated.Name)
		assert.Equal(t, scheduleModels.PeriodTypeSemester, updated.PeriodType)
		assert.False(t, updated.IsActive)
	})

	t.Run("fails on nil period", func(t *testing.T) {
		err := repo.Update(ctx, nil)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation on empty name", func(t *testing.T) {
		name := fmt.Sprintf("Test-UpdateBad-%d", time.Now().UnixNano())
		period := createTestCalendarPeriod(t, repo, name)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)

		period.Name = ""

		err := repo.Update(ctx, period)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
}

func TestCalendarPeriodRepository_FindByTenantID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns all periods for tenant", func(t *testing.T) {
		suffix := time.Now().UnixNano()
		p1 := createTestCalendarPeriod(t, repo, fmt.Sprintf("Test-Tenant-A-%d", suffix))
		p2Name := fmt.Sprintf("Test-Tenant-B-%d", suffix)
		p2 := &scheduleModels.CalendarPeriod{
			Name:            p2Name,
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 1, 31),
			WeekCycleLength: 1,
			IsActive:        false,
		}
		p2.SetTenantID(1)
		err := repo.Create(ctx, p2)
		require.NoError(t, err)

		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", p1.ID, p2.ID)

		periods, err := repo.FindByTenantID(ctx)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(periods), 2)

		foundP1, foundP2 := false, false
		for _, p := range periods {
			if p.ID == p1.ID {
				foundP1 = true
			}
			if p.ID == p2.ID {
				foundP2 = true
			}
		}
		assert.True(t, foundP1, "should find first period")
		assert.True(t, foundP2, "should find second period")
	})
}

func TestCalendarPeriodRepository_FindActiveByTenantID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("returns only active periods", func(t *testing.T) {
		suffix := time.Now().UnixNano()

		activePeriod := createTestCalendarPeriod(t, repo, fmt.Sprintf("Test-Active-%d", suffix))

		inactivePeriod := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("Test-Inactive-%d", suffix),
			PeriodType:      scheduleModels.PeriodTypeSemester,
			StartDate:       timezone.NewDate(2025, 8, 1),
			EndDate:         timezone.NewDate(2026, 1, 31),
			WeekCycleLength: 1,
			IsActive:        false,
		}
		inactivePeriod.SetTenantID(1)
		err := repo.Create(ctx, inactivePeriod)
		require.NoError(t, err)

		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", activePeriod.ID, inactivePeriod.ID)

		periods, err := repo.FindActiveByTenantID(ctx)

		require.NoError(t, err)

		foundActive, foundInactive := false, false
		for _, p := range periods {
			if p.ID == activePeriod.ID {
				foundActive = true
			}
			if p.ID == inactivePeriod.ID {
				foundInactive = true
			}
		}
		assert.True(t, foundActive, "should find active period")
		assert.False(t, foundInactive, "should not find inactive period")
	})
}

func TestCalendarPeriodRepository_FindByName(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("finds period by name", func(t *testing.T) {
		name := fmt.Sprintf("Test-FindByName-%d", time.Now().UnixNano())
		created := createTestCalendarPeriod(t, repo, name)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", created.ID)

		found, err := repo.FindByName(ctx, name)

		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, name, found.Name)
	})

	t.Run("returns nil for non-existent name", func(t *testing.T) {
		found, err := repo.FindByName(ctx, "non-existent-period-name-xyz")

		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

// newOverlapTenant provisions a fresh tenant so date-range assertions can be
// exact: the shared tenant 1 accumulates periods from other tests (and other
// packages running in parallel against the same test DB) whose ranges would
// pollute overlap results.
func newOverlapTenant(t *testing.T, db *bun.DB, base int64) (int64, context.Context) {
	t.Helper()
	tenantID := base + time.Now().UnixNano()%50000
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		_, _ = db.NewDelete().
			Table("schedule.calendar_periods").
			Where("tenant_id = ?", tenantID).
			Exec(context.Background())
	})
	return tenantID, testpkg.TenantContext(tenantID)
}

func createPeriodForTenant(t *testing.T, repo scheduleModels.CalendarPeriodRepository, ctx context.Context, tenantID int64, name string, start, end timezone.Date, isActive bool) *scheduleModels.CalendarPeriod {
	t.Helper()
	period := &scheduleModels.CalendarPeriod{
		Name:            fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
		PeriodType:      scheduleModels.PeriodTypeCustom,
		StartDate:       start,
		EndDate:         end,
		WeekCycleLength: 1,
		IsActive:        isActive,
	}
	period.SetTenantID(tenantID)
	require.NoError(t, repo.Create(ctx, period))
	return period
}

func TestCalendarPeriodRepository_CreateIfAbsent(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)

	t.Run("inserts when name is absent, no-ops on conflict", func(t *testing.T) {
		tenantID, ctx := newOverlapTenant(t, db, 510000)
		name := fmt.Sprintf("Bootstrap-%d", time.Now().UnixNano())

		first := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2030, time.August, 1),
			EndDate:         timezone.NewDate(2031, time.July, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		first.SetTenantID(tenantID)
		created, err := repo.CreateIfAbsent(ctx, first)
		require.NoError(t, err)
		assert.True(t, created)
		assert.Greater(t, first.ID, int64(0))

		second := &scheduleModels.CalendarPeriod{
			Name:            name,
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2030, time.August, 1),
			EndDate:         timezone.NewDate(2031, time.July, 31),
			WeekCycleLength: 1,
			IsActive:        true,
		}
		second.SetTenantID(tenantID)
		createdAgain, err := repo.CreateIfAbsent(ctx, second)
		require.NoError(t, err, "conflict must be a clean no-op, not an error")
		assert.False(t, createdAgain)

		var rowCount int
		require.NoError(t, db.NewSelect().
			TableExpr("schedule.calendar_periods").
			ColumnExpr("COUNT(*)").
			Where("tenant_id = ?", tenantID).
			Where("name = ?", name).
			Scan(ctx, &rowCount))
		assert.Equal(t, 1, rowCount)
	})

	t.Run("same name in another tenant still inserts", func(t *testing.T) {
		tenantA, ctxA := newOverlapTenant(t, db, 520000)
		tenantB, ctxB := newOverlapTenant(t, db, 530000)
		name := fmt.Sprintf("Bootstrap-Cross-%d", time.Now().UnixNano())

		makePeriod := func(tenantID int64) *scheduleModels.CalendarPeriod {
			p := &scheduleModels.CalendarPeriod{
				Name:            name,
				PeriodType:      scheduleModels.PeriodTypeSchoolYear,
				StartDate:       timezone.NewDate(2030, time.August, 1),
				EndDate:         timezone.NewDate(2031, time.July, 31),
				WeekCycleLength: 1,
				IsActive:        true,
			}
			p.SetTenantID(tenantID)
			return p
		}

		createdA, err := repo.CreateIfAbsent(ctxA, makePeriod(tenantA))
		require.NoError(t, err)
		assert.True(t, createdA)

		createdB, err := repo.CreateIfAbsent(ctxB, makePeriod(tenantB))
		require.NoError(t, err)
		assert.True(t, createdB, "uniqueness is per tenant, not global")
	})

	t.Run("fails on nil period", func(t *testing.T) {
		_, ctx := newOverlapTenant(t, db, 540000)
		created, err := repo.CreateIfAbsent(ctx, nil)
		require.Error(t, err)
		assert.False(t, created)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("fails validation before touching the database", func(t *testing.T) {
		tenantID, ctx := newOverlapTenant(t, db, 550000)
		invalid := &scheduleModels.CalendarPeriod{
			PeriodType:      scheduleModels.PeriodTypeSchoolYear,
			StartDate:       timezone.NewDate(2030, time.August, 1),
			EndDate:         timezone.NewDate(2031, time.July, 31),
			WeekCycleLength: 1,
		}
		invalid.SetTenantID(tenantID)
		created, err := repo.CreateIfAbsent(ctx, invalid)
		require.Error(t, err)
		assert.False(t, created)
		assert.Contains(t, err.Error(), "name is required")
	})
}

func TestCalendarPeriodRepository_FindActiveOverlapping(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	tenantID, ctx := newOverlapTenant(t, db, 560000)

	// Fixture layout (all in the fresh tenant):
	//   active     2030-08-01 .. 2031-07-31  (the period others may collide with)
	//   inactive   2030-08-01 .. 2031-07-31  (same range, is_active = false)
	//   adjacent   2031-08-01 .. 2032-07-31  (starts the day after active ends)
	active := createPeriodForTenant(t, repo, ctx, tenantID, "Overlap-Aktiv",
		timezone.NewDate(2030, time.August, 1), timezone.NewDate(2031, time.July, 31), true)
	inactive := createPeriodForTenant(t, repo, ctx, tenantID, "Overlap-Inaktiv",
		timezone.NewDate(2030, time.August, 1), timezone.NewDate(2031, time.July, 31), false)
	adjacent := createPeriodForTenant(t, repo, ctx, tenantID, "Overlap-Angrenzend",
		timezone.NewDate(2031, time.August, 1), timezone.NewDate(2032, time.July, 31), true)

	idsOf := func(periods []*scheduleModels.CalendarPeriod) []int64 {
		ids := make([]int64, 0, len(periods))
		for _, p := range periods {
			ids = append(ids, p.ID)
		}
		return ids
	}

	t.Run("finds overlapping active period", func(t *testing.T) {
		got, err := repo.FindActiveOverlapping(ctx,
			timezone.NewDate(2030, time.September, 1), timezone.NewDate(2030, time.October, 1), 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{active.ID}, idsOf(got))
	})

	t.Run("ignores inactive periods", func(t *testing.T) {
		got, err := repo.FindActiveOverlapping(ctx,
			timezone.NewDate(2030, time.September, 1), timezone.NewDate(2030, time.October, 1), 0)
		require.NoError(t, err)
		assert.NotContains(t, idsOf(got), inactive.ID)
	})

	t.Run("excludes the period itself", func(t *testing.T) {
		got, err := repo.FindActiveOverlapping(ctx,
			active.StartDate, active.EndDate, active.ID)
		require.NoError(t, err)
		assert.NotContains(t, idsOf(got), active.ID)
	})

	t.Run("inclusive boundary day counts as overlap", func(t *testing.T) {
		// Range starting exactly on active's last day must still collide.
		got, err := repo.FindActiveOverlapping(ctx,
			timezone.NewDate(2031, time.July, 31), timezone.NewDate(2031, time.July, 31), adjacent.ID)
		require.NoError(t, err)
		assert.Equal(t, []int64{active.ID}, idsOf(got))
	})

	t.Run("adjacent ranges do not overlap", func(t *testing.T) {
		// adjacent's own range starts 2031-08-01, one day after active ends.
		got, err := repo.FindActiveOverlapping(ctx,
			adjacent.StartDate, adjacent.EndDate, adjacent.ID)
		require.NoError(t, err)
		assert.NotContains(t, idsOf(got), active.ID)
		assert.Empty(t, got)
	})

	t.Run("disjoint range finds nothing", func(t *testing.T) {
		got, err := repo.FindActiveOverlapping(ctx,
			timezone.NewDate(2040, time.January, 1), timezone.NewDate(2040, time.December, 31), 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("cross-tenant isolation", func(t *testing.T) {
		otherTenantID, otherCtx := newOverlapTenant(t, db, 570000)
		other := createPeriodForTenant(t, repo, otherCtx, otherTenantID, "Overlap-Fremd",
			timezone.NewDate(2030, time.August, 1), timezone.NewDate(2031, time.July, 31), true)

		// Query in the first tenant: the other tenant's period is invisible.
		got, err := repo.FindActiveOverlapping(ctx,
			timezone.NewDate(2030, time.September, 1), timezone.NewDate(2030, time.October, 1), 0)
		require.NoError(t, err)
		assert.NotContains(t, idsOf(got), other.ID)
		assert.Equal(t, []int64{active.ID}, idsOf(got))

		// And vice versa: the other tenant only sees its own period.
		gotOther, err := repo.FindActiveOverlapping(otherCtx,
			timezone.NewDate(2030, time.September, 1), timezone.NewDate(2030, time.October, 1), 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{other.ID}, idsOf(gotOther))
	})
}

func TestCalendarPeriodRepository_FindActiveOverlappingByType(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	tenantID, ctx := newOverlapTenant(t, db, 580000)

	createTyped := func(cctx context.Context, tid int64, name, periodType string, isActive bool) *scheduleModels.CalendarPeriod {
		period := &scheduleModels.CalendarPeriod{
			Name:            fmt.Sprintf("%s-%d", name, time.Now().UnixNano()),
			PeriodType:      periodType,
			StartDate:       timezone.NewDate(2033, time.August, 1),
			EndDate:         timezone.NewDate(2034, time.July, 31),
			WeekCycleLength: 1,
			IsActive:        isActive,
		}
		period.SetTenantID(tid)
		require.NoError(t, repo.Create(cctx, period))
		return period
	}

	queryStart := timezone.NewDate(2033, time.September, 1)
	queryEnd := timezone.NewDate(2033, time.October, 1)
	activeSemester := createTyped(ctx, tenantID, "ByType-Halbjahr", scheduleModels.PeriodTypeSemester, true)
	holiday := createTyped(ctx, tenantID, "ByType-Ferien", scheduleModels.PeriodTypeHoliday, true)
	inactiveSemester := createTyped(ctx, tenantID, "ByType-Halbjahr-Inaktiv", scheduleModels.PeriodTypeSemester, false)

	idsOf := func(periods []*scheduleModels.CalendarPeriod) []int64 {
		ids := make([]int64, 0, len(periods))
		for _, p := range periods {
			ids = append(ids, p.ID)
		}
		return ids
	}

	t.Run("finds only active periods of the same type", func(t *testing.T) {
		got, err := repo.FindActiveOverlappingByType(ctx, scheduleModels.PeriodTypeSemester, queryStart, queryEnd, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{activeSemester.ID}, idsOf(got))
		assert.NotContains(t, idsOf(got), holiday.ID)
		assert.NotContains(t, idsOf(got), inactiveSemester.ID)
	})

	t.Run("different type sees only its own overlaps", func(t *testing.T) {
		got, err := repo.FindActiveOverlappingByType(ctx, scheduleModels.PeriodTypeHoliday, queryStart, queryEnd, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{holiday.ID}, idsOf(got))
	})

	t.Run("excludes the period itself", func(t *testing.T) {
		got, err := repo.FindActiveOverlappingByType(ctx, scheduleModels.PeriodTypeSemester, queryStart, queryEnd, activeSemester.ID)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("disjoint range finds nothing", func(t *testing.T) {
		got, err := repo.FindActiveOverlappingByType(ctx, scheduleModels.PeriodTypeSemester,
			timezone.NewDate(2040, time.January, 1), timezone.NewDate(2040, time.December, 31), 0)
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("cross-tenant isolation", func(t *testing.T) {
		otherTenantID, otherCtx := newOverlapTenant(t, db, 590000)
		other := createTyped(otherCtx, otherTenantID, "ByType-Fremd", scheduleModels.PeriodTypeSemester, true)

		got, err := repo.FindActiveOverlappingByType(ctx, scheduleModels.PeriodTypeSemester, queryStart, queryEnd, 0)
		require.NoError(t, err)
		assert.NotContains(t, idsOf(got), other.ID)

		gotOther, err := repo.FindActiveOverlappingByType(otherCtx, scheduleModels.PeriodTypeSemester, queryStart, queryEnd, 0)
		require.NoError(t, err)
		assert.Equal(t, []int64{other.ID}, idsOf(gotOther))
	})
}

func TestCalendarPeriodRepository_UsageCounts_ReturnsDatabaseError(t *testing.T) {
	db := testpkg.SetupClosableTestDB(t)
	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)
	require.NoError(t, db.Close())

	usage, err := repo.UsageCounts(ctx)

	require.Error(t, err)
	assert.Nil(t, usage)
	assert.Contains(t, err.Error(), "usage counts")
}

func TestCalendarPeriodRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("deletes period successfully", func(t *testing.T) {
		name := fmt.Sprintf("Test-Delete-%d", time.Now().UnixNano())
		period := createTestCalendarPeriod(t, repo, name)

		err := repo.Delete(ctx, period.ID)

		require.NoError(t, err)

		_, findErr := repo.FindByID(ctx, period.ID)
		assert.Error(t, findErr)
	})
}

// TestCalendarPeriodFKOnDelete verifies that the three FK columns pointing to
// schedule.calendar_periods are declared ON DELETE SET NULL, so deleting a
// period clears references instead of failing with a constraint violation.
// Catalog code 'n' = SET NULL, 'a' = NO ACTION (default RESTRICT-equivalent),
// 'c' = CASCADE, 'r' = RESTRICT, 'd' = SET DEFAULT.
func TestCalendarPeriodFKOnDelete(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.TenantContext(1)

	tables := []string{
		"activities.schedules",
		"activities.student_enrollments",
		"activities.supervisors",
	}

	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			var confdeltype string
			err := db.NewRaw(`
				SELECT c.confdeltype
				FROM pg_constraint c
				JOIN pg_class t ON t.oid = c.conrelid
				JOIN pg_namespace ns ON ns.oid = t.relnamespace
				JOIN pg_class rt ON rt.oid = c.confrelid
				JOIN pg_namespace rns ON rns.oid = rt.relnamespace
				WHERE c.contype = 'f'
				  AND ns.nspname || '.' || t.relname = ?
				  AND rns.nspname = 'schedule'
				  AND rt.relname = 'calendar_periods'
				LIMIT 1
			`, table).Scan(ctx, &confdeltype)
			require.NoError(t, err, "FK to schedule.calendar_periods must exist on %s", table)
			assert.Equal(t, "n", confdeltype,
				"%s.calendar_period_id must be ON DELETE SET NULL (got %q)", table, confdeltype)
		})
	}
}

func TestCalendarPeriodRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewCalendarPeriodRepository(db)
	ctx := testpkg.TenantContext(1)

	t.Run("lists periods with nil options", func(t *testing.T) {
		name := fmt.Sprintf("Test-List-%d", time.Now().UnixNano())
		period := createTestCalendarPeriod(t, repo, name)
		defer testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)

		periods, err := repo.List(ctx, nil)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(periods), 1)
	})
}
