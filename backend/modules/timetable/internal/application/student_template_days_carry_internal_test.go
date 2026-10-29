package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// Pickup times as Schule A books them (#5342): Randstunde until 12:45,
// Ganztag until 14:30 or 16:00.
func careDay(weekday int, pickup string) timetable.StudentDayBooking {
	return timetable.StudentDayBooking{Weekday: weekday, CountsAsCare: true, PickupTime: pickup}
}

const (
	friday     = timetable.WeekdayFriday
	monday     = timetable.WeekdayMonday
	endAt1245  = 12*60 + 45
	endAt1345  = 13*60 + 45
	endAt1500  = 15 * 60
	endAt1600  = 16 * 60
	unknownEnd = 0
)

// noNetLoss and netLossOn build the carry of the bookings left after a change.
func noNetLoss(bookings ...timetable.StudentDayBooking) bookingCarry {
	return bookingCarry{bookings: bookings, netLoss: map[int]bool{}}
}

func netLossOn(weekday int, bookings ...timetable.StudentDayBooking) bookingCarry {
	return bookingCarry{bookings: bookings, netLoss: map[int]bool{weekday: true}}
}

var (
	careKind   = timetable.RosterOfferingKindFor(timetable.GroupTypeCare, nil)
	courseKind = timetable.RosterOfferingKindFor(timetable.GroupTypeActivity, nil)
	mensa      = timetable.ListKindMensa
	mensaKind  = timetable.RosterOfferingKindFor(timetable.GroupTypeCare, &mensa)
)

func TestBookingCarry_RandstundeThatStaysCarriesTheClassRandstunde(t *testing.T) {
	t.Parallel()

	// Ganztag dropped on Friday, the Randstunde stays: the class Randstunde
	// until 12:45 keeps its booking.
	assert.True(t, netLossOn(friday, careDay(friday, "12:45")).keeps(friday, careKind, endAt1245, true))
}

func TestBookingCarry_GanztagCarriesTheEarlierClassRandstunde(t *testing.T) {
	t.Parallel()

	// Randstunde dropped, Ganztag stays: Ganztag care runs past 12:45, like
	// the Ganztag children the class Randstunde already lists.
	assert.True(t, netLossOn(monday, careDay(monday, "14:30")).keeps(monday, careKind, endAt1245, true))
}

func TestBookingCarry_EarlierPickupDoesNotCarryALaterBlock(t *testing.T) {
	t.Parallel()

	// Ganztag until 16:00 switched to 14:30, no net loss: a block until 16:00
	// loses its booking, a block until 13:45 keeps it.
	carry := noNetLoss(careDay(monday, "14:30"))
	assert.False(t, carry.keeps(monday, careKind, endAt1600, true))
	assert.True(t, carry.keeps(monday, careKind, endAt1345, true))
}

func TestBookingCarry_OnlyTheSameWeekdayCarries(t *testing.T) {
	t.Parallel()

	assert.False(t, noNetLoss(careDay(monday, "16:00")).keeps(friday, careKind, endAt1245, true))
	assert.False(t, netLossOn(friday).keeps(friday, careKind, endAt1245, true), "a child without any booking keeps nothing")
}

// Review finding 1: an AG needs a course offering. The course is dropped,
// Ganztag until 16:00 stays: the AG until 15:00 loses its booking.
func TestBookingCarry_AGNeedsACourseOffering(t *testing.T) {
	t.Parallel()

	ganztag := careDay(monday, "16:00")
	course := timetable.StudentDayBooking{Weekday: monday, PickupTime: "16:00"}
	assert.False(t, netLossOn(monday, ganztag).keeps(monday, courseKind, endAt1500, true))
	assert.True(t, noNetLoss(course).keeps(monday, courseKind, endAt1500, true))
	assert.False(t, noNetLoss(course).keeps(monday, careKind, endAt1245, true), "a course carries no care block")
}

// A Mensa list needs an offering with lunch.
func TestBookingCarry_MensaListNeedsLunch(t *testing.T) {
	t.Parallel()

	lunch := timetable.StudentDayBooking{Weekday: friday, CountsAsCare: true, IncludesLunch: true, PickupTime: "14:30"}
	assert.True(t, noNetLoss(lunch).keeps(friday, mensaKind, endAt1345, true))
	assert.False(t, noNetLoss(careDay(friday, "14:30")).keeps(friday, mensaKind, endAt1345, true))
}

// Missing data: when a booking of the day has no pickup time, the earlier
// rule decides exactly, whatever the kinds. A Randstunde and Ganztag without
// pickup times: dropping Ganztag beside the Randstunde is a net loss and is
// reported; a switch is none and is not.
func TestBookingCarry_WithoutPickupTimesKeepsTheNetLossRule(t *testing.T) {
	t.Parallel()

	assert.False(t, netLossOn(friday, careDay(friday, "")).keeps(friday, careKind, endAt1600, true))
	assert.True(t, noNetLoss(careDay(friday, "")).keeps(friday, careKind, endAt1600, true))
	assert.False(t, netLossOn(friday).keeps(friday, careKind, endAt1600, true), "no booking left is always reported")
}

// Review finding 3: care A is replaced by course B on the same day, both
// without a pickup time. The earlier balance is even, so the care block is
// not reported; the kind filter must not run before that decision.
func TestBookingCarry_NetLossRuleIgnoresTheKinds(t *testing.T) {
	t.Parallel()

	course := timetable.StudentDayBooking{Weekday: monday}
	assert.True(t, noNetLoss(course).keeps(monday, careKind, endAt1600, true))
	assert.False(t, netLossOn(monday, course).keeps(monday, careKind, endAt1600, true))
}

// One booking of the day without a pickup time leaves the times undecided,
// even beside a booking whose pickup time reaches the end.
func TestBookingCarry_OneUnknownPickupTimeKeepsTheNetLossRule(t *testing.T) {
	t.Parallel()

	assert.False(t, netLossOn(friday, careDay(friday, ""), careDay(friday, "16:00")).keeps(friday, careKind, endAt1600, true))
}

// A Regeltermin without an end follows the earlier rule as well.
func TestBookingCarry_RegelterminWithoutEndKeepsTheNetLossRule(t *testing.T) {
	t.Parallel()

	assert.False(t, netLossOn(friday, careDay(friday, "12:45")).keeps(friday, careKind, unknownEnd, false))
	assert.True(t, noNetLoss(careDay(friday, "12:45")).keeps(friday, careKind, unknownEnd, false))
}

func TestNewBookingCarry_RejectsWeekdaysOutsideTheWeek(t *testing.T) {
	t.Parallel()

	_, err := newBookingCarry(timetable.ManualStudentPlanQuery{Bookings: []timetable.StudentDayBooking{careDay(8, "12:45")}})
	assert.ErrorIs(t, err, timetable.ErrStudentTemplateDaysInvalid)
	_, err = newBookingCarry(timetable.ManualStudentPlanQuery{NetLossWeekdays: []int{0}})
	assert.ErrorIs(t, err, timetable.ErrStudentTemplateDaysInvalid)
}

// Review finding 2: a schedule counts only while materialization's validity
// rule keeps it from the date on: valid_until is exclusive, and a period
// that ended before the date ends the schedule too.
func TestScheduleRunsFrom_UsesMaterializationValidity(t *testing.T) {
	t.Parallel()

	from := calendar.NewDate(2027, 5, 31)
	period := func(id int64, start, end calendar.Date) *schedule.CalendarPeriod {
		p := &schedule.CalendarPeriod{StartDate: start, EndDate: end, IsActive: true, WeekCycleLength: 1}
		p.ID = id
		return p
	}
	current := period(41, calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 7, 31))
	expired := period(42, calendar.NewDate(2025, 8, 1), calendar.NewDate(2026, 7, 31))
	periods := []*schedule.CalendarPeriod{current, expired}
	template := &activityContract.Group{}
	pinned := func(periodID int64) *activityContract.Schedule {
		return &activityContract.Schedule{Weekday: monday, CalendarPeriodID: &periodID}
	}

	assert.True(t, scheduleRunsFrom(template, pinned(current.ID), from, periods))
	assert.False(t, scheduleRunsFrom(template, pinned(expired.ID), from, periods), "expired period")
	until := activityContract.Date(from)
	assert.False(t, scheduleRunsFrom(template, &activityContract.Schedule{Weekday: monday, ValidUntil: &until}, from, periods), "valid_until is exclusive")
	later := activityContract.Date(from.AddDays(1))
	assert.True(t, scheduleRunsFrom(template, &activityContract.Schedule{Weekday: monday, ValidUntil: &later}, from, periods))
	lastDay := activityContract.Date(from.AddDays(-1))
	assert.False(t, scheduleRunsFrom(&activityContract.Group{SeriesLastDay: &lastDay}, pinned(current.ID), from, periods), "series ended")
}

func TestClockMinutes_ReadsPickupAndTimeframeClocks(t *testing.T) {
	t.Parallel()

	for clock, want := range map[string]int{"12:45": endAt1245, "12:45:00": endAt1245, "16:00:30": endAt1600} {
		got, ok := clockMinutes(clock)
		assert.True(t, ok, clock)
		assert.Equal(t, want, got, clock)
	}
	for _, clock := range []string{"", "Mittag", "25:00"} {
		_, ok := clockMinutes(clock)
		assert.False(t, ok, clock)
	}
}

// dropCarriedDays keeps exactly the weekdays no booking carries, takes the
// latest end of a weekday and drops a template without any weekday left.
func TestDropCarriedDays_KeepsOnlyUncarriedWeekdays(t *testing.T) {
	t.Parallel()

	const classRandstunde, afternoon, duty = 31, 32, 33
	groups := []timetable.Group{
		{ID: classRandstunde, Type: timetable.GroupTypeCare},
		{ID: afternoon, Type: timetable.GroupTypeCare},
		{ID: duty, Type: timetable.GroupTypeDuty},
	}
	days := templateDays{}
	ends := templateDayEnds{}
	for _, weekday := range []int{monday, friday} {
		days.add(classRandstunde, weekday)
		ends.addClock(classRandstunde, weekday, "12:45:00")
		days.add(afternoon, weekday)
		ends.addClock(afternoon, weekday, "13:45:00")
		ends.addClock(afternoon, weekday, "16:00:00")
	}
	days.add(duty, friday)
	carry := noNetLoss(careDay(friday, "12:45"), careDay(monday, "16:00"))

	kept := carry.dropCarriedDays(groups, days, ends)

	assert.Equal(t, []timetable.Group{groups[1]}, kept)
	assert.Equal(t, templateDays{afternoon: {friday: true}}, days)
}
