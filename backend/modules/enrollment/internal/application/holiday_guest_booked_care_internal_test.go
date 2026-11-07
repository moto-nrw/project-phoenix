package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type guestBookingsStub []enrollment.HolidayGuest

func (s guestBookingsStub) ListGuestBookings(context.Context, []int64, calendar.Date, calendar.Date) ([]enrollment.HolidayGuest, error) {
	return s, nil
}

func bookedDates(days map[calendar.Date]enrollment.HolidayGuestBookedDay) []calendar.Date {
	var dates []calendar.Date
	for date := calendar.NewDate(2026, 6, 1); date.Before(calendar.NewDate(2026, 8, 1)); date = date.AddDays(1) {
		if _, ok := days[date]; ok {
			dates = append(dates, date)
		}
	}
	return dates
}

// The booked days of a guest are calendar days of its week within the
// window; a cancellation ends them on the day it was recorded (#5994).
func TestHolidayGuestBookedDaysFollowTheWeekTheWindowAndTheCancellation(t *testing.T) {
	t.Parallel()
	mon, fri := calendar.NewDate(2026, 7, 6), calendar.NewDate(2026, 7, 10)
	wed := mon.AddDays(2)
	before := mon.AddDays(-3)
	care := NewHolidayGuestBookedCare(guestBookingsStub{
		{StudentID: 1, StartsOn: mon, EndsOn: fri, TimeModelStarts: "08:00", TimeModelEnds: "16:00"},
		{StudentID: 2, StartsOn: mon, EndsOn: fri, EndedOn: &wed, TimeModelStarts: "08:00", TimeModelEnds: "13:00"},
		{StudentID: 3, StartsOn: mon, EndsOn: fri, EndedOn: &before, TimeModelStarts: "08:00", TimeModelEnds: "13:00"},
	})

	days, err := care.HolidayGuestBookedDays(context.Background(), []int64{1, 2, 3}, mon.AddDays(1), calendar.NewDate(2026, 7, 31))
	require.NoError(t, err)
	assert.Equal(t, []calendar.Date{mon.AddDays(1), wed, mon.AddDays(3), fri}, bookedDates(days[1]),
		"from the window's first day to the last day of the week")
	assert.Equal(t, "16:00", days[1][fri].EndsAt.Time().Format("15:04"))
	assert.Equal(t, []calendar.Date{mon.AddDays(1)}, bookedDates(days[2]), "cancelled on Wednesday: booked until Tuesday")
	assert.NotContains(t, days, int64(3), "cancelled before the week: never booked")
}

func TestHolidayGuestBookedDaysRefuseAnUnreadableTimeModel(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 6)
	care := NewHolidayGuestBookedCare(guestBookingsStub{
		{StudentID: 1, StartsOn: mon, EndsOn: mon, TimeModelStarts: "08:00", TimeModelEnds: "29:00"},
	})
	_, err := care.HolidayGuestBookedDays(context.Background(), []int64{1}, mon, mon)
	require.Error(t, err, "an unreadable end is an error, never a day without a booking")
}
