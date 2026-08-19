package schedule_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func createInstanceFixture(t *testing.T, db *bun.DB, prefix string, date timezone.Date) (*scheduleModels.ActivityInstance, func()) {
	t.Helper()

	fx := newActivityInstanceFixtures(t, db, prefix)

	repo := scheduleRepo.NewActivityInstanceRepository(db)
	ctx := testpkg.Ctx(t)

	inst := buildInstance(testpkg.Tenant(t), fx.roomID, &fx.activityID, date,
		time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
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

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "stf", timezone.NewDate(2026, 9, 15))
	defer cleanupInst()

	staffA := testpkg.CreateTestStaff(t, db, "Alice", fmt.Sprintf("Primary-%d", time.Now().UnixNano()))
	staffB := testpkg.CreateTestStaff(t, db, "Bob", fmt.Sprintf("Sub-%d", time.Now().UnixNano()))

	rowA := &scheduleModels.InstanceStaff{
		InstanceID: inst.ID,
		StaffID:    staffA.ID,
		IsPrimary:  true,
	}
	rowA.SetTenantID(testpkg.Tenant(t))

	rowB := &scheduleModels.InstanceStaff{
		InstanceID:   inst.ID,
		StaffID:      staffB.ID,
		IsSubstitute: true,
	}
	rowB.SetTenantID(testpkg.Tenant(t))

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

	t.Run("single and batch reads share created-at substitute order", func(t *testing.T) {
		baseTime := time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
		_, err := db.NewUpdate().Table("schedule.instance_staff").
			Set("created_at = ?", baseTime.Add(time.Hour)).Where("id = ?", rowA.ID).Exec(ctx)
		require.NoError(t, err)
		_, err = db.NewUpdate().Table("schedule.instance_staff").
			Set("created_at = ?", baseTime).Where("id = ?", rowB.ID).Exec(ctx)
		require.NoError(t, err)

		single, err := repo.FindByInstanceID(ctx, inst.ID)
		require.NoError(t, err)
		batch, err := repo.FindByInstanceIDs(ctx, []int64{inst.ID})
		require.NoError(t, err)
		require.Len(t, single, 2)
		require.Len(t, batch, 2)
		assert.Equal(t, []int64{rowB.ID, rowA.ID}, []int64{single[0].ID, single[1].ID})
		assert.Equal(t, []int64{rowB.ID, rowA.ID}, []int64{batch[0].ID, batch[1].ID})
	})

	t.Run("UNIQUE(instance_id, staff_id) blocks duplicate", func(t *testing.T) {
		dup := &scheduleModels.InstanceStaff{
			InstanceID: inst.ID,
			StaffID:    staffA.ID,
		}
		dup.SetTenantID(testpkg.Tenant(t))
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
		bad.SetTenantID(testpkg.Tenant(t))
		err := repo.Create(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance_id is required")
	})
}

func TestInstanceStaffRepository_FindByStaffAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	date := timezone.NewDate(2026, 9, 16)
	inst, cleanupInst := createInstanceFixture(t, db, "stf-date", date)
	defer cleanupInst()

	staff := testpkg.CreateTestStaff(t, db, "Claire", fmt.Sprintf("Multi-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	row := &scheduleModels.InstanceStaff{
		InstanceID: inst.ID,
		StaffID:    staff.ID,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", row.ID)

	rows, err := repo.FindByStaffAndDate(ctx, staff.ID, date)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 1)
	assert.Equal(t, row.ID, rows[0].ID)

	otherDate := timezone.NewDate(2026, 9, 17)
	rows, err = repo.FindByStaffAndDate(ctx, staff.ID, otherDate)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestInstanceStaffRepository_DeleteByInstanceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "del", timezone.NewDate(2026, 9, 18))
	defer cleanupInst()

	staff := testpkg.CreateTestStaff(t, db, "Dan", fmt.Sprintf("Del-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	row := &scheduleModels.InstanceStaff{InstanceID: inst.ID, StaffID: staff.ID}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))

	require.NoError(t, repo.DeleteByInstanceID(ctx, inst.ID))

	rows, err := repo.FindByInstanceID(ctx, inst.ID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestInstanceStaffRepository_DeleteUpcomingByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	cutoff := timezone.NewDate(2026, 10, 1)
	pastInst, cleanupPast := createInstanceFixture(t, db, "offb-past", cutoff.AddDays(-7))
	defer cleanupPast()
	sameDayPlannedInst, cleanupSameDayPlanned := createInstanceFixture(t, db, "offb-today", cutoff)
	defer cleanupSameDayPlanned()
	sameDayDoneInst, cleanupSameDayDone := createInstanceFixture(t, db, "offb-today-done", cutoff)
	defer cleanupSameDayDone()
	_, err := db.ExecContext(context.Background(),
		`UPDATE schedule.activity_instances SET status = 'completed' WHERE id = ?`, sameDayDoneInst.ID)
	require.NoError(t, err)
	futureInst, cleanupFuture := createInstanceFixture(t, db, "offb-future", cutoff.AddDays(7))
	defer cleanupFuture()

	staff := testpkg.CreateTestStaff(t, db, "Olga", fmt.Sprintf("Offboard-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)
	otherStaff := testpkg.CreateTestStaff(t, db, "Omar", fmt.Sprintf("Other-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, otherStaff.ID)

	makeRow := func(instanceID, staffID int64) *scheduleModels.InstanceStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", row.ID) })
		return row
	}
	pastRow := makeRow(pastInst.ID, staff.ID)
	sameDayPlannedRow := makeRow(sameDayPlannedInst.ID, staff.ID)
	sameDayDoneRow := makeRow(sameDayDoneInst.ID, staff.ID)
	futureRow := makeRow(futureInst.ID, staff.ID)
	otherFutureRow := makeRow(futureInst.ID, otherStaff.ID)

	affected, err := repo.DeleteUpcomingByStaffID(ctx, staff.ID, cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	remaining := func(instanceID int64) []int64 {
		rows, findErr := repo.FindByInstanceID(ctx, instanceID)
		require.NoError(t, findErr)
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		return ids
	}
	assert.Contains(t, remaining(pastInst.ID), pastRow.ID, "past assignment must stay")
	assert.NotContains(t, remaining(sameDayPlannedInst.ID), sameDayPlannedRow.ID,
		"same-day planned assignment must be deleted")
	assert.Contains(t, remaining(sameDayDoneInst.ID), sameDayDoneRow.ID,
		"same-day completed assignment must stay as history")
	futureIDs := remaining(futureInst.ID)
	assert.NotContains(t, futureIDs, futureRow.ID, "future assignment must be deleted")
	assert.Contains(t, futureIDs, otherFutureRow.ID, "other staff's assignment must stay")
}

func TestInstanceStaffRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "upd", timezone.NewDate(2026, 9, 21))
	defer cleanupInst()

	staff := testpkg.CreateTestStaff(t, db, "Eve", fmt.Sprintf("Upd-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	row := &scheduleModels.InstanceStaff{
		InstanceID: inst.ID,
		StaffID:    staff.ID,
	}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", row.ID)

	t.Run("updates fields in place", func(t *testing.T) {
		row.IsPrimary = true
		row.IsSubstitute = false
		row.IsAbsent = true

		require.NoError(t, repo.Update(ctx, row))

		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		assert.True(t, got.IsPrimary)
		assert.False(t, got.IsSubstitute)
		assert.True(t, got.IsAbsent)
	})

	t.Run("rejects nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("rejects invalid payload", func(t *testing.T) {
		bad := &scheduleModels.InstanceStaff{InstanceID: 0, StaffID: staff.ID}
		bad.SetTenantID(testpkg.Tenant(t))
		bad.ID = row.ID
		err := repo.Update(ctx, bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "instance_id is required")
	})
}

func TestInstanceStaffRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "fid", timezone.NewDate(2026, 9, 22))
	defer cleanupInst()

	staff := testpkg.CreateTestStaff(t, db, "Finn", fmt.Sprintf("FID-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	row := &scheduleModels.InstanceStaff{InstanceID: inst.ID, StaffID: staff.ID, IsPrimary: true}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", row.ID)

	t.Run("returns row for known id", func(t *testing.T) {
		got, err := repo.FindByID(ctx, row.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, row.ID, got.ID)
		assert.Equal(t, staff.ID, got.StaffID)
	})

	t.Run("wraps sql.ErrNoRows for missing id", func(t *testing.T) {
		got, err := repo.FindByID(ctx, int64(999999999))
		require.Error(t, err)
		assert.Nil(t, got)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by id", dbErr.Op)
	})

	t.Run("wraps driver errors in DatabaseError", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		got, err := repo.FindByID(cancelledCtx, row.ID)
		assert.Nil(t, got)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by id", dbErr.Op)
	})
}

func TestInstanceStaffRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	inst, cleanupInst := createInstanceFixture(t, db, "lst", timezone.NewDate(2026, 9, 23))
	defer cleanupInst()

	staff := testpkg.CreateTestStaff(t, db, "Gina", fmt.Sprintf("Lst-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)

	row := &scheduleModels.InstanceStaff{InstanceID: inst.ID, StaffID: staff.ID}
	row.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repo.Create(ctx, row))
	defer testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", row.ID)

	t.Run("nil options returns rows", func(t *testing.T) {
		rows, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(rows), 1)
	})

	t.Run("with pagination and filter", func(t *testing.T) {
		options := modelBase.NewQueryOptions()
		options.Filter.Equal("instance_id", inst.ID)
		options.WithPagination(1, 50)

		rows, err := repo.List(ctx, options)
		require.NoError(t, err)
		require.NotEmpty(t, rows)
		for _, r := range rows {
			assert.Equal(t, inst.ID, r.InstanceID)
		}
	})

	t.Run("wraps driver errors in DatabaseError", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		rows, err := repo.List(cancelledCtx, nil)
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "list with options", dbErr.Op)
	})
}

func TestInstanceStaffRepository_ErrorBranches(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	t.Run("FindByInstanceID wraps driver errors", func(t *testing.T) {
		rows, err := repo.FindByInstanceID(cancelledCtx, int64(999999))
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by instance id", dbErr.Op)
	})

	t.Run("FindByStaffAndDate wraps driver errors", func(t *testing.T) {
		rows, err := repo.FindByStaffAndDate(cancelledCtx, int64(999999), timezone.NewDate(2026, 9, 24))
		assert.Nil(t, rows)
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "find by staff and date", dbErr.Op)
	})

	t.Run("DeleteByInstanceID wraps driver errors", func(t *testing.T) {
		err := repo.DeleteByInstanceID(cancelledCtx, int64(999999))
		require.Error(t, err)
		var dbErr *modelBase.DatabaseError
		require.ErrorAs(t, err, &dbErr)
		assert.Equal(t, "delete by instance id", dbErr.Op)
	})
}

func TestInstanceStaffRepository_CountNonAbsentByInstanceIDs(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	t.Run("EmptySlice returns empty map without touching DB", func(t *testing.T) {
		m, err := repo.CountNonAbsentByInstanceIDs(ctx, []int64{})
		require.NoError(t, err)
		assert.Empty(t, m)
	})

	date := timezone.NewDate(2026, 9, 20)
	instA, cleanupA := createInstanceFixture(t, db, "cnt-a", date)
	defer cleanupA()
	instB, cleanupB := createInstanceFixture(t, db, "cnt-b", date)
	defer cleanupB()
	instEmpty, cleanupEmpty := createInstanceFixture(t, db, "cnt-empty", date)
	defer cleanupEmpty()

	suffix := time.Now().UnixNano()
	staff1 := testpkg.CreateTestStaff(t, db, "Cnt", fmt.Sprintf("S1-%d", suffix))
	staff2 := testpkg.CreateTestStaff(t, db, "Cnt", fmt.Sprintf("S2-%d", suffix))
	staff3 := testpkg.CreateTestStaff(t, db, "Cnt", fmt.Sprintf("S3-%d", suffix))
	defer testpkg.CleanupActivityFixtures(t, db, staff1.ID, staff2.ID, staff3.ID)

	// instA: 2 non-absent, 1 absent → expect 2
	rowA1 := testpkg.CreateTestInstanceStaff(t, db, instA.ID, staff1.ID, testpkg.InstanceStaffOpts{})
	rowA2 := testpkg.CreateTestInstanceStaff(t, db, instA.ID, staff2.ID, testpkg.InstanceStaffOpts{})
	rowA3 := testpkg.CreateTestInstanceStaff(t, db, instA.ID, staff3.ID, testpkg.InstanceStaffOpts{IsAbsent: true})

	// instB: 1 non-absent → expect 1
	rowB1 := testpkg.CreateTestInstanceStaff(t, db, instB.ID, staff1.ID, testpkg.InstanceStaffOpts{})

	// instEmpty: nothing assigned → must NOT appear in map
	defer testpkg.CleanupInstanceStaffFixtures(t, db, rowA1.ID, rowA2.ID, rowA3.ID, rowB1.ID)

	t.Run("GroupsByInstance", func(t *testing.T) {
		m, err := repo.CountNonAbsentByInstanceIDs(ctx, []int64{instA.ID, instB.ID, instEmpty.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, m[instA.ID])
		assert.Equal(t, 1, m[instB.ID])
		_, present := m[instEmpty.ID]
		assert.False(t, present, "empty instance must not appear in map")
	})

	t.Run("TenantIsolation excludes other tenant rows", func(t *testing.T) {
		otherTenantCtx := testpkg.TenantContext(999)
		m, err := repo.CountNonAbsentByInstanceIDs(otherTenantCtx, []int64{instA.ID, instB.ID})
		require.NoError(t, err)
		assert.Empty(t, m)
	})
}

func TestInstanceStaffRepository_FindByStaffAndDateRange(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	repo := scheduleRepo.NewInstanceStaffRepository(db)

	instEarly, cleanupEarly := createInstanceFixture(t, db, "range-early", timezone.NewDate(2026, 9, 18))
	defer cleanupEarly()
	instLate, cleanupLate := createInstanceFixture(t, db, "range-late", timezone.NewDate(2026, 9, 20))
	defer cleanupLate()
	instOutside, cleanupOutside := createInstanceFixture(t, db, "range-out", timezone.NewDate(2026, 9, 25))
	defer cleanupOutside()

	staff := testpkg.CreateTestStaff(t, db, "Dana", fmt.Sprintf("Range-%d", time.Now().UnixNano()))
	other := testpkg.CreateTestStaff(t, db, "Erik", fmt.Sprintf("Other-%d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID, other.ID)

	newRow := func(instID, staffID int64) *scheduleModels.InstanceStaff {
		row := &scheduleModels.InstanceStaff{InstanceID: instID, StaffID: staffID}
		row.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repo.Create(ctx, row))
		t.Cleanup(func() { testpkg.CleanupTableRecords(t, db, "schedule.instance_staff", row.ID) })
		return row
	}

	rowLate := newRow(instLate.ID, staff.ID)
	rowEarly := newRow(instEarly.ID, staff.ID)
	newRow(instOutside.ID, staff.ID) // outside the queried window
	newRow(instEarly.ID, other.ID)   // another staff member — must not leak

	rows, err := repo.FindByStaffAndDateRange(ctx, staff.ID, timezone.NewDate(2026, 9, 18), timezone.NewDate(2026, 9, 20))
	require.NoError(t, err)
	require.Len(t, rows, 2, "only the calling staff member's in-window rows come back")
	assert.Equal(t, rowEarly.ID, rows[0].ID, "ordered by instance date ascending")
	assert.Equal(t, rowLate.ID, rows[1].ID)

	empty, err := repo.FindByStaffAndDateRange(ctx, staff.ID, timezone.NewDate(2026, 10, 1), timezone.NewDate(2026, 10, 5))
	require.NoError(t, err)
	assert.Empty(t, empty)
}
