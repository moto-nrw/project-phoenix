package api

import (
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Workforce adapters that serve the Settings Platform's retained
// work-time contracts (staff work schedules and templates) over the Workforce
// capability. The adapters translate between two owners, so their suite runs
// in the root.

// The schedule day indexes (Monday=0 … Sunday=6) the fixtures use.
const (
	adapterDayMonday    = 0
	adapterDayTuesday   = 1
	adapterDayWednesday = 2
	adapterDayThursday  = 3
	adapterDayFriday    = 4
)

// adapterWorkforce composes the Workforce capability over the test database
// with the School Membership owner.
func adapterWorkforce(t *testing.T, db *bun.DB) workforce.Capability {
	t.Helper()
	membership, err := owners.NewSchoolMembership(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	capability, err := owners.NewWorkforce(testpkg.NewDatabase(t, db), membership)
	require.NoError(t, err)
	return capability
}

func adapterScheduleRepository(t *testing.T, db *bun.DB) configModels.StaffWorkScheduleRepository {
	t.Helper()
	return owners.NewStaffWorkScheduleRepository(adapterWorkforce(t, db))
}

func adapterModelRepository(t *testing.T, db *bun.DB) configModels.WorkTimeModelRepository {
	t.Helper()
	return owners.NewWorkTimeModelRepository(adapterWorkforce(t, db))
}

func adapterCalendarDate(year int, month time.Month, day int) configModels.CalendarDate {
	return configModels.CalendarDate(calendar.NewDate(year, month, day).String())
}

func adapterDatePointer(d configModels.CalendarDate) *configModels.CalendarDate {
	return &d
}

func TestStaffWorkScheduleReplaceSchedule_UsesExclusiveValidUntil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Schedule", "Exclusive")

	repo := adapterScheduleRepository(t, db)
	require.NoError(t, repo.ReplaceSchedule(ctx, staff.ID, []*configModels.StaffWorkSchedule{
		{
			DayOfWeek:      adapterDayMonday,
			TargetMinutes:  240,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}, configModels.CalendarDate("")))
	require.NoError(t, repo.ReplaceSchedule(ctx, staff.ID, []*configModels.StaffWorkSchedule{
		{
			DayOfWeek:      adapterDayTuesday,
			TargetMinutes:  300,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}, configModels.CalendarDate("")))

	today := configModels.CalendarDate(calendar.TodayDate().String())
	entries, err := repo.GetByStaffIDAndDate(ctx, staff.ID, today)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, adapterDayTuesday, entries[0].DayOfWeek)
	assert.Equal(t, 300, entries[0].TargetMinutes)
}

func TestStaffWorkScheduleReplaceSchedule_InvalidEntryKeepsCurrentSchedule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Schedule", "Invalid")

	repo := adapterScheduleRepository(t, db)
	require.NoError(t, repo.ReplaceSchedule(ctx, staff.ID, []*configModels.StaffWorkSchedule{
		{
			DayOfWeek:      adapterDayMonday,
			TargetMinutes:  240,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}, configModels.CalendarDate("")))

	err := repo.ReplaceSchedule(ctx, staff.ID, []*configModels.StaffWorkSchedule{
		{
			DayOfWeek:      9,
			TargetMinutes:  300,
			WeekIndex:      0,
			RotationLength: 1,
		},
	}, configModels.CalendarDate(""))
	require.Error(t, err)

	entries, err := repo.GetCurrentByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, adapterDayMonday, entries[0].DayOfWeek)
	assert.Equal(t, 240, entries[0].TargetMinutes)
}

func TestStaffWorkScheduleGetByStaffIDAndDate_DoesNotLeakOtherStaffRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	ownStaff := testpkg.CreateTestStaff(t, db, "Schedule", "Owner")
	otherStaff := testpkg.CreateTestStaff(t, db, "Schedule", "Other")

	queryDate := adapterCalendarDate(2026, time.June, 8)
	otherRow := &configModels.StaffWorkSchedule{
		StaffID:        otherStaff.ID,
		WeekIndex:      0,
		RotationLength: 1,
		DayOfWeek:      adapterDayMonday,
		TargetMinutes:  480,
		ValidFrom:      queryDate.AddDays(10),
		ValidUntil:     adapterDatePointer(queryDate.AddDays(20)),
	}
	otherRow.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().
		Model(otherRow).
		ModelTableExpr("config.staff_work_schedules").
		Exec(ctx)
	require.NoError(t, err)

	repo := adapterScheduleRepository(t, db)
	entries, err := repo.GetByStaffIDAndDate(ctx, ownStaff.ID, queryDate)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestWorkTimeModelRefreshAssignedStaffSchedules_UpdatesCurrentSnapshots(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Template", "Refresh")

	repo := adapterModelRepository(t, db)
	model := &configModels.WorkTimeModel{
		Name:               "Refresh assigned schedule",
		RotationLength:     1,
		RotationAnchorDate: adapterCalendarDate(2026, time.January, 5),
	}
	require.NoError(t, repo.Create(ctx, model, []*configModels.WorkTimeModelEntry{
		{
			WeekIndex:     0,
			DayOfWeek:     adapterDayMonday,
			TargetMinutes: 300,
		},
	}))
	defer func() {
		_, _ = db.NewUpdate().
			Table("users.staff_employment_profiles").
			Set("work_time_model_id = NULL").
			Where("membership_id = ?", staff.ID).
			Exec(ctx)
		_ = repo.Delete(ctx, model.ID)
	}()

	scheduleRepo := adapterScheduleRepository(t, db)
	require.NoError(t, scheduleRepo.ReplaceSchedule(ctx, staff.ID, []*configModels.StaffWorkSchedule{
		{
			WeekIndex:      0,
			RotationLength: 1,
			DayOfWeek:      adapterDayMonday,
			TargetMinutes:  300,
		},
	}, configModels.CalendarDate("")))
	_, err := db.NewUpdate().
		Table("users.staff_employment_profiles").
		Set("work_time_model_id = ?", model.ID).
		Set("rotation_anchor_date = ?", model.RotationAnchorDate).
		Where("membership_id = ?", staff.ID).
		Exec(ctx)
	require.NoError(t, err)

	// One capability call: the template edit and the assigned-staff refresh
	// are the same unit of work.
	_, err = adapterWorkforce(t, db).UpdateWorkTimeModel(ctx, workforce.UpdateWorkTimeModel{
		ID: model.ID,
		WorkTimeModelFields: workforce.WorkTimeModelFields{
			Name:               model.Name,
			RotationLength:     1,
			RotationAnchorDate: "2026-06-01",
			Entries: []workforce.WorkTimeModelEntry{
				{WeekIndex: 0, DayOfWeek: adapterDayTuesday, TargetMinutes: 420},
			},
		},
	})
	require.NoError(t, err)

	currentRows, err := scheduleRepo.GetCurrentByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, currentRows, 1)
	assert.Equal(t, adapterDayTuesday, currentRows[0].DayOfWeek)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	repo := adapterModelRepository(t, db)
	model := &configModels.WorkTimeModel{
		Name:               "Update safety test",
		RotationLength:     1,
		RotationAnchorDate: adapterCalendarDate(2026, time.January, 5),
	}
	entries := []*configModels.WorkTimeModelEntry{
		{
			WeekIndex:     0,
			DayOfWeek:     adapterDayMonday,
			TargetMinutes: 480,
		},
	}
	require.NoError(t, repo.Create(ctx, model, entries))
	defer func() { _ = repo.Delete(ctx, model.ID) }()

	_, err := adapterWorkforce(t, db).UpdateWorkTimeModel(ctx, workforce.UpdateWorkTimeModel{
		ID: model.ID + 100000,
		WorkTimeModelFields: workforce.WorkTimeModelFields{
			Name:               "Missing update",
			RotationLength:     1,
			RotationAnchorDate: string(model.RotationAnchorDate),
		},
	})
	require.ErrorIs(t, err, workforce.ErrWorkTimeModelNotFound)

	found, err := repo.FindByID(ctx, model.ID)
	require.NoError(t, err)
	require.Len(t, found.Entries, 1)
	assert.Equal(t, 480, found.Entries[0].TargetMinutes)
}

func TestWorkTimeModelDelete_BlocksAssignedModel(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)

	staff := testpkg.CreateTestStaff(t, db, "Assigned", "Template")
	repo := adapterModelRepository(t, db)
	model := &configModels.WorkTimeModel{
		Name:               "Assigned delete safety",
		RotationLength:     1,
		RotationAnchorDate: adapterCalendarDate(2026, time.January, 5),
	}
	entries := []*configModels.WorkTimeModelEntry{
		{
			WeekIndex:     0,
			DayOfWeek:     adapterDayMonday,
			TargetMinutes: 480,
		},
	}
	require.NoError(t, repo.Create(ctx, model, entries))
	defer func() {
		_, _ = db.NewUpdate().
			Table("users.staff_employment_profiles").
			Set("work_time_model_id = NULL").
			Where("membership_id = ?", staff.ID).
			Exec(ctx)
		_ = repo.Delete(ctx, model.ID)
	}()

	_, err := db.NewUpdate().
		Table("users.staff_employment_profiles").
		Set("work_time_model_id = ?", model.ID).
		Where("membership_id = ?", staff.ID).
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)

	first := testpkg.CreateTestStaff(t, db, "Range", "First")
	second := testpkg.CreateTestStaff(t, db, "Range", "Second")
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Range", "Foreign")

	from := adapterCalendarDate(2026, time.June, 8)
	to := from.AddDays(6)
	rows := []*configModels.StaffWorkSchedule{
		// Intersects: open-ended and valid before the range.
		{StaffID: first.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: adapterDayMonday, TargetMinutes: 240, ValidFrom: from.AddDays(-30)},
		// Intersects: window ends inside the range (valid_until exclusive).
		{StaffID: second.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: adapterDayTuesday, TargetMinutes: 300, ValidFrom: from.AddDays(-30), ValidUntil: adapterDatePointer(from.AddDays(2))},
		// Disjoint: window closed before the range starts.
		{StaffID: first.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: adapterDayWednesday, TargetMinutes: 700, ValidFrom: from.AddDays(-30), ValidUntil: adapterDatePointer(from)},
		// Disjoint: window starts after the range ends.
		{StaffID: second.ID, WeekIndex: 0, RotationLength: 1, DayOfWeek: adapterDayThursday, TargetMinutes: 700, ValidFrom: to.AddDays(1)},
	}
	for _, row := range rows {
		row.SetTenantID(testpkg.Tenant(t))
		_, err := db.NewInsert().Model(row).ModelTableExpr("config.staff_work_schedules").Exec(ctx)
		require.NoError(t, err)
	}
	foreignRow := &configModels.StaffWorkSchedule{
		StaffID: foreign.ID, WeekIndex: 0, RotationLength: 1,
		DayOfWeek: adapterDayMonday, TargetMinutes: 480, ValidFrom: from.AddDays(-30),
	}
	foreignRow.SetTenantID(foreignTenantID)
	_, err := db.NewInsert().Model(foreignRow).ModelTableExpr("config.staff_work_schedules").Exec(ctx)
	require.NoError(t, err)

	repo := adapterScheduleRepository(t, db)

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
	assert.Equal(t, first.ID, byDay[adapterDayMonday])
	assert.Equal(t, second.ID, byDay[adapterDayTuesday])
}

func TestWorkTimeModelFindByIDs_BatchesEntriesAndIsolates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	repo := adapterModelRepository(t, db)
	anchor := adapterCalendarDate(2026, time.January, 5)

	localA := &configModels.WorkTimeModel{Name: "FindByIDs A", RotationLength: 1, RotationAnchorDate: anchor}
	require.NoError(t, repo.Create(ctx, localA, []*configModels.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: adapterDayMonday, TargetMinutes: 240},
		{WeekIndex: 0, DayOfWeek: adapterDayTuesday, TargetMinutes: 300},
	}))
	localB := &configModels.WorkTimeModel{Name: "FindByIDs B", RotationLength: 2, RotationAnchorDate: anchor}
	require.NoError(t, repo.Create(ctx, localB, []*configModels.WorkTimeModelEntry{
		{WeekIndex: 1, DayOfWeek: adapterDayFriday, TargetMinutes: 180},
	}))
	foreignModel := &configModels.WorkTimeModel{Name: "FindByIDs Foreign", RotationLength: 1, RotationAnchorDate: anchor}
	require.NoError(t, repo.Create(foreignCtx, foreignModel, []*configModels.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: adapterDayMonday, TargetMinutes: 480},
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
	byID := make(map[int64]*configModels.WorkTimeModel, len(models))
	for _, model := range models {
		byID[model.ID] = model
	}
	require.Contains(t, byID, localA.ID)
	require.Contains(t, byID, localB.ID)
	assert.Len(t, byID[localA.ID].Entries, 2)
	require.Len(t, byID[localB.ID].Entries, 1)
	assert.Equal(t, 180, byID[localB.ID].Entries[0].TargetMinutes)

	// config.work_time_model_entries is batch-loaded by model id, so the
	// tenant boundary of the entries is only as good as the model filter
	// above. Assert it directly rather than by implication.
	for _, model := range models {
		for _, entry := range model.Entries {
			assert.Equal(t, model.ID, entry.ModelID,
				"an entry of a foreign tenant's model must never be attached to a visible model")
			assert.NotEqual(t, 480, entry.TargetMinutes,
				"the foreign tenant's entry must not cross the tenant boundary")
		}
	}
}
