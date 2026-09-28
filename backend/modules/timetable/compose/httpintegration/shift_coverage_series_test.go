package httpintegration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shift-coverage probe of a series edit (ReplanWeek) reads a fixed number
// of batched rows and never crosses the tenant boundary. Moved with the probe
// from the Workforce overview suite (#3550).

func seriesClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return calendar.NormalizeWallClock(parsed)
}

func TestShiftCoverageProjection_BatchesEffectiveSeriesReadsAndIsolatesTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	monday := calendar.NewDate(2060, 4, 5)
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
	repos, err := repositories.NewTimetableTestRepositories(db)
	require.NoError(t, err)
	shiftRows := repos.StaffShift
	localCtx := testpkg.Ctx(t)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	period := &scheduleModel.CalendarPeriod{
		Name:       fmt.Sprintf("Coverage projection %d", time.Now().UnixNano()),
		PeriodType: scheduleModel.PeriodTypeSchoolYear,
		StartDate:  scheduleModel.Date(monday), EndDate: scheduleModel.Date(nextWednesday), WeekCycleLength: 1, IsActive: true,
	}
	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.CalendarPeriod.Create(localCtx, period))
	validFrom := activitiesModel.Date(friday)
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

	for _, date := range []calendar.Date{monday, nextMonday} {
		groupID := localGroup.ID
		instance := &scheduleModel.ActivityInstance{
			Date: scheduleModel.Date(date), ActivityGroupID: &groupID, Title: "Coverage block",
			StartTime: seriesClock(t, "09:00"), EndTime: seriesClock(t, "10:00"),
			RoomID: localRoom.ID, Status: scheduleModel.InstanceStatusPlanned,
		}
		instance.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, repos.ActivityInstance.Create(localCtx, instance))
		for _, assignment := range []*scheduleModel.InstanceStaff{
			{InstanceID: instance.ID, StaffID: localBase.ID, IsAbsent: true},
			{InstanceID: instance.ID, StaffID: localSub.ID, IsSubstitute: true},
		} {
			assignment.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, repos.InstanceStaff.Create(localCtx, assignment))
		}
		shift := &scheduleModel.StaffShift{
			StaffID: localSub.ID, Date: scheduleModel.Date(date),
			StartTime: seriesClock(t, "09:00"), EndTime: seriesClock(t, "10:00"),
			CreatedBy: localSub.ID,
		}
		shift.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, shiftRows.Create(localCtx, shift))
	}
	localException := &scheduleModel.ActivityException{
		ActivityGroupID: localGroup.ID, ExceptionDate: scheduleModel.Date(friday),
		ExceptionType: scheduleModel.ActivityExceptionCancelled,
	}
	localException.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivityException.Create(localCtx, localException))

	foreignGroupID := foreignGroup.ID
	foreignInstance := &scheduleModel.ActivityInstance{
		Date: scheduleModel.Date(monday), ActivityGroupID: &foreignGroupID, Title: "Foreign coverage block",
		StartTime: seriesClock(t, "09:00"), EndTime: seriesClock(t, "10:00"),
		RoomID: foreignRoom.ID, Status: scheduleModel.InstanceStatusPlanned,
	}
	foreignInstance.SetTenantID(foreignTenantID)
	require.NoError(t, repos.ActivityInstance.Create(foreignCtx, foreignInstance))
	foreignAssignment := &scheduleModel.InstanceStaff{InstanceID: foreignInstance.ID, StaffID: foreignStaff.ID, IsSubstitute: true}
	foreignAssignment.SetTenantID(foreignTenantID)
	require.NoError(t, repos.InstanceStaff.Create(foreignCtx, foreignAssignment))
	foreignException := &scheduleModel.ActivityException{
		ActivityGroupID: foreignGroup.ID, ExceptionDate: scheduleModel.Date(wednesday),
		ExceptionType: scheduleModel.ActivityExceptionCancelled,
	}
	foreignException.SetTenantID(foreignTenantID)
	require.NoError(t, repos.ActivityException.Create(foreignCtx, foreignException))

	foreignInstances, err := repos.ActivityInstance.FindByActivityGroupAndDateRange(localCtx, foreignGroup.ID, scheduleModel.Date(monday), scheduleModel.Date(nextWednesday))
	require.NoError(t, err)
	assert.Empty(t, foreignInstances, "superuser test connection must still honor tenant filtering")
	foreignExceptions, err := repos.ActivityException.FindByActivityGroupAndDateRange(localCtx, foreignGroup.ID, scheduleModel.Date(monday), scheduleModel.Date(nextWednesday))
	require.NoError(t, err)
	assert.Empty(t, foreignExceptions, "foreign exceptions must not cross the tenant boundary")
	foreignSchedules, err := repos.ActivitySchedule.FindByGroupID(localCtx, foreignGroup.ID)
	require.NoError(t, err)
	assert.Empty(t, foreignSchedules, "foreign recurrence bounds must not cross the tenant boundary")

	queryCounter := testpkg.NewQueryCounter()
	countedRepos, err := repositories.NewTimetableTestRepositories(db.WithQueryHook(queryCounter))
	require.NoError(t, err)
	detection := newConflictDetection(t, countedRepos)
	pattern := 0
	coverage, err := detection.DetectShiftCoverage(localCtx, timetable.ShiftCoverageProbe{
		Dates:     []calendar.Date{monday, wednesday, friday, nextMonday, nextWednesday},
		StartTime: seriesClock(t, "09:00"), EndTime: seriesClock(t, "10:00"),
		StaffIDs: []int64{localBase.ID}, ReplanActivityGroupID: &localGroup.ID,
		CalendarPeriodID: &period.ID, WeekPattern: &pattern,
	})
	require.NoError(t, err)
	warnings := coverage.Warnings
	// Series volume must use eight fixed batch reads.
	testpkg.AssertQueryBudget(t, "services.schedule.shift_coverage.series", queryCounter.Queries())
	require.Len(t, warnings, 1, "pre-valid_from candidates must not be projected")
	assert.Equal(t, nextWednesday.String(), warnings[0].Date)
	for _, warning := range warnings {
		assert.Equal(t, localBase.ID, warning.StaffID)
		assert.NotEqual(t, foreignStaff.ID, warning.StaffID)
	}
}
