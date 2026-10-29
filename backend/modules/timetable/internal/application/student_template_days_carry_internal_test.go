package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
	endAt1600  = 16 * 60
	unknownEnd = 0
)

func TestCarriedBy_RandstundeThatStaysCarriesTheClassRandstunde(t *testing.T) {
	t.Parallel()

	// Ganztag dropped on Friday, the Randstunde stays: the class Randstunde
	// until 12:45 keeps its booking.
	assert.Equal(t, carried, carriedBy([]timetable.StudentDayBooking{careDay(friday, "12:45")}, friday, true, endAt1245, true))
}

func TestCarriedBy_GanztagCarriesTheEarlierClassRandstunde(t *testing.T) {
	t.Parallel()

	// Randstunde dropped, Ganztag stays: Ganztag care runs past 12:45, like
	// the Ganztag children the class Randstunde already lists.
	assert.Equal(t, carried, carriedBy([]timetable.StudentDayBooking{careDay(monday, "14:30")}, monday, true, endAt1245, true))
}

func TestCarriedBy_EarlierPickupDoesNotCarryALaterBlock(t *testing.T) {
	t.Parallel()

	// Ganztag until 16:00 switched to 14:30: a block until 16:00 loses its
	// booking, a block until 13:45 keeps it.
	bookings := []timetable.StudentDayBooking{careDay(monday, "14:30")}
	assert.Equal(t, notCarried, carriedBy(bookings, monday, true, endAt1600, true))
	assert.Equal(t, carried, carriedBy(bookings, monday, true, endAt1345, true))
}

func TestCarriedBy_OnlyTheSameWeekdayCarries(t *testing.T) {
	t.Parallel()

	assert.Equal(t, notCarried, carriedBy([]timetable.StudentDayBooking{careDay(monday, "16:00")}, friday, true, endAt1245, true))
	assert.Equal(t, notCarried, carriedBy(nil, friday, true, endAt1245, true), "a child without any booking keeps nothing")
}

// Without a pickup time or a Regeltermin end the times cannot decide.
func TestCarriedBy_MissingPickupOrEndLeavesItUndecided(t *testing.T) {
	t.Parallel()

	assert.Equal(t, undecided, carriedBy([]timetable.StudentDayBooking{careDay(friday, "")}, friday, true, endAt1600, true))
	assert.Equal(t, undecided, carriedBy([]timetable.StudentDayBooking{careDay(friday, "12:45")}, friday, true, unknownEnd, false))
	assert.Equal(t, carried, carriedBy([]timetable.StudentDayBooking{careDay(friday, ""), careDay(friday, "16:00")}, friday, true, endAt1600, true),
		"a booking with a pickup time that reaches the end decides")
}

// A care Regeltermin needs an offering that counts as care; other
// Regeltermine take any offering.
func TestCarriedBy_CareRegelterminNeedsACareOffering(t *testing.T) {
	t.Parallel()

	course := []timetable.StudentDayBooking{{Weekday: friday, PickupTime: "16:00"}}
	assert.Equal(t, notCarried, carriedBy(course, friday, true, endAt1245, true))
	assert.Equal(t, carried, carriedBy(course, friday, false, endAt1245, true))
}

// Mensa lists are care Regeltermine. Schools that flag no offering with
// lunch still have Mensa lists, so the lunch flag does not decide; the
// pickup time does.
func TestCarriedBy_MensaListDoesNotNeedTheLunchFlag(t *testing.T) {
	t.Parallel()

	assert.Equal(t, carried, carriedBy([]timetable.StudentDayBooking{careDay(friday, "14:30")}, friday, true, endAt1345, true))
}

// A school with a Randstunde and Ganztag variants but without pickup times
// (missing data, #5342): the times cannot decide, so the day keeps the
// earlier rule. Dropping Ganztag beside the Randstunde is a net loss and is
// reported; a switch between Ganztag variants is none and is not.
func TestBookingCarry_WithoutPickupTimesKeepsTheNetLossRule(t *testing.T) {
	t.Parallel()

	randstundeLeft := bookingCarry{bookings: []timetable.StudentDayBooking{careDay(friday, "")}, netLoss: map[int]bool{friday: true}}
	assert.False(t, randstundeLeft.keeps(friday, true, endAt1600, true))
	switched := bookingCarry{bookings: []timetable.StudentDayBooking{careDay(friday, "")}, netLoss: map[int]bool{}}
	assert.True(t, switched.keeps(friday, true, endAt1600, true))
	nothingLeft := bookingCarry{netLoss: map[int]bool{}}
	assert.False(t, nothingLeft.keeps(friday, true, endAt1600, true), "no booking left is always reported")
}

// A Regeltermin without an end follows the same rule.
func TestBookingCarry_RegelterminWithoutEndKeepsTheNetLossRule(t *testing.T) {
	t.Parallel()

	carry := bookingCarry{bookings: []timetable.StudentDayBooking{careDay(friday, "12:45")}, netLoss: map[int]bool{friday: true}}
	assert.False(t, carry.keeps(friday, true, unknownEnd, false))
	carry.netLoss = map[int]bool{}
	assert.True(t, carry.keeps(friday, true, unknownEnd, false))
}

func TestNewBookingCarry_RejectsWeekdaysOutsideTheWeek(t *testing.T) {
	t.Parallel()

	_, err := newBookingCarry(timetable.ManualStudentPlanQuery{Bookings: []timetable.StudentDayBooking{careDay(8, "12:45")}})
	assert.ErrorIs(t, err, timetable.ErrStudentTemplateDaysInvalid)
	_, err = newBookingCarry(timetable.ManualStudentPlanQuery{NetLossWeekdays: []int{0}})
	assert.ErrorIs(t, err, timetable.ErrStudentTemplateDaysInvalid)
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
	carry := bookingCarry{bookings: []timetable.StudentDayBooking{careDay(friday, "12:45"), careDay(monday, "16:00")}, netLoss: map[int]bool{}}

	kept := carry.dropCarriedDays(groups, days, ends)

	assert.Equal(t, []timetable.Group{groups[1]}, kept)
	assert.Equal(t, templateDays{afternoon: {friday: true}}, days)
}
