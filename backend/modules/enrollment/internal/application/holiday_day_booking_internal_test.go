package application

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// A week from Monday 4 to Friday 8 January 2027.
var (
	firstFitMonday    = calendar.NewDate(2027, 1, 4)
	firstFitTuesday   = calendar.NewDate(2027, 1, 5)
	firstFitWednesday = calendar.NewDate(2027, 1, 6)
)

func firstFitIDs(bookings []enrollment.HolidayQueuedBooking) []int64 {
	ids := []int64{}
	for _, booking := range bookings {
		ids = append(ids, booking.ID)
	}
	return ids
}

// The capacity holds on each day (#4701): a booking gets its place only when
// all its days are free, it waits as a whole otherwise, and a later booking
// of other days passes it. A whole week needs every care day, so it keeps
// the order of #4699. Each case goes red without the day check.
func TestHolidayFirstFitHoldsTheCapacityOnEachDay(t *testing.T) {
	t.Parallel()
	one := 1
	queue := enrollment.HolidayWeekQueue{
		Capacity: &one, StartsOn: firstFitMonday, EndsOn: calendar.NewDate(2027, 1, 10),
		Bookings: []enrollment.HolidayQueuedBooking{
			{ID: 1, Placed: true, Days: []calendar.Date{firstFitMonday}},
			{ID: 2, Days: []calendar.Date{firstFitMonday, firstFitTuesday}},
			{ID: 3, Days: []calendar.Date{firstFitTuesday}},
			{ID: 4, Days: []calendar.Date{firstFitTuesday, firstFitWednesday}},
			{ID: 5, Days: []calendar.Date{firstFitWednesday}},
		},
	}
	assert.Equal(t, []int64{3, 5}, firstFitIDs(holidayFirstFit(queue)))

	two := 2
	whole := enrollment.HolidayWeekQueue{
		Capacity: &two, StartsOn: firstFitMonday, EndsOn: calendar.NewDate(2027, 1, 8),
		Bookings: []enrollment.HolidayQueuedBooking{{ID: 1, Placed: true}, {ID: 2}, {ID: 3}},
	}
	assert.Equal(t, []int64{2}, firstFitIDs(holidayFirstFit(whole)), "a whole week takes the next free place in order")

	whole.Capacity = nil
	assert.Equal(t, []int64{2, 3}, firstFitIDs(holidayFirstFit(whole)), "without a capacity every booking gets its place")
	zero := 0
	whole.Capacity = &zero
	assert.Empty(t, holidayFirstFit(whole))
}

// Whole-week offers without weekdays still have one place per child.
func TestHolidayFirstFitKeepsCapacityForWeekendOnlyWeeks(t *testing.T) {
	t.Parallel()
	one := 1
	queue := enrollment.HolidayWeekQueue{
		Capacity: &one, StartsOn: calendar.NewDate(2027, 1, 9), EndsOn: calendar.NewDate(2027, 1, 10),
		Bookings: []enrollment.HolidayQueuedBooking{{ID: 1}, {ID: 2}, {ID: 3}},
	}
	assert.Equal(t, []int64{1}, firstFitIDs(holidayFirstFit(queue)))
	queue.Bookings[0].Placed = true
	assert.Empty(t, holidayFirstFit(queue), "the existing booking already takes the only place")
	queue.Capacity = nil
	assert.Equal(t, []int64{2, 3}, firstFitIDs(holidayFirstFit(queue)))
}

// The care days of a week are its days from Monday to Friday; a whole week
// counts all of them against the bound of the period.
func TestHolidayDayLimitCountsTheCareDaysOfEveryBooking(t *testing.T) {
	t.Parallel()
	limit := 3
	period := enrollment.HolidayPeriod{MaxDaysPerChild: &limit, Weeks: []enrollment.HolidayWeek{
		{ID: 1, StartsOn: firstFitMonday, EndsOn: calendar.NewDate(2027, 1, 10), BookingUnit: enrollment.HolidayBookingUnitDay},
		{ID: 2, StartsOn: calendar.NewDate(2027, 1, 12), EndsOn: calendar.NewDate(2027, 1, 15), BookingUnit: enrollment.HolidayBookingUnitWeek},
	}}
	assert.Len(t, enrollment.HolidayWeekCareDays(firstFitMonday, calendar.NewDate(2027, 1, 10)), 5, "Saturday and Sunday are no care days")

	twoDays := []enrollment.HolidayBookingInput{{WeekID: 1, Days: []calendar.Date{firstFitMonday, firstFitTuesday}}}
	require.NoError(t, refuseHolidayDayLimit(0, twoDays, period, 0))
	shortWeek := []enrollment.HolidayBookingInput{{WeekID: 2}}
	assert.Equal(t, 4, holidayBookedDayCount(shortWeek, period), "a week from Tuesday counts four care days")

	err := refuseHolidayDayLimit(1, append(twoDays, shortWeek...), period, 0)
	var refusal *enrollment.HolidayChildDayLimitRefusal
	require.True(t, errors.As(err, &refusal))
	assert.Equal(t, enrollment.HolidayChildDayLimitRefusal{Child: 1, Limit: 3, Days: 6}, *refusal)
	assert.ErrorIs(t, err, enrollment.ErrHolidayChildDayLimit)

	assert.NoError(t, refuseHolidayDayLimit(0, shortWeek, period, 6), "a child over a lowered bound may lower its days")
	assert.Error(t, refuseHolidayDayLimit(0, append(twoDays, shortWeek...), period, 4), "but not raise them")
	period.MaxDaysPerChild = nil
	assert.NoError(t, refuseHolidayDayLimit(0, append(twoDays, shortWeek...), period, 0), "no bound, no refusal")
}

// A booking of a week by days names at least one of its care days and only
// those; a whole week names none.
func TestHolidayBookingDaysLieInTheirWeek(t *testing.T) {
	t.Parallel()
	byDays := enrollment.HolidayWeek{StartsOn: firstFitMonday, EndsOn: calendar.NewDate(2027, 1, 8), BookingUnit: enrollment.HolidayBookingUnitDay}
	assert.NoError(t, validateHolidayBookingDays(byDays, []calendar.Date{firstFitMonday}))
	assert.Error(t, validateHolidayBookingDays(byDays, nil))
	assert.Error(t, validateHolidayBookingDays(byDays, []calendar.Date{calendar.NewDate(2027, 1, 9)}))
	whole := byDays
	whole.BookingUnit = enrollment.HolidayBookingUnitWeek
	assert.NoError(t, validateHolidayBookingDays(whole, nil))
	assert.Error(t, validateHolidayBookingDays(whole, []calendar.Date{firstFitMonday}))
	assert.Equal(t, []calendar.Date{firstFitMonday, firstFitTuesday},
		enrollment.NormalizeHolidayDays([]calendar.Date{firstFitTuesday, firstFitMonday, firstFitTuesday}))
}

// A week by days with a day without care at its host school stays bookable
// on its other days (#4701 on #4702): the offer leaves the day out, a new
// booking of it is refused, a booking that had it before keeps it, and a
// whole week with such a day still takes no new booking.
func TestHolidayDayBookingLeavesOutTheDaysWithoutCare(t *testing.T) {
	t.Parallel()
	closed := enrollment.HolidayNoCareDay{Date: firstFitTuesday, Kind: enrollment.HolidayNoCareClosingDay, Name: "Teamtag"}
	byDays := enrollment.HolidayWeek{ID: 1, StartsOn: firstFitMonday, EndsOn: calendar.NewDate(2027, 1, 8),
		BookingUnit: enrollment.HolidayBookingUnitDay, NoCareDays: []enrollment.HolidayNoCareDay{closed}}
	whole := byDays
	whole.ID, whole.BookingUnit = 2, enrollment.HolidayBookingUnitWeek
	period := enrollment.HolidayPeriod{Weeks: []enrollment.HolidayWeek{byDays, whole}}

	assert.NotContains(t, holidayBookableCareDays(byDays), firstFitTuesday)
	assert.Len(t, holidayBookableCareDays(byDays), 4)
	assert.NoError(t, refuseHolidayBookingsWithoutCare(
		[]enrollment.HolidayBookingInput{{WeekID: 1, Days: []calendar.Date{firstFitMonday, firstFitWednesday}}}, period, nil))
	tuesday := []enrollment.HolidayBookingInput{{WeekID: 1, Days: []calendar.Date{firstFitMonday, firstFitTuesday}}}
	var refusal *enrollment.HolidayWeekNoCareDayRefusal
	require.ErrorAs(t, refuseHolidayBookingsWithoutCare(tuesday, period, nil), &refusal)
	assert.Equal(t, firstFitTuesday, refusal.Day.Date)
	assert.NoError(t, refuseHolidayBookingsWithoutCare(tuesday, period, tuesday), "a day booked before stays")
	assert.Error(t, refuseHolidayBookingsWithoutCare([]enrollment.HolidayBookingInput{{WeekID: 2}}, period, nil))
}

func TestDailyHolidayWeekNeedsASelectableWeekday(t *testing.T) {
	t.Parallel()
	week := enrollment.HolidayWeekInput{HostSchoolID: 1, StartsOn: calendar.NewDate(2027, 1, 9),
		EndsOn: calendar.NewDate(2027, 1, 10), BookingUnit: enrollment.HolidayBookingUnitDay}
	assert.Contains(t, validateHolidayWeek(week), "ends_on", "a weekend-only daily offer has no selectable day")
	stored := enrollment.HolidayPeriod{Weeks: []enrollment.HolidayWeek{{StartsOn: week.StartsOn, EndsOn: week.EndsOn,
		BookingUnit: week.BookingUnit, TimeModels: []enrollment.HolidayTimeModel{{ID: 1}}}}}
	assert.False(t, holidayPeriodComplete(stored), "an existing weekend-only draft cannot be published")
	week.StartsOn = calendar.NewDate(2027, 1, 8)
	assert.Empty(t, validateHolidayWeek(week), "Friday remains selectable")
	week.StartsOn, week.BookingUnit = calendar.NewDate(2027, 1, 9), enrollment.HolidayBookingUnitWeek
	assert.Empty(t, validateHolidayWeek(week), "whole-week offers keep their existing date contract")
}
