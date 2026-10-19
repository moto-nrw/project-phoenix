package httpintegration_test

import (
	"fmt"
	"testing"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"

	"github.com/moto-nrw/project-phoenix/api"

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
	repos, err := api.NewTimetableTestRepositoriesWithMembers(testpkg.NewDatabase(t, db), api.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	shiftRows := fixtures.FromWorkforceShiftRows(repos.StaffShift)
	localCtx := testpkg.Ctx(t)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	period := fixtures.NewCalendarPeriod(func(fixtureRow *fixtures.CalendarPeriod) {
		fixtureRow.Name = fmt.Sprintf("Coverage projection %d", time.Now().UnixNano())
		fixtureRow.PeriodType = fixtures.PeriodTypeSchoolYear()
		fixtureRow.StartDate = calendar.Date(monday)
		fixtureRow.EndDate = calendar.Date(nextWednesday)
		fixtureRow.WeekCycleLength = 1
		fixtureRow.IsActive = true
	})

	period.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, fixtures.FromCalendarPeriodRecords(repos.CalendarPeriod).Create(localCtx, period))
	validFrom := activityContract.Date(friday)
	localSchedule := &activityContract.Schedule{
		Weekday: activityContract.WeekdayMonday, ActivityGroupID: localGroup.ID,
		CalendarPeriodID: &period.ID, ValidFrom: &validFrom,
	}
	localSchedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.ActivitySchedule.Create(localCtx, localSchedule))
	foreignSchedule := &activityContract.Schedule{
		Weekday: activityContract.WeekdayMonday, ActivityGroupID: foreignGroup.ID,
	}
	foreignSchedule.SetTenantID(foreignTenantID)
	require.NoError(t, repos.ActivitySchedule.Create(foreignCtx, foreignSchedule))

	for _, date := range []calendar.Date{monday, nextMonday} {
		groupID := localGroup.ID
		instance := fixtures.NewActivityInstance(func(fixtureRow *fixtures.ActivityInstance) {
			fixtureRow.Date = calendar.Date(date)
			fixtureRow.ActivityGroupID = &groupID
			fixtureRow.Title = "Coverage block"
			fixtureRow.StartTime = seriesClock(t, "09:00")
			fixtureRow.EndTime = seriesClock(t, "10:00")
			fixtureRow.RoomID = localRoom.ID
			fixtureRow.Status = fixtures.InstanceStatusPlanned()
		})

		instance.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, fixtures.FromActivityInstanceRecords(repos.ActivityInstance).Create(localCtx, instance))
		for _, assignment := range []*fixtures.InstanceStaff{fixtures.NewInstanceStaff(func(fixtureRow *fixtures.InstanceStaff) {
			fixtureRow.InstanceID = instance.ID
			fixtureRow.StaffID = localBase.ID
			fixtureRow.IsAbsent = true
		}), fixtures.NewInstanceStaff(func(fixtureRow *fixtures.InstanceStaff) {
			fixtureRow.InstanceID = instance.ID
			fixtureRow.StaffID = localSub.ID
			fixtureRow.IsSubstitute = true
		}),
		} {
			assignment.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, fixtures.FromInstanceStaffRecords(repos.InstanceStaff).Create(localCtx, assignment))
		}
		shift := fixtures.NewStaffShift(func(fixtureRow *fixtures.StaffShift) {
			fixtureRow.StaffID = localSub.ID
			fixtureRow.Date = calendar.Date(date)
			fixtureRow.StartTime = seriesClock(t, "09:00")
			fixtureRow.EndTime = seriesClock(t, "10:00")
			fixtureRow.CreatedBy = localSub.ID
		})

		shift.SetTenantID(testpkg.Tenant(t))
		require.NoError(t, shiftRows.Create(localCtx, shift))
	}
	localException := fixtures.NewActivityException(func(fixtureRow *fixtures.ActivityException) {
		fixtureRow.ActivityGroupID = localGroup.ID
		fixtureRow.ExceptionDate = calendar.Date(friday)
		fixtureRow.ExceptionType = fixtures.ActivityExceptionCancelled()
	})

	localException.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, fixtures.FromActivityExceptionRecords(repos.ActivityException).Create(localCtx, localException))

	foreignGroupID := foreignGroup.ID
	foreignInstance := fixtures.NewActivityInstance(func(fixtureRow *fixtures.ActivityInstance) {
		fixtureRow.Date = calendar.Date(monday)
		fixtureRow.ActivityGroupID = &foreignGroupID
		fixtureRow.Title = "Foreign coverage block"
		fixtureRow.StartTime = seriesClock(t, "09:00")
		fixtureRow.EndTime = seriesClock(t, "10:00")
		fixtureRow.RoomID = foreignRoom.ID
		fixtureRow.Status = fixtures.InstanceStatusPlanned()
	})

	foreignInstance.SetTenantID(foreignTenantID)
	require.NoError(t, fixtures.FromActivityInstanceRecords(repos.ActivityInstance).Create(foreignCtx, foreignInstance))
	foreignAssignment := fixtures.NewInstanceStaff(func(fixtureRow *fixtures.InstanceStaff) {
		fixtureRow.InstanceID = foreignInstance.ID
		fixtureRow.StaffID = foreignStaff.ID
		fixtureRow.IsSubstitute = true
	})
	foreignAssignment.SetTenantID(foreignTenantID)
	require.NoError(t, fixtures.FromInstanceStaffRecords(repos.InstanceStaff).Create(foreignCtx, foreignAssignment))
	foreignException := fixtures.NewActivityException(func(fixtureRow *fixtures.ActivityException) {
		fixtureRow.ActivityGroupID = foreignGroup.ID
		fixtureRow.ExceptionDate = calendar.Date(wednesday)
		fixtureRow.ExceptionType = fixtures.ActivityExceptionCancelled()
	})

	foreignException.SetTenantID(foreignTenantID)
	require.NoError(t, fixtures.FromActivityExceptionRecords(repos.ActivityException).Create(foreignCtx, foreignException))

	foreignInstances, err := fixtures.FromActivityInstanceRecords(repos.ActivityInstance).FindByActivityGroupAndDateRange(localCtx, foreignGroup.ID, calendar.Date(monday), calendar.Date(nextWednesday))
	require.NoError(t, err)
	assert.Empty(t, foreignInstances, "superuser test connection must still honor tenant filtering")
	foreignExceptions, err := fixtures.FromActivityExceptionRecords(repos.ActivityException).FindByActivityGroupAndDateRange(localCtx, foreignGroup.ID, calendar.Date(monday), calendar.Date(nextWednesday))
	require.NoError(t, err)
	assert.Empty(t, foreignExceptions, "foreign exceptions must not cross the tenant boundary")
	foreignSchedules, err := repos.ActivitySchedule.FindByGroupID(localCtx, foreignGroup.ID)
	require.NoError(t, err)
	assert.Empty(t, foreignSchedules, "foreign recurrence bounds must not cross the tenant boundary")

	queryCounter := testpkg.NewQueryCounter()
	countedDB := db.WithQueryHook(queryCounter)
	countedRepos, err := api.NewTimetableTestRepositoriesWithMembers(testpkg.NewDatabase(t, countedDB), api.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, countedDB)))
	require.NoError(t, err)
	detection := newConflictDetection(t, countedDB, countedRepos)
	pattern := 0
	coverage, err := testpkg.MeasuredTenantQuery1(t, localCtx, countedDB, queryCounter, detection.DetectShiftCoverage, timetable.ShiftCoverageProbe{
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
