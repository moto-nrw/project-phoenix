package schedule_test

import (
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
