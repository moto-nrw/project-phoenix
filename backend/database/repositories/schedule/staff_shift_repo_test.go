package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func wallClock(hour, minute int) time.Time {
	return time.Date(1, 1, 1, hour, minute, 0, 0, time.UTC)
}

func newShift(staffID int64, date timezone.Date, startHour, endHour int, createdBy int64) *scheduleModels.StaffShift {
	return &scheduleModels.StaffShift{
		StaffID:   staffID,
		Date:      date,
		StartTime: wallClock(startHour, 0),
		EndTime:   wallClock(endHour, 30),
		CreatedBy: createdBy,
	}
}

func cleanupShifts(t *testing.T, repo scheduleModels.StaffShiftRepository, ctx context.Context, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		_ = repo.Delete(ctx, id)
	}
}

func TestStaffShiftRepository_CreateFindNormalizesWallClock(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffShift
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Shift", "Owner")
	defer testpkg.CleanupActivityFixtures(t, db, staff.PersonID)

	monday := timezone.NewDate(2026, time.July, 6)
	shift := newShift(staff.ID, monday, 8, 16, staff.ID)
	require.NoError(t, repo.Create(ctx, shift))
	defer cleanupShifts(t, repo, ctx, shift.ID)
	require.NotZero(t, shift.ID)
	assert.Equal(t, int64(1), shift.TenantID, "tenant_id must be stamped from context")

	rows, err := repo.FindByStaffAndDateRange(ctx, staff.ID, monday, monday)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	got := rows[0]
	assert.Equal(t, monday, got.Date)
	// TIME columns scan back with a driver-arbitrary year anchor; the repo
	// must normalize to WallClock so comparisons and formatting work.
	assert.Equal(t, wallClock(8, 0), got.StartTime)
	assert.Equal(t, wallClock(16, 30), got.EndTime)
	assert.Equal(t, "08:00", timezone.WallClock(got.StartTime).Format("15:04"))
}

func TestStaffShiftRepository_UpdateDeleteUseTenantAlias(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffShift
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Shift", "WriteAlias")
	defer testpkg.CleanupActivityFixtures(t, db, staff.PersonID)

	day := timezone.NewDate(2026, time.July, 6)
	shift := newShift(staff.ID, day, 8, 16, staff.ID)
	require.NoError(t, repo.Create(ctx, shift))
	defer cleanupShifts(t, repo, ctx, shift.ID)

	shift.EndTime = wallClock(17, 0)
	shift.Notes = "Updated through generic repo"
	require.NoError(t, repo.Update(ctx, shift), "tenant-scoped update must keep the staff_shift alias")

	rows, err := repo.FindByStaffAndDateRange(ctx, staff.ID, day, day)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, wallClock(17, 0), rows[0].EndTime)
	assert.Equal(t, "Updated through generic repo", rows[0].Notes)

	require.NoError(t, repo.Delete(ctx, shift.ID), "tenant-scoped delete must keep the staff_shift alias")
	rows, err = repo.FindByStaffAndDateRange(ctx, staff.ID, day, day)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStaffShiftRepository_FindByStaffIDsAndDate(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffShift
	ctx := testpkg.TenantContext(1)

	staffA := testpkg.CreateTestStaff(t, db, "ShiftA", "Batch")
	staffB := testpkg.CreateTestStaff(t, db, "ShiftB", "Batch")
	defer testpkg.CleanupActivityFixtures(t, db, staffA.PersonID, staffB.PersonID)

	day := timezone.NewDate(2026, time.July, 7)
	other := day.AddDays(1)

	s1 := newShift(staffA.ID, day, 8, 12, staffA.ID)
	s2 := newShift(staffA.ID, day, 13, 17, staffA.ID)
	s3 := newShift(staffB.ID, day, 9, 15, staffA.ID)
	s4 := newShift(staffB.ID, other, 9, 15, staffA.ID) // other day, must not appear
	for _, s := range []*scheduleModels.StaffShift{s1, s2, s3, s4} {
		require.NoError(t, repo.Create(ctx, s))
	}
	defer cleanupShifts(t, repo, ctx, s1.ID, s2.ID, s3.ID, s4.ID)

	rows, err := repo.FindByStaffIDsAndDate(ctx, []int64{staffA.ID, staffB.ID}, day)
	require.NoError(t, err)
	assert.Len(t, rows, 3)

	// Empty id list short-circuits without touching the DB.
	rows, err = repo.FindByStaffIDsAndDate(ctx, nil, day)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestStaffShiftRepository_CoverageReadsExactStaffDatesAndUsedWeeks(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffShift
	ctx := testpkg.TenantContext(1)
	staffA := testpkg.CreateTestStaff(t, db, "CoverageA", "Batch")
	staffB := testpkg.CreateTestStaff(t, db, "CoverageB", "Batch")
	defer testpkg.CleanupActivityFixtures(t, db, staffA.PersonID, staffB.PersonID)

	monday := timezone.NewDate(2026, time.July, 6)
	saturday := monday.AddDays(5)
	nextMonday := monday.AddDays(7)
	thirdMonday := monday.AddDays(14)
	// The third week holds ONLY a cancelled shift: it must not count as a used
	// week — every consumer ignores cancelled shifts in its per-staff data, so
	// reporting the week as planned would suppress the missing-Dienstplan hint.
	cancelledOnly := newShift(staffA.ID, thirdMonday, 8, 12, staffA.ID)
	cancelledOnly.Cancelled = true
	rows := []*scheduleModels.StaffShift{
		newShift(staffA.ID, monday, 8, 12, staffA.ID),
		newShift(staffA.ID, saturday, 8, 12, staffA.ID),
		newShift(staffA.ID, nextMonday, 8, 12, staffA.ID),
		newShift(staffB.ID, monday, 8, 12, staffA.ID),
		cancelledOnly,
	}
	for _, shift := range rows {
		require.NoError(t, repo.Create(ctx, shift))
	}
	defer cleanupShifts(t, repo, ctx, rows[0].ID, rows[1].ID, rows[2].ID, rows[3].ID, rows[4].ID)

	shifts, err := repo.FindByStaffIDsAndDates(ctx, []int64{staffA.ID}, []timezone.Date{monday, nextMonday})
	require.NoError(t, err)
	require.Len(t, shifts, 2)
	assert.Equal(t, monday, shifts[0].Date)
	assert.Equal(t, nextMonday, shifts[1].Date)

	weeks, err := repo.FindUsedCalendarWeeks(ctx, monday, thirdMonday.AddDays(6))
	require.NoError(t, err)
	assert.Equal(t, []timezone.Date{monday, nextMonday}, weeks,
		"the cancelled-only week must not report as used")
}

func TestStaffShiftRepository_DeleteUpcomingByStaffID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffShift
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Shift", "DeleteUpcoming")
	defer testpkg.CleanupActivityFixtures(t, db, staff.PersonID)

	today := timezone.TodayDate()
	past := newShift(staff.ID, today.AddDays(-1), 8, 12, staff.ID)
	sameDay := newShift(staff.ID, today, 8, 12, staff.ID)
	future := newShift(staff.ID, today.AddDays(1), 8, 12, staff.ID)
	for _, s := range []*scheduleModels.StaffShift{past, sameDay, future} {
		require.NoError(t, repo.Create(ctx, s))
	}
	defer cleanupShifts(t, repo, ctx, past.ID, sameDay.ID, future.ID)

	const otherTenantID = int64(910004)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherTenantCtx := tenant.WithTenantID(context.Background(), otherTenantID)
	deleted, err := repo.DeleteUpcomingByStaffID(otherTenantCtx, staff.ID, today)
	require.NoError(t, err)
	assert.Zero(t, deleted, "wrong tenant must not delete tenant 1 shifts")

	deleted, err = repo.DeleteUpcomingByStaffID(ctx, staff.ID, today)
	require.NoError(t, err)
	expectedDeleted := int64(0)
	for range []*scheduleModels.StaffShift{sameDay, future} {
		expectedDeleted++
	}
	assert.Equal(t, expectedDeleted, deleted)

	rows, err := repo.FindByStaffAndDateRange(ctx, staff.ID, today.AddDays(-2), today.AddDays(2))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, past.ID, rows[0].ID)
}

func TestStaffShiftRepository_DuplicateStartRejected(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffShift
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Shift", "Dup")
	defer testpkg.CleanupActivityFixtures(t, db, staff.PersonID)

	day := timezone.NewDate(2026, time.July, 8)
	first := newShift(staff.ID, day, 8, 16, staff.ID)
	require.NoError(t, repo.Create(ctx, first))
	defer cleanupShifts(t, repo, ctx, first.ID)

	dup := newShift(staff.ID, day, 8, 14, staff.ID) // same start_time
	err := repo.Create(ctx, dup)
	require.Error(t, err, "unique(tenant, staff, date, start_time) must reject the duplicate")
}

func TestStaffShiftRepository_TenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffShift
	tenant1 := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Shift", "Isolated")
	defer testpkg.CleanupActivityFixtures(t, db, staff.PersonID)

	const otherTenantID = int64(910002)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	tenant2 := tenant.WithTenantID(context.Background(), otherTenantID)

	day := timezone.NewDate(2026, time.July, 9)
	shift := newShift(staff.ID, day, 8, 16, staff.ID)
	require.NoError(t, repo.Create(tenant1, shift))
	defer cleanupShifts(t, repo, tenant1, shift.ID)

	rows, err := repo.FindByDateRange(tenant2, day, day)
	require.NoError(t, err)
	assert.Empty(t, rows, "tenant 2 must not see tenant 1's shifts")

	rows, err = repo.FindByDateRange(tenant1, day, day)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, shift.ID, rows[0].ID)
}

func TestStaffShiftRepository_RejectsCrossTenantStaffReference(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).StaffShift
	tenant1 := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Shift", "CrossTenant")
	defer testpkg.CleanupActivityFixtures(t, db, staff.PersonID)

	const otherTenantID = int64(910003)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	tenant2 := tenant.WithTenantID(context.Background(), otherTenantID)

	day := timezone.NewDate(2026, time.July, 10)
	shift := newShift(staff.ID, day, 8, 16, staff.ID)
	err := repo.Create(tenant2, shift)
	require.Error(t, err, "tenant 2 shift must not reference tenant 1 staff")

	rows, findErr := repo.FindByDateRange(tenant1, day, day)
	require.NoError(t, findErr)
	assert.Empty(t, rows, "failed cross-tenant insert must not leave tenant 1 data behind")
}
