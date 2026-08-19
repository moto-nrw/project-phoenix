package schedule_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func integrationClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return timezone.WallClock(parsed)
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
	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(tenantID)

	instance := &scheduleModel.ActivityInstance{
		Date: date, Title: fmt.Sprintf("Tenant %d block", tenantID),
		StartTime: integrationClock(t, "09:00"), EndTime: integrationClock(t, "10:00"),
		RoomID: room.ID, Status: scheduleModel.InstanceStatusPlanned, IsSpontaneous: true,
	}
	instance.SetTenantID(tenantID)
	require.NoError(t, repos.ActivityInstance.Create(ctx, instance))
	assignment := &scheduleModel.InstanceStaff{InstanceID: instance.ID, StaffID: staff.ID}
	assignment.SetTenantID(tenantID)
	require.NoError(t, repos.InstanceStaff.Create(ctx, assignment))

	fixture := overviewTenantFixture{
		staffID: staff.ID, roomID: room.ID, instanceID: instance.ID, assignmentID: assignment.ID,
	}
	if withShift {
		shift := &scheduleModel.StaffShift{
			StaffID: staff.ID, Date: date,
			StartTime: integrationClock(t, "08:00"), EndTime: integrationClock(t, "11:00"),
			CreatedBy: staff.ID,
		}
		shift.SetTenantID(tenantID)
		require.NoError(t, repos.StaffShift.Create(ctx, shift))
		fixture.shiftID = shift.ID
	}
	return fixture
}

func cleanupOverviewTenantFixture(t *testing.T, db *bun.DB, fixture overviewTenantFixture) {
	t.Helper()
	testpkg.CleanupTableRecords(t, db, "schedule.staff_shifts", fixture.shiftID)
	testpkg.CleanupInstanceStaffFixtures(t, db, fixture.assignmentID)
	testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", fixture.instanceID)
	testpkg.CleanupStaffFixtures(t, db, fixture.staffID)
	testpkg.CleanupTableRecords(t, db, "facilities.rooms", fixture.roomID)
}

type overviewQueryCounter struct {
	count atomic.Int64
}

func (h *overviewQueryCounter) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return ctx
}

func (h *overviewQueryCounter) AfterQuery(_ context.Context, _ *bun.QueryEvent) {
	h.count.Add(1)
}

func TestStaffScheduleOverview_TenantIsolationAcrossEveryProjectionRead(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	foreignTenantID := int64(1812002)
	date := timezone.TodayDate().AddDays(12000 + int(time.Now().UnixNano()%1000))
	local := createOverviewTenantFixture(t, db, testpkg.Tenant(t), date, false)
	foreign := createOverviewTenantFixture(t, db, foreignTenantID, date, true)
	secondLocalStaff := testpkg.CreateTestStaffForTenant(t, db, testpkg.Tenant(t), "Overview", "Second-Assignment")
	secondLocalAssignment := &scheduleModel.InstanceStaff{InstanceID: local.instanceID, StaffID: secondLocalStaff.ID}
	secondLocalAssignment.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(db).InstanceStaff.Create(testpkg.Ctx(t), secondLocalAssignment))
	t.Cleanup(func() {
		testpkg.CleanupInstanceStaffFixtures(t, db, secondLocalAssignment.ID)
		testpkg.CleanupStaffFixtures(t, db, secondLocalStaff.ID)
		cleanupOverviewTenantFixture(t, db, foreign)
		cleanupOverviewTenantFixture(t, db, local)
	})

	queryCounter := &overviewQueryCounter{}
	countedDB := db.WithQueryHook(queryCounter)
	repos := repositories.NewFactory(countedDB)
	service := scheduleSvc.NewStaffScheduleOverviewService(scheduleSvc.StaffScheduleOverviewDependencies{
		Shifts: repos.StaffShift, Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff,
		Rooms: repos.Room, Staff: repos.Staff,
		WorkSchedules: repos.StaffWorkSchedule, WorkModels: repos.WorkTimeModel,
	})

	localOverview, err := service.GetOverview(testpkg.Ctx(t), date, date.AddDays(4))
	require.NoError(t, err)
	assert.Equal(t, int64(6), queryCounter.count.Load(), "overview query count must stay fixed as assignment volume grows")
	assert.False(t, localOverview.DienstplanInUse, "foreign tenant shift must not activate the local Dienstplan")
	localAssignments := make(map[int64]scheduleSvc.StaffScheduleAssignment)
	for _, assignment := range localOverview.Assignments {
		localAssignments[assignment.InstanceID] = assignment
	}
	require.Contains(t, localAssignments, local.instanceID)
	require.GreaterOrEqual(t, len(localOverview.Assignments), 2, "query-count regression must exercise multiple assignments")
	assert.NotContains(t, localAssignments, foreign.instanceID)
	assert.Equal(t, scheduleSvc.CoverageStatusNotApplicable, localAssignments[local.instanceID].CoverageStatus)
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
	assert.Equal(t, int64(13), queryCounter.count.Load(), "overview reads must stay fixed batches: 6 without shifts, 7 with an active Dienstplan week")
	assert.True(t, foreignOverview.DienstplanInUse)
	require.Len(t, foreignOverview.Assignments, 1)
	assert.Equal(t, foreign.instanceID, foreignOverview.Assignments[0].InstanceID)
	assert.Equal(t, foreign.roomID, foreignOverview.Assignments[0].RoomID)
	assert.Equal(t, scheduleSvc.CoverageStatusCovered, foreignOverview.Assignments[0].CoverageStatus)
}

func cleanupWorkSchedulesForStaff(t *testing.T, db *bun.DB, staffIDs ...int64) {
	t.Helper()
	if len(staffIDs) == 0 {
		return
	}
	_, err := db.NewDelete().
		Table("config.staff_work_schedules").
		Where("staff_id IN (?)", bun.List(staffIDs)).
		Exec(context.Background())
	require.NoError(t, err)
}

func insertWorkScheduleRow(t *testing.T, db *bun.DB, tenantID, staffID int64, day, targetMinutes int, validFrom timezone.Date) {
	t.Helper()
	row := &configModel.StaffWorkSchedule{
		StaffID: staffID, WeekIndex: 0, RotationLength: 1,
		DayOfWeek: day, TargetMinutes: targetMinutes, ValidFrom: validFrom,
	}
	row.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(row).ModelTableExpr("config.staff_work_schedules").Exec(testpkg.TenantContext(tenantID))
	require.NoError(t, err)
}

func createOverviewShift(t *testing.T, db *bun.DB, tenantID, staffID int64, date timezone.Date, start, end string, breakMinutes int) int64 {
	t.Helper()
	shift := &scheduleModel.StaffShift{
		StaffID: staffID, Date: date,
		StartTime: integrationClock(t, start), EndTime: integrationClock(t, end),
		BreakMinutes: breakMinutes, CreatedBy: staffID,
	}
	shift.SetTenantID(tenantID)
	require.NoError(t, repositories.NewFactory(db).StaffShift.Create(testpkg.TenantContext(tenantID), shift))
	return shift.ID
}

func TestStaffScheduleOverview_WeeklySummariesResolveSollAndIsolateTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	tenantID := int64(1837003)
	foreignTenantID := int64(1837004)

	monday := timezone.TodayDate().AddDays(14000 + int(time.Now().UnixNano()%1000))
	for monday.Weekday() != time.Monday {
		monday = monday.AddDays(1)
	}
	friday := monday.AddDays(4)
	validFrom := monday.AddDays(-30)

	// Fixture staff: shift 08:00-11:00 (180 min), no contractual target.
	local := createOverviewTenantFixture(t, db, tenantID, monday, true)
	foreign := createOverviewTenantFixture(t, db, foreignTenantID, monday, true)
	insertWorkScheduleRow(t, db, foreignTenantID, foreign.staffID, configModel.DayMonday, 480, validFrom)

	scheduledStaff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Summary", "Scheduled")
	scheduledShiftID := createOverviewShift(t, db, tenantID, scheduledStaff.ID, monday, "08:00", "12:30", 30)
	insertWorkScheduleRow(t, db, tenantID, scheduledStaff.ID, configModel.DayMonday, 240, validFrom)
	insertWorkScheduleRow(t, db, tenantID, scheduledStaff.ID, configModel.DayTuesday, 60, validFrom)

	modelStaff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Summary", "ModelFallback")
	modelShiftID := createOverviewShift(t, db, tenantID, modelStaff.ID, monday, "09:00", "10:00", 0)
	modelRepo := repositories.NewFactory(db).WorkTimeModel
	workModel := &configModel.WorkTimeModel{
		Name:               fmt.Sprintf("Summary fallback %d", time.Now().UnixNano()),
		RotationLength:     1,
		RotationAnchorDate: monday,
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
		testpkg.CleanupTableRecords(t, db, "schedule.staff_shifts", scheduledShiftID, modelShiftID)
		cleanupWorkSchedulesForStaff(t, db, scheduledStaff.ID, contractedStaff.ID, foreign.staffID)
		_, _ = db.NewUpdate().Table("users.staff").
			Set("work_time_model_id = NULL").
			Where("id = ?", modelStaff.ID).
			Exec(context.Background())
		_ = modelRepo.Delete(testpkg.TenantContext(tenantID), workModel.ID)
		testpkg.CleanupStaffFixtures(t, db, scheduledStaff.ID, modelStaff.ID, contractedStaff.ID)
		cleanupOverviewTenantFixture(t, db, foreign)
		cleanupOverviewTenantFixture(t, db, local)
	})

	queryCounter := &overviewQueryCounter{}
	repos := repositories.NewFactory(db.WithQueryHook(queryCounter))
	service := scheduleSvc.NewStaffScheduleOverviewService(scheduleSvc.StaffScheduleOverviewDependencies{
		Shifts: repos.StaffShift, Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff,
		Rooms: repos.Room, Staff: repos.Staff,
		WorkSchedules: repos.StaffWorkSchedule, WorkModels: repos.WorkTimeModel,
	})

	overview, err := service.GetOverview(testpkg.TenantContext(tenantID), monday, friday)
	require.NoError(t, err)
	// 6 base reads + 1 staff-work-schedule batch + 2 for the work-time-model
	// fallback (models + entries). The fallback fires only because one staff
	// member has a model and no schedule rows.
	assert.Equal(t, int64(9), queryCounter.count.Load(), "summary reads must stay fixed batches regardless of staff volume")

	summaries := make(map[int64]scheduleSvc.StaffWeeklySummary, len(overview.WeeklySummaries))
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
	foreignSummaries := make(map[int64]scheduleSvc.StaffWeeklySummary, len(foreignOverview.WeeklySummaries))
	for _, summary := range foreignOverview.WeeklySummaries {
		foreignSummaries[summary.StaffID] = summary
	}
	require.Contains(t, foreignSummaries, foreign.staffID)
	require.NotNil(t, foreignSummaries[foreign.staffID].TargetMinutes)
	assert.Equal(t, 480, *foreignSummaries[foreign.staffID].TargetMinutes)
	assert.NotContains(t, foreignSummaries, scheduledStaff.ID, "local staff must not leak into the foreign tenant")
}

func TestStaffScheduleOverview_WeeklySummariesIncludeShiftsOutsideViewport(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	tenantID := int64(1882001)
	testpkg.EnsureTestTenant(t, db, tenantID)

	monday := timezone.TodayDate().AddDays(15000 + int(time.Now().UnixNano()%1000))
	for monday.Weekday() != time.Monday {
		monday = monday.AddDays(1)
	}
	friday := monday.AddDays(4)
	saturday := monday.AddDays(5)

	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Summary", "Weekend")
	weekdayShiftID := createOverviewShift(t, db, tenantID, staff.ID, friday, "08:00", "10:00", 0)
	weekendShiftID := createOverviewShift(t, db, tenantID, staff.ID, saturday, "09:00", "12:00", 0)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.staff_shifts", weekdayShiftID, weekendShiftID)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
	})

	repos := repositories.NewFactory(db)
	service := scheduleSvc.NewStaffScheduleOverviewService(scheduleSvc.StaffScheduleOverviewDependencies{
		Shifts: repos.StaffShift, Instances: repos.ActivityInstance, InstanceStaff: repos.InstanceStaff,
		Rooms: repos.Room, Staff: repos.Staff,
		WorkSchedules: repos.StaffWorkSchedule, WorkModels: repos.WorkTimeModel,
	})

	// The Dienstplan grid requests Mon-Fri, but the contractual week is
	// Mon-Sun: the Saturday shift must count toward the weekly total while
	// staying out of the viewport-scoped grid payload.
	overview, err := service.GetOverview(testpkg.TenantContext(tenantID), monday, friday)
	require.NoError(t, err)

	var summary *scheduleSvc.StaffWeeklySummary
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

func TestShiftCoverageProjection_BatchesEffectiveSeriesReadsAndIsolatesTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	foreignTenantID := int64(1812003)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	monday := timezone.TodayDate().AddDays(13000 + int(time.Now().UnixNano()%1000))
	// Keep the fixture anchored to Monday so containing-calendar-week activation is
	// deterministic regardless of the randomized far-future offset.
	for monday.Weekday() != time.Monday {
		monday = monday.AddDays(1)
	}
	wednesday := monday.AddDays(2)
	friday := monday.AddDays(4)
	nextMonday := monday.AddDays(7)
	nextWednesday := monday.AddDays(9)

	localGroup := testpkg.CreateTestActivityGroupForTenant(t, db, testpkg.Tenant(t), "Coverage-Local")
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Coverage-Foreign")
	localBase := testpkg.CreateTestStaffForTenant(t, db, testpkg.Tenant(t), "Coverage", "Base")
	localSub := testpkg.CreateTestStaffForTenant(t, db, testpkg.Tenant(t), "Coverage", "Substitute")
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Coverage", "Foreign")
	localRoom := testpkg.CreateTestRoomForTenant(t, db, testpkg.Tenant(t), "Coverage-Local")
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreignTenantID, "Coverage-Foreign")
	repos := repositories.NewFactory(db)
	localCtx := testpkg.Ctx(t)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	period := &scheduleModel.CalendarPeriod{
		Name:       fmt.Sprintf("Coverage projection %d", time.Now().UnixNano()),
		PeriodType: scheduleModel.PeriodTypeSchoolYear,
		StartDate:  monday, EndDate: nextWednesday, WeekCycleLength: 1, IsActive: true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.CalendarPeriod.Create(localCtx, period))
	validFrom := friday
	localSchedule := &activitiesModel.Schedule{
		Weekday: activitiesModel.WeekdayMonday, ActivityGroupID: localGroup.ID,
		CalendarPeriodID: &period.ID, ValidFrom: &validFrom,
	}
	localSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(localCtx, localSchedule))
	foreignSchedule := &activitiesModel.Schedule{
		Weekday: activitiesModel.WeekdayMonday, ActivityGroupID: foreignGroup.ID,
	}
	foreignSchedule.SetTenantID(foreignTenantID)
	require.NoError(t, repos.ActivitySchedule.Create(foreignCtx, foreignSchedule))

	localInstanceIDs := make([]int64, 0, 2)
	localAssignmentIDs := make([]int64, 0, 4)
	localShiftIDs := make([]int64, 0, 2)
	for _, date := range []timezone.Date{monday, nextMonday} {
		groupID := localGroup.ID
		instance := &scheduleModel.ActivityInstance{
			Date: date, ActivityGroupID: &groupID, Title: "Coverage block",
			StartTime: integrationClock(t, "09:00"), EndTime: integrationClock(t, "10:00"),
			RoomID: localRoom.ID, Status: scheduleModel.InstanceStatusPlanned,
		}
		instance.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repos.ActivityInstance.Create(localCtx, instance))
		localInstanceIDs = append(localInstanceIDs, instance.ID)
		for _, assignment := range []*scheduleModel.InstanceStaff{
			{InstanceID: instance.ID, StaffID: localBase.ID, IsAbsent: true},
			{InstanceID: instance.ID, StaffID: localSub.ID, IsSubstitute: true},
		} {
			assignment.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, repos.InstanceStaff.Create(localCtx, assignment))
			localAssignmentIDs = append(localAssignmentIDs, assignment.ID)
		}
		shift := &scheduleModel.StaffShift{
			StaffID: localSub.ID, Date: date,
			StartTime: integrationClock(t, "09:00"), EndTime: integrationClock(t, "10:00"),
			CreatedBy: localSub.ID,
		}
		shift.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repos.StaffShift.Create(localCtx, shift))
		localShiftIDs = append(localShiftIDs, shift.ID)
	}
	localException := &scheduleModel.ActivityException{
		ActivityGroupID: localGroup.ID, ExceptionDate: friday,
		ExceptionType: scheduleModel.ActivityExceptionCancelled,
	}
	localException.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivityException.Create(localCtx, localException))

	foreignGroupID := foreignGroup.ID
	foreignInstance := &scheduleModel.ActivityInstance{
		Date: monday, ActivityGroupID: &foreignGroupID, Title: "Foreign coverage block",
		StartTime: integrationClock(t, "09:00"), EndTime: integrationClock(t, "10:00"),
		RoomID: foreignRoom.ID, Status: scheduleModel.InstanceStatusPlanned,
	}
	foreignInstance.SetTenantID(foreignTenantID)
	require.NoError(t, repos.ActivityInstance.Create(foreignCtx, foreignInstance))
	foreignAssignment := &scheduleModel.InstanceStaff{InstanceID: foreignInstance.ID, StaffID: foreignStaff.ID, IsSubstitute: true}
	foreignAssignment.SetTenantID(foreignTenantID)
	require.NoError(t, repos.InstanceStaff.Create(foreignCtx, foreignAssignment))
	foreignException := &scheduleModel.ActivityException{
		ActivityGroupID: foreignGroup.ID, ExceptionDate: wednesday,
		ExceptionType: scheduleModel.ActivityExceptionCancelled,
	}
	foreignException.SetTenantID(foreignTenantID)
	require.NoError(t, repos.ActivityException.Create(foreignCtx, foreignException))

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.staff_shifts", localShiftIDs...)
		testpkg.CleanupInstanceStaffFixtures(t, db, append(localAssignmentIDs, foreignAssignment.ID)...)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_exceptions", localException.ID, foreignException.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", append(localInstanceIDs, foreignInstance.ID)...)
		testpkg.CleanupTableRecords(t, db, "activities.schedules", localSchedule.ID, foreignSchedule.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.calendar_periods", period.ID)
		testpkg.CleanupTableRecords(t, db, "activities.groups", localGroup.ID, foreignGroup.ID)
		testpkg.CleanupTableRecords(t, db, "activities.categories", localGroup.CategoryID, foreignGroup.CategoryID)
		testpkg.CleanupStaffFixtures(t, db, localBase.ID, localSub.ID, foreignStaff.ID, *localGroup.CreatedBy, *foreignGroup.CreatedBy)
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", localRoom.ID, foreignRoom.ID)
	})

	instanceReader, ok := repos.ActivityInstance.(scheduleSvc.ActivityGroupInstanceRangeReader)
	require.True(t, ok)
	exceptionReader, ok := repos.ActivityException.(scheduleSvc.ActivityExceptionRangeReader)
	require.True(t, ok)
	foreignInstances, err := instanceReader.FindByActivityGroupAndDateRange(localCtx, foreignGroup.ID, monday, nextWednesday)
	require.NoError(t, err)
	assert.Empty(t, foreignInstances, "superuser test connection must still honor tenant filtering")
	foreignExceptions, err := exceptionReader.FindByActivityGroupAndDateRange(localCtx, foreignGroup.ID, monday, nextWednesday)
	require.NoError(t, err)
	assert.Empty(t, foreignExceptions, "foreign exceptions must not cross the tenant boundary")
	scheduleReader, ok := repos.ActivitySchedule.(scheduleSvc.ActivityScheduleGroupReader)
	require.True(t, ok)
	foreignSchedules, err := scheduleReader.FindByGroupID(localCtx, foreignGroup.ID)
	require.NoError(t, err)
	assert.Empty(t, foreignSchedules, "foreign recurrence bounds must not cross the tenant boundary")

	queryCounter := &overviewQueryCounter{}
	countedRepos := repositories.NewFactory(db.WithQueryHook(queryCounter))
	countedInstances, ok := countedRepos.ActivityInstance.(scheduleSvc.ActivityGroupInstanceRangeReader)
	require.True(t, ok)
	countedExceptions, ok := countedRepos.ActivityException.(scheduleSvc.ActivityExceptionRangeReader)
	require.True(t, ok)
	pattern := 0
	warnings, err := scheduleSvc.DetectShiftCoverageWarnings(localCtx, scheduleSvc.ShiftCoverageDependencies{
		Shifts: countedRepos.StaffShift, Instances: countedInstances, Exceptions: countedExceptions,
		Schedules:     countedRepos.ActivitySchedule,
		InstanceStaff: countedRepos.InstanceStaff, Staff: countedRepos.Staff,
		CalendarPeriods: countedRepos.CalendarPeriod,
	}, scheduleSvc.ShiftCoverageQuery{
		Dates:     []timezone.Date{monday, wednesday, friday, nextMonday, nextWednesday},
		StartTime: integrationClock(t, "09:00"), EndTime: integrationClock(t, "10:00"),
		StaffIDs: []int64{localBase.ID}, ReplanActivityGroupID: &localGroup.ID,
		CalendarPeriodID: &period.ID, WeekPattern: &pattern,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(8), queryCounter.count.Load(), "series volume must use eight fixed batch reads")
	require.Len(t, warnings, 1, "pre-valid_from candidates must not be projected")
	assert.Equal(t, nextWednesday.String(), warnings[0].Date)
	for _, warning := range warnings {
		assert.Equal(t, localBase.ID, warning.StaffID)
		assert.NotEqual(t, foreignStaff.ID, warning.StaffID)
	}
}
