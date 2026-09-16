package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	activeRepo "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/repositories/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresenceSupervisionQueryUsesCalendarDateAtBoundary(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	repo := activeRepo.NewGroupSupervisorRepository(repositories.NewPresenceSupervisionRecords(db), func() time.Time { return now })
	ctx := testpkg.Ctx(t)
	data := createSupervisorTestData(t, db)
	today := timezone.NewDate(2026, 8, 30)
	tomorrow := timezone.NewDate(2026, 8, 31)
	endedToday := &active.GroupSupervisor{GroupID: data.ActiveGroup.ID, StaffID: data.Staff1ID, StartDate: today, EndDate: &today, Role: "today"}
	endsTomorrow := &active.GroupSupervisor{GroupID: data.ActiveGroup.ID, StaffID: data.Staff2ID, StartDate: today, EndDate: &tomorrow, Role: "tomorrow"}
	require.NoError(t, repo.Create(ctx, endedToday))
	require.NoError(t, repo.Create(ctx, endsTomorrow))

	day := today.String()
	options := studentpresence.GroupSupervisionFilter{ActiveOn: &day, IDs: []int64{endedToday.ID, endsTomorrow.ID}}
	rows, err := newPresence(t, db).QueryGroupSupervisions(ctx, options)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, endsTomorrow.ID, rows[0].ID)
}

func TestPresenceSupervisionQueryComposesDateAndStaffFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).GroupSupervisor
	ctx := testpkg.Ctx(t)
	data := createSupervisorTestData(t, db)
	start := timezone.NewDate(2020, 1, 1)
	endedOn := timezone.NewDate(2020, 1, 2)
	open := &active.GroupSupervisor{GroupID: data.ActiveGroup.ID, StaffID: data.Staff1ID, StartDate: start, Role: "open"}
	ended := &active.GroupSupervisor{GroupID: data.ActiveGroup.ID, StaffID: data.Staff2ID, StartDate: start, EndDate: &endedOn, Role: "ended"}
	require.NoError(t, repo.Create(ctx, open))
	require.NoError(t, repo.Create(ctx, ended))

	day := "2026-08-30"
	activeOptions := studentpresence.GroupSupervisionFilter{ActiveOn: &day, StaffIDs: []int64{data.Staff1ID}}
	rows, err := newPresence(t, db).QueryGroupSupervisions(ctx, activeOptions)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, open.ID, rows[0].ID)

	inactiveOptions := studentpresence.GroupSupervisionFilter{EndedBy: &day, StaffIDs: []int64{data.Staff2ID}}
	rows, err = newPresence(t, db).QueryGroupSupervisions(ctx, inactiveOptions)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, ended.ID, rows[0].ID)

	emptyOptions := studentpresence.GroupSupervisionFilter{EndedBy: &day, StaffIDs: []int64{data.Staff1ID}}
	empty, err := newPresence(t, db).QueryGroupSupervisions(ctx, emptyOptions)
	require.NoError(t, err)
	assert.Empty(t, empty)
}
