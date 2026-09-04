package schedule_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateframeRepositoryListPreservesQueryOptionsBehavior(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).Dateframe
	ctx := testpkg.Ctx(t)

	alpha := &schedule.Dateframe{
		Name:      "List behavior alpha",
		StartDate: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
	}
	beta := &schedule.Dateframe{
		Name:      "List behavior beta",
		StartDate: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.Create(ctx, alpha))
	require.NoError(t, repo.Create(ctx, beta))

	options := modelBase.NewQueryOptions().WithPagination(1, 1)
	options.Filter.ILike("name", "List behavior %")
	options.Sorting = (&modelBase.Sorting{}).AddField("name", modelBase.SortDesc)

	rows, err := repo.List(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, beta.ID, rows[0].ID)

	emptyOptions := modelBase.NewQueryOptions()
	emptyOptions.Filter.Equal("name", "missing dateframe")
	empty, err := repo.List(ctx, emptyOptions)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}

func TestRecurrenceRuleRepositoryListPreservesQueryOptionsBehavior(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := recurrenceRuleRepository(t, db)
	ctx := testpkg.Ctx(t)

	daily := &schedule.RecurrenceRule{Frequency: schedule.FrequencyDaily, IntervalCount: 1}
	weekly := &schedule.RecurrenceRule{Frequency: schedule.FrequencyWeekly, IntervalCount: 2}
	require.NoError(t, repo.Create(ctx, daily))
	require.NoError(t, repo.Create(ctx, weekly))

	options := modelBase.NewQueryOptions().WithPagination(1, 1)
	options.Filter.In("frequency", schedule.FrequencyDaily, schedule.FrequencyWeekly)
	options.Sorting = (&modelBase.Sorting{}).AddField("interval_count", modelBase.SortDesc)

	rows, err := repo.List(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, weekly.ID, rows[0].ID)

}

func TestRecurrenceRuleRepositoryListPreservesEmptySlice(t *testing.T) {
	t.Parallel()

	repo := recurrenceRuleRepository(t, testpkg.SetupTestDB(t))
	options := modelBase.NewQueryOptions()
	options.Filter.Equal("frequency", schedule.FrequencyMonthly)
	empty, err := repo.List(testpkg.Ctx(t), options)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}
