package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The access log of a home school read names each week once and counts each
// child once, also when a child is booked in two weeks (#6679).
func TestHomeSchoolAccessCountsChildrenNotBookings(t *testing.T) {
	t.Parallel()
	week := func(child, week int64, monday calendar.Date) enrollment.HolidayHomeSchoolBooking {
		return enrollment.HolidayHomeSchoolBooking{ChildID: child, WeekID: week, StartsOn: monday, EndsOn: monday.AddDays(4)}
	}
	first, second := calendar.NewDate(2099, 7, 6), calendar.NewDate(2099, 7, 13)
	access := homeSchoolAccess(enrollment.HolidayHost{AccountID: 7, Role: "lead"}, []enrollment.HolidayHomeSchoolBooking{
		week(1, 10, first), week(1, 11, second), week(2, 10, first),
	})
	assert.Equal(t, enrollment.HolidayGuestAccessHomeSchool, access.Kind)
	assert.Equal(t, 2, access.Guests, "Tom in two weeks is one child")
	assert.Equal(t, []int64{10, 11}, access.WeekIDs)
	assert.Equal(t, int64(10), access.WeekID)
	assert.Equal(t, first, access.StartsOn)
	assert.Equal(t, second.AddDays(4), access.EndsOn)
}
