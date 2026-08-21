package schedule_test

import (
	"testing"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// ClosingDayRepository Tests (#1418 3b)
// =============================================================================

func createTestClosingDay(t *testing.T, repo scheduleModels.ClosingDayRepository, start, end timezone.Date, reason string) *scheduleModels.ClosingDay {
	t.Helper()
	ctx := testpkg.Ctx(t)

	day := &scheduleModels.ClosingDay{
		StartDate: start,
		EndDate:   end,
		Reason:    reason,
	}
	day.SetTenantID(testpkg.Tenant(t))

	err := repo.Create(ctx, day)
	require.NoError(t, err)
	require.Greater(t, day.ID, int64(0))
	return day
}

func TestClosingDayRepository_CRUD(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewClosingDayRepository(db)
	ctx := testpkg.Ctx(t)

	t.Run("creates and reads a range", func(t *testing.T) {
		day := createTestClosingDay(t, repo,
			timezone.NewDate(2026, 12, 24), timezone.NewDate(2026, 12, 31), "Weihnachtswoche")
		defer testpkg.CleanupTableRecords(t, db, "schedule.closing_days", day.ID)

		found, err := repo.FindByID(ctx, day.ID)
		require.NoError(t, err)
		assert.Equal(t, timezone.NewDate(2026, 12, 24), found.StartDate)
		assert.Equal(t, timezone.NewDate(2026, 12, 31), found.EndDate)
		assert.Equal(t, "Weihnachtswoche", found.Reason)
	})

	t.Run("creates a single-day closing (start = end)", func(t *testing.T) {
		day := createTestClosingDay(t, repo,
			timezone.NewDate(2027, 2, 8), timezone.NewDate(2027, 2, 8), "Rosenmontag")
		defer testpkg.CleanupTableRecords(t, db, "schedule.closing_days", day.ID)

		found, err := repo.FindByID(ctx, day.ID)
		require.NoError(t, err)
		assert.Equal(t, found.StartDate, found.EndDate)
	})

	t.Run("fails validation on missing reason", func(t *testing.T) {
		day := &scheduleModels.ClosingDay{
			StartDate: timezone.NewDate(2026, 12, 24),
			EndDate:   timezone.NewDate(2026, 12, 31),
		}
		day.SetTenantID(testpkg.Tenant(t))

		err := repo.Create(ctx, day)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reason is required")
	})

	t.Run("updates and deletes", func(t *testing.T) {
		day := createTestClosingDay(t, repo,
			timezone.NewDate(2026, 10, 12), timezone.NewDate(2026, 10, 16), "Herbstschließung")
		defer testpkg.CleanupTableRecords(t, db, "schedule.closing_days", day.ID)

		day.EndDate = timezone.NewDate(2026, 10, 23)
		day.Reason = "Herbstschließung verlängert"
		require.NoError(t, repo.Update(ctx, day))

		found, err := repo.FindByID(ctx, day.ID)
		require.NoError(t, err)
		assert.Equal(t, timezone.NewDate(2026, 10, 23), found.EndDate)
		assert.Equal(t, "Herbstschließung verlängert", found.Reason)

		require.NoError(t, repo.Delete(ctx, day.ID))
		days, err := repo.FindByTenantID(ctx)
		require.NoError(t, err)
		for _, d := range days {
			assert.NotEqual(t, day.ID, d.ID)
		}
	})
}

func TestClosingDayRepository_FindOverlappingRange(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := scheduleRepo.NewClosingDayRepository(db)
	ctx := testpkg.Ctx(t)

	day := createTestClosingDay(t, repo,
		timezone.NewDate(2026, 7, 20), timezone.NewDate(2026, 8, 7), "Sommerschließung")

	contains := func(days []*scheduleModels.ClosingDay) bool {
		for _, d := range days {
			if d.ID == day.ID {
				return true
			}
		}
		return false
	}

	t.Run("window fully inside the range", func(t *testing.T) {
		days, err := repo.FindOverlappingRange(ctx, timezone.NewDate(2026, 7, 27), timezone.NewDate(2026, 7, 31))
		require.NoError(t, err)
		assert.True(t, contains(days))
	})

	t.Run("range overlaps the window edge", func(t *testing.T) {
		days, err := repo.FindOverlappingRange(ctx, timezone.NewDate(2026, 8, 7), timezone.NewDate(2026, 8, 31))
		require.NoError(t, err)
		assert.True(t, contains(days), "inclusive end_date must match the window start")
	})

	t.Run("no hit outside the range", func(t *testing.T) {
		days, err := repo.FindOverlappingRange(ctx, timezone.NewDate(2026, 8, 8), timezone.NewDate(2026, 8, 31))
		require.NoError(t, err)
		assert.False(t, contains(days))
	})
}

func TestClosingDayRepository_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)

	repo := scheduleRepo.NewClosingDayRepository(db)

	day := createTestClosingDay(t, repo,
		timezone.NewDate(2026, 11, 2), timezone.NewDate(2026, 11, 6), "Pädagogische Woche")

	ctxT2 := testpkg.TenantContext(otherTenantID)
	days, err := repo.FindByTenantID(ctxT2)
	require.NoError(t, err)
	for _, d := range days {
		assert.NotEqual(t, day.ID, d.ID, "the other tenant must not see this tenant's closing days")
	}

	overlapping, err := repo.FindOverlappingRange(ctxT2, timezone.NewDate(2026, 11, 1), timezone.NewDate(2026, 11, 30))
	require.NoError(t, err)
	for _, d := range overlapping {
		assert.NotEqual(t, day.ID, d.ID)
	}
}
