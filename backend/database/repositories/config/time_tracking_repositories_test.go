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

func cleanupStaffWorkSchedules(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	_, err := db.NewDelete().
		Table("config.staff_work_schedules").
		Where("staff_id = ?", staffID).
		Exec(context.Background())
	require.NoError(t, err)
}
