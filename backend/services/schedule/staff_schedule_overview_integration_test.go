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
	t.Cleanup(func() { _ = db.Close() })
	foreignTenantID := int64(1812002)
	date := timezone.TodayDate().AddDays(12000 + int(time.Now().UnixNano()%1000))
	local := createOverviewTenantFixture(t, db, 1, date, false)
	foreign := createOverviewTenantFixture(t, db, foreignTenantID, date, true)
	secondLocalStaff := testpkg.CreateTestStaffForTenant(t, db, 1, "Overview", "Second-Assignment")
	secondLocalAssignment := &scheduleModel.InstanceStaff{InstanceID: local.instanceID, StaffID: secondLocalStaff.ID}
	secondLocalAssignment.SetTenantID(1)
	require.NoError(t, repositories.NewFactory(db).InstanceStaff.Create(testpkg.TenantContext(1), secondLocalAssignment))
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
	})

	localOverview, err := service.GetOverview(testpkg.TenantContext(1), date, date.AddDays(4))
	require.NoError(t, err)
	assert.Equal(t, int64(5), queryCounter.count.Load(), "overview query count must stay fixed as assignment volume grows")
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
	assert.Equal(t, int64(10), queryCounter.count.Load(), "each overview request must perform exactly five batched reads")
	assert.True(t, foreignOverview.DienstplanInUse)
	require.Len(t, foreignOverview.Assignments, 1)
	assert.Equal(t, foreign.instanceID, foreignOverview.Assignments[0].InstanceID)
	assert.Equal(t, foreign.roomID, foreignOverview.Assignments[0].RoomID)
	assert.Equal(t, scheduleSvc.CoverageStatusCovered, foreignOverview.Assignments[0].CoverageStatus)
}

func TestShiftCoverageProjection_BatchesEffectiveSeriesReadsAndIsolatesTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
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

	localGroup := testpkg.CreateTestActivityGroupForTenant(t, db, 1, "Coverage-Local")
	foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreignTenantID, "Coverage-Foreign")
	localBase := testpkg.CreateTestStaffForTenant(t, db, 1, "Coverage", "Base")
	localSub := testpkg.CreateTestStaffForTenant(t, db, 1, "Coverage", "Substitute")
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Coverage", "Foreign")
	localRoom := testpkg.CreateTestRoomForTenant(t, db, 1, "Coverage-Local")
	foreignRoom := testpkg.CreateTestRoomForTenant(t, db, foreignTenantID, "Coverage-Foreign")
	repos := repositories.NewFactory(db)
	localCtx := testpkg.TenantContext(1)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	period := &scheduleModel.CalendarPeriod{
		Name:       fmt.Sprintf("Coverage projection %d", time.Now().UnixNano()),
		PeriodType: scheduleModel.PeriodTypeSchoolYear,
		StartDate:  monday, EndDate: nextWednesday, WeekCycleLength: 1, IsActive: true,
	}
	period.SetTenantID(1)
	require.NoError(t, repos.CalendarPeriod.Create(localCtx, period))
	validFrom := friday
	localSchedule := &activitiesModel.Schedule{
		Weekday: activitiesModel.WeekdayMonday, ActivityGroupID: localGroup.ID,
		CalendarPeriodID: &period.ID, ValidFrom: &validFrom,
	}
	localSchedule.SetTenantID(1)
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
		instance.SetTenantID(1)
		require.NoError(t, repos.ActivityInstance.Create(localCtx, instance))
		localInstanceIDs = append(localInstanceIDs, instance.ID)
		for _, assignment := range []*scheduleModel.InstanceStaff{
			{InstanceID: instance.ID, StaffID: localBase.ID, IsAbsent: true},
			{InstanceID: instance.ID, StaffID: localSub.ID, IsSubstitute: true},
		} {
			assignment.SetTenantID(1)
			require.NoError(t, repos.InstanceStaff.Create(localCtx, assignment))
			localAssignmentIDs = append(localAssignmentIDs, assignment.ID)
		}
		shift := &scheduleModel.StaffShift{
			StaffID: localSub.ID, Date: date,
			StartTime: integrationClock(t, "09:00"), EndTime: integrationClock(t, "10:00"),
			CreatedBy: localSub.ID,
		}
		shift.SetTenantID(1)
		require.NoError(t, repos.StaffShift.Create(localCtx, shift))
		localShiftIDs = append(localShiftIDs, shift.ID)
	}
	localException := &scheduleModel.ActivityException{
		ActivityGroupID: localGroup.ID, ExceptionDate: friday,
		ExceptionType: scheduleModel.ActivityExceptionCancelled,
	}
	localException.SetTenantID(1)
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
	assert.Equal(t, int64(7), queryCounter.count.Load(), "series volume must use seven fixed batch reads")
	require.Len(t, warnings, 1, "pre-valid_from candidates must not be projected")
	assert.Equal(t, nextWednesday.String(), warnings[0].Date)
	for _, warning := range warnings {
		assert.Equal(t, localBase.ID, warning.StaffID)
		assert.NotEqual(t, foreignStaff.ID, warning.StaffID)
	}
}
