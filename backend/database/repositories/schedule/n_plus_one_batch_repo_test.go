package schedule_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActivityInstanceRepository_FindByActiveGroupIDs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewActivityInstanceRepository(db)
	date := timezone.NewDate(2026, time.October, 14)

	instances := make([]*scheduleModels.ActivityInstance, 0, 2)
	activeGroupIDs := make([]int64, 0, 2)
	for _, suffix := range []string{"first", "second"} {
		fixture := newActivityInstanceFixtures(t, db, "active-batch-"+suffix)
		activeGroup := testpkg.CreateTestActiveGroup(t, db, fixture.activityID, fixture.roomID)
		instance := buildInstance(testpkg.Tenant(t), fixture.roomID, &fixture.activityID, date,
			time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC), suffix)
		instance.ActiveGroupID = &activeGroup.ID
		require.NoError(t, repo.Create(ctx, instance))
		instances = append(instances, instance)
		activeGroupIDs = append(activeGroupIDs, activeGroup.ID)
	}

	rows, err := repo.FindByActiveGroupIDs(ctx, activeGroupIDs)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.ElementsMatch(t, []int64{instances[0].ID, instances[1].ID}, []int64{rows[0].ID, rows[1].ID})

	rows, err = repo.FindByActiveGroupIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestInstanceStaffRepository_FindByStaffIDsAndDate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)
	day := timezone.NewDate(2026, time.October, 15)
	first, _ := createInstanceFixture(t, db, "staff-batch-first", day)
	otherDay, _ := createInstanceFixture(t, db, "staff-batch-other", day.AddDays(1))
	staffA := testpkg.CreateTestStaff(t, db, "StaffBatch", "First")
	staffB := testpkg.CreateTestStaff(t, db, "StaffBatch", "Second")

	for _, row := range []*scheduleModels.InstanceStaff{
		{InstanceID: first.ID, StaffID: staffA.ID},
		{InstanceID: first.ID, StaffID: staffB.ID},
		{InstanceID: otherDay.ID, StaffID: staffA.ID},
	} {
		require.NoError(t, repo.Create(ctx, row))
	}

	rows, err := repo.FindByStaffIDsAndDate(ctx, []int64{staffA.ID, staffB.ID}, day)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.ElementsMatch(t, []int64{staffA.ID, staffB.ID}, []int64{rows[0].StaffID, rows[1].StaffID})

	rows, err = repo.FindByStaffIDsAndDate(ctx, nil, day)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStaffShiftRepository_FindByOriginShiftIDs(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db).StaffShift
	day := timezone.NewDate(2026, time.October, 16)
	staff := make([]int64, 4)
	for i := range staff {
		member := testpkg.CreateTestStaff(t, db, "ShiftBatch", time.Now().Add(time.Duration(i)*time.Nanosecond).Format("150405.000000000"))
		staff[i] = member.ID
	}

	originA := newShift(staff[0], day, 8, 12, staff[0])
	originB := newShift(staff[1], day, 13, 17, staff[0])
	require.NoError(t, repo.Create(ctx, originA))
	require.NoError(t, repo.Create(ctx, originB))
	coverA := newShift(staff[2], day, 8, 12, staff[0])
	coverA.OriginShiftID = &originA.ID
	coverB := newShift(staff[3], day, 13, 17, staff[0])
	coverB.OriginShiftID = &originB.ID
	require.NoError(t, repo.Create(ctx, coverA))
	require.NoError(t, repo.Create(ctx, coverB))

	rows, err := repo.FindByOriginShiftIDs(ctx, []int64{originA.ID, originB.ID})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.ElementsMatch(t, []int64{coverA.ID, coverB.ID}, []int64{rows[0].ID, rows[1].ID})

	rows, err = repo.FindByOriginShiftIDs(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
