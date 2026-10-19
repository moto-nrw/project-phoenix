package httpintegration_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClosingDayAdapter_KeepsTheLegacyContracts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := api.NewCareScheduleSuiteRepositories(testpkg.NewDatabase(t, db)).ClosingDay
	ctx := testpkg.Ctx(t)
	ranges := fixturetest.ClosingDayRanges(repo)

	fixture := readBack(t, ctx, repo.FindByID, testpkg.CreateTestClosingDay(t, db, testpkg.Date(2030, time.November, 4), testpkg.Date(2030, time.November, 8), "Pädagogische Woche").ID)
	day := *fixture
	day.ID = 0
	day.StartDate = "2030-12-23"
	day.EndDate = "2030-12-31"
	day.Reason = "Weihnachtswoche"
	require.NoError(t, repo.Create(ctx, &day))
	assert.Positive(t, day.ID)
	assert.NotEqual(t, fixture.ID, day.ID)
	assert.Equal(t, testpkg.Tenant(t), day.GetTenantID())

	found, err := repo.FindByID(ctx, day.ID)
	require.NoError(t, err)
	assert.Equal(t, "2030-12-23", found.StartDate.String())
	assert.Equal(t, "Weihnachtswoche", found.Reason)

	_, err = repo.FindByID(ctx, missingID(day.ID))
	require.Error(t, err)
	assert.True(t, testpkg.IsNotFoundError(err))

	blank := *fixture
	blank.ID = 0
	blank.Reason = ""
	require.EqualError(t, repo.Create(ctx, &blank), "reason is required")
	require.EqualError(t, repo.Create(ctx, nil), "ClosingDay cannot be nil or zero value")

	day.EndDate = "2031-01-03"
	require.NoError(t, repo.Update(ctx, &day))
	overlapping, err := ranges.FindOverlappingRange(ctx, "2031-01-03", "2031-01-31")
	require.NoError(t, err)
	require.Len(t, overlapping, 1, "the inclusive end date matches the window start")
	assert.Equal(t, day.ID, overlapping[0].ID)
	outside, err := ranges.FindOverlappingRange(ctx, "2031-01-04", "2031-01-31")
	require.NoError(t, err)
	assert.Empty(t, outside)

	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	foreign, err := repo.FindByTenantID(testpkg.TenantContext(otherTenantID))
	require.NoError(t, err)
	assert.Empty(t, foreign, "the other tenant must not see this tenant's closing days")

	all, err := repo.FindByTenantID(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, fixture.ID, all[0].ID, "ordered by start date")

	require.NoError(t, repo.Delete(ctx, day.ID))
	require.NoError(t, repo.Delete(ctx, day.ID), "delete stays idempotent")
	days, err := repo.FindByTenantID(ctx)
	require.NoError(t, err)
	require.Len(t, days, 1)
	assert.Equal(t, fixture.ID, days[0].ID)
}

func readBack[M any](t *testing.T, ctx context.Context, find func(context.Context, int64) (M, error), id int64) M {
	t.Helper()
	row, err := find(ctx, id)
	require.NoError(t, err)
	return row
}
func missingID(anchor int64) int64 { return anchor + 1_000_000 }
