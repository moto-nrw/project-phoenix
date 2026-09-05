package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupSupervisorRepositoryListUsesInjectedCalendarDateAtBoundary(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	repo := activeRepo.NewGroupSupervisorRepository(db, func() time.Time { return now })
	ctx := testpkg.Ctx(t)
	data := createSupervisorTestData(t, db)
	today := timezone.NewDate(2026, 8, 30)
	tomorrow := timezone.NewDate(2026, 8, 31)
	endedToday := &active.GroupSupervisor{GroupID: data.ActiveGroup.ID, StaffID: data.Staff1.ID, StartDate: today, EndDate: &today, Role: "today"}
	endsTomorrow := &active.GroupSupervisor{GroupID: data.ActiveGroup.ID, StaffID: data.Staff2.ID, StartDate: today, EndDate: &tomorrow, Role: "tomorrow"}
	require.NoError(t, repo.Create(ctx, endedToday))
	require.NoError(t, repo.Create(ctx, endsTomorrow))

	options := modelBase.NewQueryOptions()
	options.Filter.Equal("active_only", true).In("id", endedToday.ID, endsTomorrow.ID)
	rows, err := repo.List(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, endsTomorrow.ID, rows[0].ID)
}

func TestGroupSupervisorRepositoryListPreservesActivePseudoFilterComposition(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).GroupSupervisor
	ctx := testpkg.Ctx(t)
	data := createSupervisorTestData(t, db)
	start := timezone.NewDate(2020, 1, 1)
	endedOn := timezone.NewDate(2020, 1, 2)
	open := &active.GroupSupervisor{GroupID: data.ActiveGroup.ID, StaffID: data.Staff1.ID, StartDate: start, Role: "open"}
	ended := &active.GroupSupervisor{GroupID: data.ActiveGroup.ID, StaffID: data.Staff2.ID, StartDate: start, EndDate: &endedOn, Role: "ended"}
	require.NoError(t, repo.Create(ctx, open))
	require.NoError(t, repo.Create(ctx, ended))

	activeOptions := modelBase.NewQueryOptions()
	activeOptions.Filter.Equal("active_only", true).Equal("staff_id", data.Staff1.ID)
	rows, err := repo.List(ctx, activeOptions)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, open.ID, rows[0].ID)

	inactiveOptions := modelBase.NewQueryOptions()
	inactiveOptions.Filter.Equal("active_only", false).Equal("staff_id", data.Staff2.ID)
	rows, err = repo.List(ctx, inactiveOptions)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, ended.ID, rows[0].ID)

	emptyOptions := modelBase.NewQueryOptions()
	emptyOptions.Filter.Equal("active_only", false).Equal("staff_id", data.Staff1.ID)
	empty, err := repo.List(ctx, emptyOptions)
	require.NoError(t, err)
	assert.Nil(t, empty)
}

func TestCombinedGroupRepositoryListPreservesActivePseudoFilterComposition(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CombinedGroup
	ctx := testpkg.Ctx(t)
	past := time.Date(2020, time.January, 2, 12, 0, 0, 0, time.UTC)
	future := time.Date(2099, time.January, 2, 12, 0, 0, 0, time.UTC)
	futureEnded := &active.CombinedGroup{StartTime: past, EndTime: &future}
	pastEnded := &active.CombinedGroup{StartTime: past.Add(-time.Hour), EndTime: &past}
	require.NoError(t, repo.Create(ctx, futureEnded))
	require.NoError(t, repo.Create(ctx, pastEnded))

	activeOptions := modelBase.NewQueryOptions()
	activeOptions.Filter.Equal("active_only", true).Equal("id", futureEnded.ID)
	rows, err := repo.List(ctx, activeOptions)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, futureEnded.ID, rows[0].ID)

	inactiveOptions := modelBase.NewQueryOptions()
	inactiveOptions.Filter.Equal("active_only", false).Equal("id", pastEnded.ID)
	rows, err = repo.List(ctx, inactiveOptions)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, pastEnded.ID, rows[0].ID)

}

func TestCombinedGroupRepositoryListPreservesEmptyNilSlice(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CombinedGroup
	ctx := testpkg.Ctx(t)
	future := time.Date(2099, time.January, 2, 12, 0, 0, 0, time.UTC)
	group := &active.CombinedGroup{StartTime: future.Add(-time.Hour), EndTime: &future}
	require.NoError(t, repo.Create(ctx, group))
	options := modelBase.NewQueryOptions()
	options.Filter.Equal("active_only", false).Equal("id", group.ID)
	empty, err := repo.List(ctx, options)
	require.NoError(t, err)
	assert.Nil(t, empty)
}
