package schedule_test

import (
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func createInstanceFixture(t *testing.T, db *bun.DB, prefix string, date time.Time) (*scheduleModels.ActivityInstance, func()) {
	t.Helper()

	fx := newActivityInstanceFixtures(t, db, prefix)

	repo := scheduleRepo.NewActivityInstanceRepository(db)
	ctx := testpkg.TenantContext(1)

	inst := buildInstance(1, fx.roomID, &fx.activityID, date,
		time.Date(0, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(0, 1, 1, 15, 0, 0, 0, time.UTC),
		fmt.Sprintf("Instance-%s-%d", prefix, time.Now().UnixNano()),
	)
	require.NoError(t, repo.Create(ctx, inst))

	cleanup := func() {
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)
		fx.cleanup()
	}

	return inst, cleanup
}

func TestInstanceStaffRepository_Create_and_FindByInstanceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stf", time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	defer cleanupInst()

	staffA := testpkg.CreateTestStaff(t, db, "Alice", fmt.Sprintf("Primary-%d", time.Now().UnixNano()))
	staffB := testpkg.CreateTestStaff(t, db, "Bob", fmt.Sprintf("Sub-%d", time.Now().UnixNano()))

	rowA := &scheduleModels.InstanceStaff{
		InstanceID: inst.ID,
		StaffID:    staffA.ID,
		IsPrimary:  true,
	}
	rowA.SetTenantID(1)

	rowB := &scheduleModels.InstanceStaff{
		InstanceID:   inst.ID,
		StaffID:      staffB.ID,
		IsSubstitute: true,
	}
	rowB.SetTenantID(1)

	require.NoError(t, repo.Create(ctx, rowA))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", rowA.ID)
	require.NoError(t, repo.Create(ctx, rowB))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", rowB.ID)
	defer testpkg.CleanupActivityFixtures(t, db, staffA.ID, staffB.ID)

	t.Run("FindByInstanceID returns both", func(t *testing.T) {
		rows, err := repo.FindByInstanceID(ctx, inst.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rows), 2)
	})

	t.Run("UNIQUE(instance_id, staff_id) blocks duplicate", func(t *testing.T) {
		dup := &scheduleModels.InstanceStaff{
			InstanceID: inst.ID,
			StaffID:    staffA.ID,
		}
		dup.SetTenantID(1)
		err := repo.Create(ctx, dup)
		require.Error(t, err, "UNIQUE constraint must reject duplicate")
	})

	t.Run("Create rejects nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Create rejects invalid payload", func(t *testing.T) {
		bad := &scheduleModels.InstanceStaff{InstanceID: 0, StaffID: staffA.ID}
		bad.SetTenantID(1)
		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance_id is required")
	})
}

func TestInstanceStaffRepository_FindByStaffAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	date := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	inst, cleanupInst := createInstanceFixture(t, db, "stf-date", date)
	defer cleanupInst()

	staff := testpkg.CreateTestStaff(t, db, "Claire", fmt.Sprintf("Multi-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	row := &scheduleModels.InstanceStaff{
		InstanceID: inst.ID,
		StaffID:    staff.ID,
	}
	row.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", row.ID)

	rows, err := repo.FindByStaffAndDate(ctx, staff.ID, date)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 1)
	assert.Equal(t, row.ID, rows[0].ID)

	otherDate := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	rows, err = repo.FindByStaffAndDate(ctx, staff.ID, otherDate)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestInstanceStaffRepository_DeleteByInstanceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "del", time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC))
	defer cleanupInst()

	staff := testpkg.CreateTestStaff(t, db, "Dan", fmt.Sprintf("Del-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	row := &scheduleModels.InstanceStaff{InstanceID: inst.ID, StaffID: staff.ID}
	row.SetTenantID(1)
	require.NoError(t, repo.Create(ctx, row))

	require.NoError(t, repo.DeleteByInstanceID(ctx, inst.ID))

	rows, err := repo.FindByInstanceID(ctx, inst.ID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
