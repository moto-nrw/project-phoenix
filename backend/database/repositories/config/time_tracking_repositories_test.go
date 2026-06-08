package config_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffWorkScheduleReplaceSchedule_UsesExclusiveValidUntil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Schedule", "Exclusive")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)
	defer cleanupStaffWorkSchedules(t, db, staff.ID)

	repo := configRepo.NewStaffWorkScheduleRepository(db)
	require.NoError(t, repo.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{
		{
			DayOfWeek:      configModel.DayMonday,
			TargetMinutes:  240,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}))
	require.NoError(t, repo.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{
		{
			DayOfWeek:      configModel.DayTuesday,
			TargetMinutes:  300,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}))

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	entries, err := repo.GetByStaffIDAndDate(ctx, staff.ID, today)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, configModel.DayTuesday, entries[0].DayOfWeek)
	assert.Equal(t, 300, entries[0].TargetMinutes)
}

func TestStaffWorkScheduleReplaceSchedule_InvalidEntryKeepsCurrentSchedule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Schedule", "Invalid")
	defer testpkg.CleanupActivityFixtures(t, db, staff.ID)
	defer cleanupStaffWorkSchedules(t, db, staff.ID)

	repo := configRepo.NewStaffWorkScheduleRepository(db)
	require.NoError(t, repo.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{
		{
			DayOfWeek:      configModel.DayMonday,
			TargetMinutes:  240,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}))

	err := repo.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{
		{
			DayOfWeek:      9,
			TargetMinutes:  300,
			WeekIndex:      0,
			RotationLength: 1,
		},
	})
	require.Error(t, err)

	entries, err := repo.GetCurrentByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, configModel.DayMonday, entries[0].DayOfWeek)
	assert.Equal(t, 240, entries[0].TargetMinutes)
}

func TestStaffWorkScheduleGetByStaffIDAndDate_DoesNotLeakOtherStaffRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := testpkg.TenantContext(1)

	ownStaff := testpkg.CreateTestStaff(t, db, "Schedule", "Owner")
	otherStaff := testpkg.CreateTestStaff(t, db, "Schedule", "Other")
	defer testpkg.CleanupStaffFixtures(t, db, otherStaff.ID, ownStaff.ID)
	defer cleanupStaffWorkSchedules(t, db, ownStaff.ID)
	defer cleanupStaffWorkSchedules(t, db, otherStaff.ID)

	queryDate := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	otherRow := &configModel.StaffWorkSchedule{
		StaffID:        otherStaff.ID,
		WeekIndex:      0,
		RotationLength: 1,
		DayOfWeek:      configModel.DayMonday,
		TargetMinutes:  480,
		ValidFrom:      queryDate.AddDate(0, 0, 10),
		ValidUntil:     ptrTime(queryDate.AddDate(0, 0, 20)),
	}
	otherRow.SetTenantID(1)
	_, err := db.NewInsert().
		Model(otherRow).
		ModelTableExpr("config.staff_work_schedules").
		Exec(ctx)
	require.NoError(t, err)

	repo := configRepo.NewStaffWorkScheduleRepository(db)
	entries, err := repo.GetByStaffIDAndDate(ctx, ownStaff.ID, queryDate)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestWorkTimeModelRefreshAssignedStaffSchedules_UpdatesCurrentSnapshots(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Template", "Refresh")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer cleanupStaffWorkSchedules(t, db, staff.ID)

	repo := configRepo.NewWorkTimeModelRepository(db)
	model := &configModel.WorkTimeModel{
		Name:               "Refresh assigned schedule",
		RotationLength:     1,
		RotationAnchorDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.Create(ctx, model, []*configModel.WorkTimeModelEntry{
		{
			WeekIndex:     0,
			DayOfWeek:     configModel.DayMonday,
			TargetMinutes: 300,
		},
	}))
	defer func() {
		_, _ = db.NewUpdate().
			Table("users.staff").
			Set("work_time_model_id = NULL").
			Where("id = ?", staff.ID).
			Exec(ctx)
		_ = repo.Delete(ctx, model.ID)
	}()

	scheduleRepo := configRepo.NewStaffWorkScheduleRepository(db)
	require.NoError(t, scheduleRepo.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{
		{
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      configModel.DayMonday,
			TargetMinutes:  300,
		},
	}))
	_, err := db.NewUpdate().
		Table("users.staff").
		Set("work_time_model_id = ?", model.ID).
		Set("rotation_anchor_date = ?", model.RotationAnchorDate).
		Where("id = ?", staff.ID).
		Exec(ctx)
	require.NoError(t, err)

	updated := &configModel.WorkTimeModel{
		ID:                 model.ID,
		Name:               model.Name,
		RotationLength:     1,
		RotationAnchorDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repo.Update(ctx, updated, []*configModel.WorkTimeModelEntry{
		{
			WeekIndex:     0,
			DayOfWeek:     configModel.DayTuesday,
			TargetMinutes: 420,
		},
	}))
	require.NoError(t, repo.RefreshAssignedStaffSchedules(ctx, model.ID))

	currentRows, err := scheduleRepo.GetCurrentByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, currentRows, 1)
	assert.Equal(t, configModel.DayTuesday, currentRows[0].DayOfWeek)
	assert.Equal(t, 420, currentRows[0].TargetMinutes)

	var oldRows int
	err = db.NewSelect().
		TableExpr("config.staff_work_schedules").
		ColumnExpr("COUNT(*)").
		Where("staff_id = ?", staff.ID).
		Where("valid_until IS NOT NULL").
		Scan(ctx, &oldRows)
	require.NoError(t, err)
	assert.Positive(t, oldRows)
}

func TestWorkTimeModelUpdate_MissingModelDoesNotDeleteEntries(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := testpkg.TenantContext(1)

	repo := configRepo.NewWorkTimeModelRepository(db)
	model := &configModel.WorkTimeModel{
		Name:               "Update safety test",
		RotationLength:     1,
		RotationAnchorDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
	}
	entries := []*configModel.WorkTimeModelEntry{
		{
			WeekIndex:     0,
			DayOfWeek:     configModel.DayMonday,
			TargetMinutes: 480,
		},
	}
	require.NoError(t, repo.Create(ctx, model, entries))
	defer func() { _ = repo.Delete(ctx, model.ID) }()

	missing := &configModel.WorkTimeModel{
		ID:                 model.ID + 100000,
		Name:               "Missing update",
		RotationLength:     1,
		RotationAnchorDate: model.RotationAnchorDate,
	}
	err := repo.Update(ctx, missing, nil)
	require.True(t, errors.Is(err, sql.ErrNoRows), "expected sql.ErrNoRows, got %v", err)

	found, err := repo.FindByID(ctx, model.ID)
	require.NoError(t, err)
	require.Len(t, found.Entries, 1)
	assert.Equal(t, 480, found.Entries[0].TargetMinutes)
}

func TestWorkTimeModelDelete_BlocksAssignedModel(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Assigned", "Template")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	repo := configRepo.NewWorkTimeModelRepository(db)
	model := &configModel.WorkTimeModel{
		Name:               "Assigned delete safety",
		RotationLength:     1,
		RotationAnchorDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
	}
	entries := []*configModel.WorkTimeModelEntry{
		{
			WeekIndex:     0,
			DayOfWeek:     configModel.DayMonday,
			TargetMinutes: 480,
		},
	}
	require.NoError(t, repo.Create(ctx, model, entries))
	defer func() {
		_, _ = db.NewUpdate().
			Table("users.staff").
			Set("work_time_model_id = NULL").
			Where("id = ?", staff.ID).
			Exec(ctx)
		_ = repo.Delete(ctx, model.ID)
	}()

	_, err := db.NewUpdate().
		Table("users.staff").
		Set("work_time_model_id = ?", model.ID).
		Where("id = ?", staff.ID).
		Exec(ctx)
	require.NoError(t, err)

	err = repo.Delete(ctx, model.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "assigned to staff")

	found, err := repo.FindByID(ctx, model.ID)
	require.NoError(t, err)
	assert.Equal(t, model.ID, found.ID)
}

func cleanupStaffWorkSchedules(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	_, err := db.NewDelete().
		Table("config.staff_work_schedules").
		Where("staff_id = ?", staffID).
		Exec(context.Background())
	require.NoError(t, err)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
