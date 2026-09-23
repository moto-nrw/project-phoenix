package planning_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	planning "github.com/moto-nrw/project-phoenix/modules/workforce/internal/planning"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// overviewRepositories composes the retained readers and the Dienstplan row
// adapter over the same handle, so a query counter attached to db observes
// every read the projection makes.
func overviewRepositories(db *bun.DB) (*repositories.Factory, *workforceCompose.ShiftRows) {
	dependencies := repositories.NewUnobservedTimetableDependencies(db)
	return repositories.NewFactory(db, dependencies), workforceCompose.NewShiftRows(dependencies.Workforce)
}

func integrationClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}

type overviewTenantFixture struct {
	staffID      int64
	roomID       int64
	instanceID   int64
	assignmentID int64
	shiftID      int64
}

func createOverviewTenantFixture(
	t *testing.T,
	db *bun.DB,
	tenantID int64,
	date timezone.Date,
	withShift bool,
) overviewTenantFixture {
	t.Helper()
	testpkg.EnsureTestTenant(t, db, tenantID)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Overview", fmt.Sprintf("Tenant-%d", tenantID))
	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("Overview-%d", tenantID))
	_, shiftRows := overviewRepositories(db)
	ctx := testpkg.TenantContext(tenantID)

	instance := testpkg.CreateTestActivityInstanceForTenant(t, db, tenantID, date, room.ID, testpkg.ActivityInstanceOpts{
		Title: fmt.Sprintf("Tenant %d block", tenantID), StartHHMM: "09:00", EndHHMM: "10:00", IsSpontaneous: true,
	})
	assignment := testpkg.CreateTestInstanceStaffForTenant(t, db, tenantID, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})

	fixture := overviewTenantFixture{
		staffID: staff.ID, roomID: room.ID, instanceID: instance.ID, assignmentID: assignment.ID,
	}
	if withShift {
		shift := &planning.StaffShift{
			StaffID: staff.ID, Date: timezone.Date(date),
			StartTime: integrationClock(t, "08:00"), EndTime: integrationClock(t, "11:00"),
			CreatedBy: staff.ID,
		}
		shift.TenantID = tenantID
		require.NoError(t, shiftRows.Create(ctx, shift))
		fixture.shiftID = shift.ID
	}
	return fixture
}

func TestStaffScheduleOverview_TenantIsolationAcrossEveryProjectionRead(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	date := timezone.NewDate(2060, 1, 5)
	local := createOverviewTenantFixture(t, db, testpkg.Tenant(t), date, false)
	foreign := createOverviewTenantFixture(t, db, foreignTenantID, date, true)
	secondLocalStaff := testpkg.CreateTestStaffForTenant(t, db, testpkg.Tenant(t), "Overview", "Second-Assignment")
	testpkg.CreateTestInstanceStaffForTenant(t, db, testpkg.Tenant(t), local.instanceID, secondLocalStaff.ID, testpkg.InstanceStaffOpts{})

	queryCounter := testpkg.NewQueryCounter()
	countedDB := db.WithQueryHook(queryCounter)
	repos, shiftRows := overviewRepositories(countedDB)
	service := planning.NewStaffScheduleOverviewService(planning.StaffScheduleOverviewDependencies{
		Shifts: shiftRows, Instances: repositories.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff: repositories.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Rooms:         repos.Room, Staff: repos.Staff,
		WorkSchedules: repos.StaffWorkSchedule, WorkModels: repos.WorkTimeModel,
	})

	localOverview, err := service.GetOverview(testpkg.Ctx(t), date, date.AddDays(4))
	require.NoError(t, err)
	// Overview query count must stay fixed as assignment volume grows.
	testpkg.AssertQueryBudget(t, "services.schedule.staff_overview.week", queryCounter.Queries())
	assert.False(t, localOverview.DienstplanInUse, "foreign tenant shift must not activate the local Dienstplan")
	localAssignments := make(map[int64]planning.StaffScheduleAssignment)
	for _, assignment := range localOverview.Assignments {
		localAssignments[assignment.InstanceID] = assignment
	}
	require.Contains(t, localAssignments, local.instanceID)
	require.GreaterOrEqual(t, len(localOverview.Assignments), 2, "query-count regression must exercise multiple assignments")
	assert.NotContains(t, localAssignments, foreign.instanceID)
	assert.Equal(t, planning.CoverageStatusNotApplicable, localAssignments[local.instanceID].CoverageStatus)
	for _, member := range localOverview.Staff {
		assert.NotEqual(t, foreign.staffID, member.ID)
	}
	for _, shift := range localOverview.Shifts {
		assert.NotEqual(t, foreign.shiftID, shift.ID)
	}

	foreignOverview, err := service.GetOverview(testpkg.TenantContext(foreignTenantID), date, date.AddDays(4))
	require.NoError(t, err)
	// 6 base reads for the shift-less local tenant plus 7 for the foreign
	// tenant: a Dienstplan-active week adds exactly one batched
	// staff-work-schedule read for the weekly Soll summaries (#1837).
	// Cumulative on the same counter: 6 without shifts plus 7 with an active Dienstplan week.
	testpkg.AssertQueryBudget(t, "services.schedule.staff_overview.week_plus_dienstplan_week", queryCounter.Queries())
	assert.True(t, foreignOverview.DienstplanInUse)
	require.Len(t, foreignOverview.Assignments, 1)
	assert.Equal(t, foreign.instanceID, foreignOverview.Assignments[0].InstanceID)
	assert.Equal(t, foreign.roomID, foreignOverview.Assignments[0].RoomID)
	assert.Equal(t, planning.CoverageStatusCovered, foreignOverview.Assignments[0].CoverageStatus)
}

func insertWorkScheduleRow(t *testing.T, db *bun.DB, tenantID, staffID int64, day, targetMinutes int, validFrom timezone.Date) {
	t.Helper()
	row := &configModel.StaffWorkSchedule{
		StaffID: staffID, WeekIndex: 0, RotationLength: 1,
		DayOfWeek: day, TargetMinutes: targetMinutes,
		ValidFrom: configModel.NewCalendarDate(validFrom.Year(), validFrom.Month(), validFrom.Day()),
	}
	row.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(row).ModelTableExpr("config.staff_work_schedules").Exec(testpkg.TenantContext(tenantID))
	require.NoError(t, err)
}

func createOverviewShift(t *testing.T, db *bun.DB, tenantID, staffID int64, date timezone.Date, start, end string, breakMinutes int) int64 {
	t.Helper()
	shift := &planning.StaffShift{
		StaffID: staffID, Date: timezone.Date(date),
		StartTime: integrationClock(t, start), EndTime: integrationClock(t, end),
		BreakMinutes: breakMinutes, CreatedBy: staffID,
	}
	shift.TenantID = tenantID
	_, shiftRows := overviewRepositories(db)
	require.NoError(t, shiftRows.Create(testpkg.TenantContext(tenantID), shift))
	return shift.ID
}

func TestStaffScheduleOverview_WeeklySummariesResolveSollAndIsolateTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)

	monday := timezone.NewDate(2060, 2, 2)
	friday := monday.AddDays(4)
	validFrom := monday.AddDays(-30)

	// Fixture staff: shift 08:00-11:00 (180 min), no contractual target.
	local := createOverviewTenantFixture(t, db, tenantID, monday, true)
	foreign := createOverviewTenantFixture(t, db, foreignTenantID, monday, true)
	insertWorkScheduleRow(t, db, foreignTenantID, foreign.staffID, configModel.DayMonday, 480, validFrom)

	scheduledStaff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Summary", "Scheduled")
	createOverviewShift(t, db, tenantID, scheduledStaff.ID, monday, "08:00", "12:30", 30)
	insertWorkScheduleRow(t, db, tenantID, scheduledStaff.ID, configModel.DayMonday, 240, validFrom)
	insertWorkScheduleRow(t, db, tenantID, scheduledStaff.ID, configModel.DayTuesday, 60, validFrom)

	modelStaff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Summary", "ModelFallback")
	createOverviewShift(t, db, tenantID, modelStaff.ID, monday, "09:00", "10:00", 0)
	modelRepos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	modelRepos.SetConfigRuntime(testpkg.ConfigRuntime(db))
	modelRepo := modelRepos.WorkTimeModel
	workModel := &configModel.WorkTimeModel{
		Name:               fmt.Sprintf("Summary fallback %d", time.Now().UnixNano()),
		RotationLength:     1,
		RotationAnchorDate: configModel.NewCalendarDate(monday.Year(), monday.Month(), monday.Day()),
	}
	require.NoError(t, modelRepo.Create(testpkg.TenantContext(tenantID), workModel, []*configModel.WorkTimeModelEntry{
		{WeekIndex: 0, DayOfWeek: configModel.DayMonday, TargetMinutes: 120},
	}))
	_, err := db.NewUpdate().Table("users.staff").
		Set("work_time_model_id = ?", workModel.ID).
		Where("id = ?", modelStaff.ID).
		Exec(context.Background())
	require.NoError(t, err)

	contractedStaff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Summary", "Unplanned")
	insertWorkScheduleRow(t, db, tenantID, contractedStaff.ID, configModel.DayMonday, 480, validFrom)

	t.Cleanup(func() {
		_, _ = db.NewUpdate().Table("users.staff").
			Set("work_time_model_id = NULL").
			Where("id = ?", modelStaff.ID).
			Exec(context.Background())
		_ = modelRepo.Delete(testpkg.TenantContext(tenantID), workModel.ID)
	})

	queryCounter := testpkg.NewQueryCounter()
	countedDB := db.WithQueryHook(queryCounter)
	repos, shiftRows := overviewRepositories(countedDB)
	repos.SetConfigRuntime(testpkg.ConfigRuntime(countedDB))
	service := planning.NewStaffScheduleOverviewService(planning.StaffScheduleOverviewDependencies{
		Shifts: shiftRows, Instances: repositories.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff: repositories.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Rooms:         repos.Room, Staff: repos.Staff,
		WorkSchedules: repos.StaffWorkSchedule, WorkModels: repos.WorkTimeModel,
	})

	overview, err := service.GetOverview(testpkg.TenantContext(tenantID), monday, friday)
	require.NoError(t, err)
	// 6 base reads + 1 staff-work-schedule batch + 2 for the work-time-model
	// fallback (models + entries). The fallback fires only because one staff
	// member has a model and no schedule rows.
	testpkg.AssertQueryBudget(t, "services.schedule.staff_overview.weekly_summaries", queryCounter.Queries())

	summaries := make(map[int64]planning.StaffWeeklySummary, len(overview.WeeklySummaries))
	for _, summary := range overview.WeeklySummaries {
		assert.Equal(t, monday, summary.WeekStart)
		summaries[summary.StaffID] = summary
	}
	require.Len(t, summaries, 4, "exactly the four seeded staff must produce summary rows")

	scheduled := summaries[scheduledStaff.ID]
	assert.Equal(t, 240, scheduled.PlannedMinutes, "shift span minus break")
	require.NotNil(t, scheduled.TargetMinutes)
	assert.Equal(t, 300, *scheduled.TargetMinutes, "date-valid schedule target wins")
	require.NotNil(t, scheduled.DeltaMinutes)
	assert.Equal(t, -60, *scheduled.DeltaMinutes)

	fallback := summaries[modelStaff.ID]
	assert.Equal(t, 60, fallback.PlannedMinutes)
	require.NotNil(t, fallback.TargetMinutes)
	assert.Equal(t, 120, *fallback.TargetMinutes, "work-time model is the fallback without schedule rows")
	require.NotNil(t, fallback.DeltaMinutes)
	assert.Equal(t, -60, *fallback.DeltaMinutes)

	contracted := summaries[contractedStaff.ID]
	assert.Equal(t, 0, contracted.PlannedMinutes, "contracted staff without shifts surface as underplanned")
	require.NotNil(t, contracted.TargetMinutes)
	assert.Equal(t, 480, *contracted.TargetMinutes)
	require.NotNil(t, contracted.DeltaMinutes)
	assert.Equal(t, -480, *contracted.DeltaMinutes)

	plannedOnly := summaries[local.staffID]
	assert.Equal(t, 180, plannedOnly.PlannedMinutes)
	assert.Nil(t, plannedOnly.TargetMinutes, "no schedule and no model leaves the target unset")
	assert.Nil(t, plannedOnly.DeltaMinutes)

	assert.NotContains(t, summaries, foreign.staffID, "foreign tenant staff must not appear in summaries")

	// A week without any tenant shift yields no summaries at all (Dienstplan
	// unused), even though contractual targets would resolve.
	unusedWeek, err := service.GetOverview(testpkg.TenantContext(tenantID), monday.AddDays(14), monday.AddDays(18))
	require.NoError(t, err)
	assert.False(t, unusedWeek.DienstplanInUse)
	assert.Empty(t, unusedWeek.WeeklySummaries)

	foreignOverview, err := service.GetOverview(testpkg.TenantContext(foreignTenantID), monday, friday)
	require.NoError(t, err)
	foreignSummaries := make(map[int64]planning.StaffWeeklySummary, len(foreignOverview.WeeklySummaries))
	for _, summary := range foreignOverview.WeeklySummaries {
		foreignSummaries[summary.StaffID] = summary
	}
	require.Contains(t, foreignSummaries, foreign.staffID)
	require.NotNil(t, foreignSummaries[foreign.staffID].TargetMinutes)
	assert.Equal(t, 480, *foreignSummaries[foreign.staffID].TargetMinutes)
	assert.NotContains(t, foreignSummaries, scheduledStaff.ID, "local staff must not leak into the foreign tenant")
}

func TestStaffScheduleOverview_WeeklySummariesIncludeShiftsOutsideViewport(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	monday := timezone.NewDate(2060, 3, 1)
	friday := monday.AddDays(4)
	saturday := monday.AddDays(5)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Summary", "Weekend")
	weekdayShiftID := createOverviewShift(t, db, tenantID, staff.ID, friday, "08:00", "10:00", 0)
	weekendShiftID := createOverviewShift(t, db, tenantID, staff.ID, saturday, "09:00", "12:00", 0)

	repos, shiftRows := overviewRepositories(db)
	service := planning.NewStaffScheduleOverviewService(planning.StaffScheduleOverviewDependencies{
		Shifts: shiftRows, Instances: repositories.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff: repositories.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Rooms:         repos.Room, Staff: repos.Staff,
		WorkSchedules: repos.StaffWorkSchedule, WorkModels: repos.WorkTimeModel,
	})

	// The Dienstplan grid requests Mon-Fri, but the contractual week is
	// Mon-Sun: the Saturday shift must count toward the weekly total while
	// staying out of the viewport-scoped grid payload.
	overview, err := service.GetOverview(testpkg.TenantContext(tenantID), monday, friday)
	require.NoError(t, err)

	var summary *planning.StaffWeeklySummary
	for index := range overview.WeeklySummaries {
		if overview.WeeklySummaries[index].StaffID == staff.ID {
			summary = &overview.WeeklySummaries[index]
		}
	}
	require.NotNil(t, summary, "the staff member must produce a summary row")
	assert.Equal(t, monday, summary.WeekStart)
	assert.Equal(t, 300, summary.PlannedMinutes,
		"the Saturday shift outside the Mon-Fri viewport must count toward the weekly total")

	gridShiftIDs := make([]int64, 0, len(overview.Shifts))
	for _, shift := range overview.Shifts {
		gridShiftIDs = append(gridShiftIDs, shift.ID)
	}
	assert.Contains(t, gridShiftIDs, weekdayShiftID)
	assert.NotContains(t, gridShiftIDs, weekendShiftID, "grid shifts must stay scoped to the requested range")
}
