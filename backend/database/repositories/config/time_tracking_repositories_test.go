package config_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffWorkScheduleReplaceSchedule_UsesExclusiveValidUntil(t *testing.T) {
	db := testpkg.SetupTestDB(t)
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
	}, timezone.Date{}))
	require.NoError(t, repo.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{
		{
			DayOfWeek:      configModel.DayTuesday,
			TargetMinutes:  300,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}, timezone.Date{}))

	today := timezone.TodayDate()
	entries, err := repo.GetByStaffIDAndDate(ctx, staff.ID, today)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, configModel.DayTuesday, entries[0].DayOfWeek)
	assert.Equal(t, 300, entries[0].TargetMinutes)
}

func TestStaffWorkScheduleReplaceSchedule_InvalidEntryKeepsCurrentSchedule(t *testing.T) {
	db := testpkg.SetupTestDB(t)
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
	}, timezone.Date{}))

	err := repo.ReplaceSchedule(ctx, staff.ID, []*configModel.StaffWorkSchedule{
		{
			DayOfWeek:      9,
			TargetMinutes:  300,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}, timezone.Date{})
	require.Error(t, err)

	entries, err := repo.GetCurrentByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, configModel.DayMonday, entries[0].DayOfWeek)
	assert.Equal(t, 240, entries[0].TargetMinutes)
}

func TestStaffWorkScheduleGetByStaffIDAndDate_DoesNotLeakOtherStaffRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.TenantContext(1)

	ownStaff := testpkg.CreateTestStaff(t, db, "Schedule", "Owner")
	otherStaff := testpkg.CreateTestStaff(t, db, "Schedule", "Other")
	defer testpkg.CleanupStaffFixtures(t, db, otherStaff.ID, ownStaff.ID)
	defer cleanupStaffWorkSchedules(t, db, ownStaff.ID)
	defer cleanupStaffWorkSchedules(t, db, otherStaff.ID)

	queryDate := timezone.NewDate(2026, time.June, 8)
	otherRow := &configModel.StaffWorkSchedule{
		StaffID:        otherStaff.ID,
		WeekIndex:      0,
		RotationLength: 1,
		DayOfWeek:      configModel.DayMonday,
		TargetMinutes:  480,
		ValidFrom:      queryDate.AddDays(10),
		ValidUntil:     ptrDate(queryDate.AddDays(20)),
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
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Template", "Refresh")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	defer cleanupStaffWorkSchedules(t, db, staff.ID)

	repo := configRepo.NewWorkTimeModelRepository(db)
	model := &configModel.WorkTimeModel{
		Name:               "Refresh assigned schedule",
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2026, time.January, 5),
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
	}, timezone.Date{}))
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
		RotationAnchorDate: timezone.NewDate(2026, time.June, 1),
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
	ctx := testpkg.TenantContext(1)

	repo := configRepo.NewWorkTimeModelRepository(db)
	model := &configModel.WorkTimeModel{
		Name:               "Update safety test",
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2026, time.January, 5),
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
	ctx := testpkg.TenantContext(1)

	staff := testpkg.CreateTestStaff(t, db, "Assigned", "Template")
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)
	repo := configRepo.NewWorkTimeModelRepository(db)
	model := &configModel.WorkTimeModel{
		Name:               "Assigned delete safety",
		RotationLength:     1,
		RotationAnchorDate: timezone.NewDate(2026, time.January, 5),
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

func TestStaffWorkScheduleFindByStaffIDsValidInRange_BatchesAndIsolates(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.TenantContext(1)
	foreignTenantID := int64(1837001)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)

	first := testpkg.CreateTestStaff(t, db, "Range", "First")
	second := testpkg.CreateTestStaff(t, db, "Range", "Second")
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Range", "Foreign")
	defer testpkg.CleanupStaffFixtures(t, db, first.ID, second.ID, foreign.ID)
	defer cleanupStaffWorkSchedules(t, db, first.ID)
	defer cleanupStaffWorkSchedules(t, db, second.ID)
	defer cleanupStaffWorkSchedules(t, db, foreign.ID)

	from := timezone.NewDate(2026, time.June, 8)
	to := from.AddDays(6)
	rows := []*configModel.StaffWorkSchedule{
		// Intersects: open-ended and valid before the range.
		{StaffID: first.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayMonday, TargetMinutes: 240, ValidFrom: from.AddDays(-30)},
		// Intersects: window ends inside the range (valid_until exclusive).
		{StaffID: second.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayTuesday, TargetMinutes: 300, ValidFrom: from.AddDays(-30), ValidUntil: ptrDate(from.AddDays(2))},
		// Disjoint: window closed before the range starts.
		{StaffID: first.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayWednesday, TargetMinutes: 700, ValidFrom: from.AddDays(-30), ValidUntil: ptrDate(from)},
		// Disjoint: window starts after the range ends.
		{StaffID: second.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: configModel.DayThursday, TargetMinutes: 700, ValidFrom: to.AddDays(1)},
	}
	for _, row := range rows {
		row.SetTenantID(1)
		_, err := db.NewInsert().Model(row).ModelTableExpr("config.staff_work_schedules").Exec(ctx)
		require.NoError(t, err)
	}
	foreignRow := &configModel.StaffWorkSchedule{
		StaffID: foreign.ID, WeekIndex: 0, RotationLength: 1,
		DayOfWeek: configModel.DayMonday, TargetMinutes: 480, ValidFrom: from.AddDays(-30),
	}
	foreignRow.SetTenantID(foreignTenantID)
	_, err := db.NewInsert().Model(foreignRow).ModelTableExpr("config.staff_work_schedules").Exec(ctx)
	require.NoError(t, err)

	repo := configRepo.NewStaffWorkScheduleRepository(db)

	empty, err := repo.FindByStaffIDsValidInRange(ctx, nil, from, to)
	require.NoError(t, err)
	assert.Nil(t, empty)

	entries, err := repo.FindByStaffIDsValidInRange(ctx, []int64{first.ID, second.ID, foreign.ID}, from, to)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	byDay := make(map[int]int64, len(entries))
	for _, entry := range entries {
		byDay[entry.DayOfWeek] = entry.StaffID
		assert.NotEqual(t, 700, entry.TargetMinutes, "disjoint validity windows must not be returned")
		assert.NotEqual(t, foreign.ID, entry.StaffID, "foreign tenant rows must not cross the tenant boundary")
	}
	assert.Equal(t, first.ID, byDay[configModel.DayMonday])
	assert.Equal(t, second.ID, byDay[configModel.DayTuesday])
}

func TestWorkTimeModelFindByIDs_BatchesEntriesAndIsolates(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.TenantContext(1)
	foreignTenantID := int64(1837002)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	repo := configRepo.NewWorkTimeModelRepository(db)
	anchor := timezone.NewDate(2026, time.January, 5)

	localA := &configModel.WorkTimeModel{Name: "FindByIDs A", RotationLength: 1, RotationAnchorDate: anchor}
	require.NoError(t, repo.Create(ctx, localA, []*configModel.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: configModel.DayMonday, TargetMinutes: 240},
		{WeekIndex: 0, DayOfWeek: configModel.DayTuesday, TargetMinutes: 300},
	}))
	localB := &configModel.WorkTimeModel{Name: "FindByIDs B", RotationLength: 2, RotationAnchorDate: anchor}
	require.NoError(t, repo.Create(ctx, localB, []*configModel.WorkTimeModelEntry{
		{WeekIndex: 1, DayOfWeek: configModel.DayFriday, TargetMinutes: 180},
	}))
	foreignModel := &configModel.WorkTimeModel{Name: "FindByIDs Foreign", RotationLength: 1, RotationAnchorDate: anchor}
	require.NoError(t, repo.Create(foreignCtx, foreignModel, []*configModel.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: configModel.DayMonday, TargetMinutes: 480},
	}))
	defer func() {
		_ = repo.Delete(ctx, localA.ID)
		_ = repo.Delete(ctx, localB.ID)
		_ = repo.Delete(foreignCtx, foreignModel.ID)
	}()

	empty, err := repo.FindByIDs(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, empty)

	models, err := repo.FindByIDs(ctx, []int64{localA.ID, localB.ID, foreignModel.ID})
	require.NoError(t, err)
	require.Len(t, models, 2, "foreign tenant models must be absent from the result")
	byID := make(map[int64]*configModel.WorkTimeModel, len(models))
	for _, model := range models {
		byID[model.ID] = model
	}
	require.Contains(t, byID, localA.ID)
	require.Contains(t, byID, localB.ID)
	assert.Len(t, byID[localA.ID].Entries, 2)
	require.Len(t, byID[localB.ID].Entries, 1)
	assert.Equal(t, 180, byID[localB.ID].Entries[0].TargetMinutes)
}

func cleanupStaffWorkSchedules(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	_, err := db.NewDelete().
		Table("config.staff_work_schedules").
		Where("staff_id = ?", staffID).
		Exec(context.Background())
	require.NoError(t, err)
}

func ptrDate(d timezone.Date) *timezone.Date {
	return &d
}
