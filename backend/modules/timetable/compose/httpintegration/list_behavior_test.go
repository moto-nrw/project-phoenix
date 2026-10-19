package httpintegration_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDateframeRepositoryListPreservesQueryOptionsBehavior(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := fixtures.FromDateframeRecords(api.NewCareScheduleSuiteRepositories(testpkg.NewDatabase(t, db)).Dateframe).(dateframeQueryRepository)
	ctx := testpkg.Ctx(t)

	alpha := fixtures.NewDateframe(func(fixtureRow *fixtures.Dateframe) {
		fixtureRow.Name = "List behavior alpha"
		fixtureRow.StartDate = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
		fixtureRow.EndDate = time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	})

	beta := fixtures.NewDateframe(func(fixtureRow *fixtures.Dateframe) {
		fixtureRow.Name = "List behavior beta"
		fixtureRow.StartDate = time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
		fixtureRow.EndDate = time.Date(2026, time.February, 2, 0, 0, 0, 0, time.UTC)
	})

	require.NoError(t, repo.Create(ctx, alpha))
	require.NoError(t, repo.Create(ctx, beta))

	options := newListOptions().WithPagination(1, 1)
	options.Filter.ILike("name", "List behavior %")
	sortedBy(options, "name", true)

	rows, err := repo.List(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, beta.ID, rows[0].ID)

	emptyOptions := newListOptions()
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

	daily := fixtures.NewRecurrenceRule(func(fixtureRow *fixtures.RecurrenceRule) {
		fixtureRow.Frequency = fixtures.FrequencyDaily()
		fixtureRow.IntervalCount = 1
	})
	weekly := fixtures.NewRecurrenceRule(func(fixtureRow *fixtures.RecurrenceRule) {
		fixtureRow.Frequency = fixtures.FrequencyWeekly()
		fixtureRow.IntervalCount = 2
	})
	require.NoError(t, repo.Create(ctx, daily))
	require.NoError(t, repo.Create(ctx, weekly))

	options := newListOptions().WithPagination(1, 1)
	options.Filter.In("frequency", fixtures.FrequencyDaily(), fixtures.FrequencyWeekly())
	sortedBy(options, "interval_count", true)

	rows, err := repo.List(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, weekly.ID, rows[0].ID)

}

func TestRecurrenceRuleRepositoryListPreservesEmptySlice(t *testing.T) {
	t.Parallel()

	repo := recurrenceRuleRepository(t, testpkg.SetupTestDB(t))
	options := newListOptions()
	options.Filter.Equal("frequency", fixtures.FrequencyMonthly())
	empty, err := repo.List(testpkg.Ctx(t), options)
	require.NoError(t, err)
	assert.NotNil(t, empty)
	assert.Empty(t, empty)
}
