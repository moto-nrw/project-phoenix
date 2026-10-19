package contracttest_test

import (
	"testing"
	"time"

	timetable "github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// #3946: the Kinderkarte marks a one-day change of the Gehzeit or the
// Abholart, so the day read carries both.
func TestPickupScheduleService_GetBulkEffectivePickupDaysForDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupPickupScheduleService(t, db)
	ctx := testpkg.Ctx(t)
	staffID := createPickupServiceTestStaffID(t, db)

	// January 11, 2024 is a Thursday.
	testDate := calendar.NewDate(2024, 1, 11)
	usual := testpkg.CreateTestStudent(t, db, "Pickup", "Usual", "1a")
	changed := testpkg.CreateTestStudent(t, db, "Pickup", "Changed", "1a")
	for _, studentID := range []int64{usual.ID, changed.ID} {
		require.NoError(t, testpkg.TenantCommand1(t, ctx, db, service.UpsertStudentPickupSchedule, &careplan.PickupSchedule{
			StudentID:  studentID,
			Weekday:    timetable.WeekdayThursday,
			PickupTime: time.Date(2024, 1, 1, 15, 0, 0, 0, time.UTC),
			CreatedBy:  staffID,
		}))
	}
	_, err := testpkg.TenantQuery5(t, ctx, db, service.SetDayDeparture, changed.ID, testDate, careplan.DepartureModePickup, nil, staffID)
	require.NoError(t, err)

	results, err := testpkg.TenantQuery2(t, ctx, db, service.GetBulkEffectivePickupDaysForDate, []int64{usual.ID, changed.ID}, testDate)
	require.NoError(t, err)

	require.Contains(t, results, usual.ID)
	require.Contains(t, results, changed.ID)
	assert.Empty(t, results[usual.ID].DayDepartureMode, "the weekly plan marks nothing")
	assert.Equal(t, careplan.DepartureModePickup, results[changed.ID].DayDepartureMode)
	assert.False(t, results[changed.ID].IsException, "the Gehzeit itself stays the weekly plan's")
	require.NotNil(t, results[changed.ID].PickupTime)
	assert.Equal(t, 15, results[changed.ID].PickupTime.Hour())

	otherDay, err := testpkg.TenantQuery2(t, ctx, db, service.GetBulkEffectivePickupDaysForDate, []int64{changed.ID}, testDate.AddDays(7))
	require.NoError(t, err)
	require.Contains(t, otherDay, changed.ID)
	assert.Empty(t, otherDay[changed.ID].DayDepartureMode, "the Abholart holds for its one day only")
}
