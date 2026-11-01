package timetable_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	leftoverChild    = int64(501)
	leftoverOther    = int64(502)
	leftoverPeriod   = int64(77)
	leftoverGanztag  = int64(601)
	leftoverMensa    = int64(602)
	leftoverTemplate = int64(701)
)

// Wednesday 2026-11-04; the booking change took effect on Monday.
var (
	leftoverToday  = calendar.NewDate(2026, 11, 4)
	leftoverChange = calendar.NewDate(2026, 11, 2)
)

func leftoverScope() timetable.BookingLeftoverScope {
	return timetable.BookingLeftoverScope{Tomorrow: leftoverToday.AddDays(1), PeriodID: leftoverPeriod}
}

func leftoverWindow(from, until calendar.Date, ended bool, weekdays ...int) timetable.OfferingBookingWindow {
	return timetable.OfferingBookingWindow{ValidFrom: &from, ValidUntil: &until, Weekdays: weekdays, Ended: ended}
}

func leftoverBooking(offeringID, studentID int64, pickup map[int]string, windows ...timetable.OfferingBookingWindow) timetable.OfferingWeekdayBooking {
	return timetable.OfferingWeekdayBooking{
		OfferingID: offeringID, StudentID: studentID, CountsAsCare: true, PickupTimes: pickup, Windows: windows,
	}
}

// handTemplate is a care block 14:00–16:00 on Monday to Friday that plans the
// children through hand rows.
func handTemplate(entries ...timetable.TemplateRosterEntry) timetable.BookingLeftoverTemplate {
	return timetable.BookingLeftoverTemplate{
		ID: leftoverTemplate, Name: "Hausaufgaben", Kind: timetable.RosterOfferingCare,
		Schedules: weeklySchedules(1, 2, 3, 4, 5),
		Roster:    timetable.TemplatePlannedRoster{Entries: entries},
	}
}

// weeklySchedules runs 14:00–16:00 every week on the weekdays.
func weeklySchedules(weekdays ...int) []timetable.BookingLeftoverSchedule {
	schedules := make([]timetable.BookingLeftoverSchedule, 0, len(weekdays))
	for _, weekday := range weekdays {
		schedules = append(schedules, timetable.BookingLeftoverSchedule{Weekday: weekday, End: "16:00:00"})
	}
	return schedules
}

// endThursdays ends the Thursday schedule on the exclusive day.
func endThursdays(template *timetable.BookingLeftoverTemplate, until calendar.Date) {
	for index := range template.Schedules {
		if template.Schedules[index].Weekday == 4 {
			template.Schedules[index].ValidUntil = &until
		}
	}
}

func handRow(studentID int64) timetable.TemplateRosterEntry {
	from := calendar.NewDate(2026, 8, 17)
	return timetable.TemplateRosterEntry{StudentID: studentID, ValidFrom: &from}
}

var (
	phaseStart = calendar.NewDate(2026, 8, 17)
	phaseEnd   = calendar.NewDate(2027, 7, 1)
)

// thursdayDropped is the reproduced case: a Korrektur ended the Ganztag link
// for Monday to Friday on the change day and booked it again without
// Thursday.
func thursdayDropped(studentID int64) timetable.OfferingWeekdayBooking {
	return leftoverBooking(leftoverGanztag, studentID, nil,
		leftoverWindow(phaseStart, leftoverChange, true, 1, 2, 3, 4, 5),
		leftoverWindow(leftoverChange, phaseEnd, false, 1, 2, 3, 5))
}

func TestDetectBookingLeftoversNamesTheDroppedWeekday(t *testing.T) {
	t.Parallel()
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild), handRow(leftoverOther))},
		[]timetable.OfferingWeekdayBooking{
			thursdayDropped(leftoverChild),
			leftoverBooking(leftoverGanztag, leftoverOther, nil, leftoverWindow(phaseStart, phaseEnd, false, 1, 2, 3, 4, 5)),
		})

	require.Len(t, leftovers, 1, "only the child whose Thursday ended")
	assert.Equal(t, timetable.BookingLeftover{
		TemplateID: leftoverTemplate, TemplateName: "Hausaufgaben", StudentID: leftoverChild,
		Weekdays: []int{4}, BookingEndedOn: leftoverChange, From: leftoverToday.AddDays(1),
	}, leftovers[0])
}

func TestDetectBookingLeftoversIgnoresPhaseEndAndRemainingBookings(t *testing.T) {
	t.Parallel()
	phaseOnly := leftoverBooking(leftoverGanztag, leftoverChild, nil,
		leftoverWindow(phaseStart, leftoverChange, false, 1, 2, 3, 4, 5))
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild))},
		[]timetable.OfferingWeekdayBooking{phaseOnly})
	assert.Empty(t, leftovers, "a window the phase ends is no booking change")

	switched := []timetable.OfferingWeekdayBooking{
		leftoverBooking(leftoverGanztag, leftoverChild, nil, leftoverWindow(phaseStart, leftoverChange, true, 4)),
		leftoverBooking(leftoverMensa, leftoverChild, nil, leftoverWindow(leftoverChange, phaseEnd, false, 4)),
	}
	leftovers = timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild))}, switched)
	assert.Empty(t, leftovers, "without pickup times a switch offsets the dropped offering, as in the save response")
}

func TestDetectBookingLeftoversUsesPickupTimes(t *testing.T) {
	t.Parallel()
	early := map[int]string{4: "14:30"}
	bookings := []timetable.OfferingWeekdayBooking{
		leftoverBooking(leftoverGanztag, leftoverChild, map[int]string{4: "16:00"}, leftoverWindow(phaseStart, leftoverChange, true, 4)),
		leftoverBooking(leftoverMensa, leftoverChild, early, leftoverWindow(leftoverChange, phaseEnd, false, 4)),
	}
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild))}, bookings)
	require.Len(t, leftovers, 1, "pickup at 14:30 does not carry a block until 16:00")
	assert.Equal(t, []int{4}, leftovers[0].Weekdays)

	bookings[1].PickupTimes = map[int]string{4: "16:30"}
	leftovers = timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild))}, bookings)
	assert.Empty(t, leftovers, "pickup at 16:30 carries the block")
}

func TestDetectBookingLeftoversRespectsTheKeepDecision(t *testing.T) {
	t.Parallel()
	row := handRow(leftoverChild)
	row.KeptWithoutBooking = map[int]calendar.Date{4: leftoverChange}
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(row)},
		[]timetable.OfferingWeekdayBooking{thursdayDropped(leftoverChild)})
	require.Len(t, leftovers, 1)
	assert.True(t, leftovers[0].Kept, "a kept leftover is listed as decided")

	earlier := leftoverChange.AddDays(-7)
	row.KeptWithoutBooking = map[int]calendar.Date{4: earlier}
	leftovers = timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(row)},
		[]timetable.OfferingWeekdayBooking{thursdayDropped(leftoverChild)})
	require.Len(t, leftovers, 1)
	assert.False(t, leftovers[0].Kept, "a booking ending after the decision shows the hint again")
}

func TestDetectBookingLeftoversSkipsBookingRowsTargetsAndEndedRows(t *testing.T) {
	t.Parallel()
	booked := handRow(leftoverChild)
	booked.FromBooking = true
	ended := handRow(leftoverChild)
	until := leftoverToday
	ended.ValidUntil = &until
	targeted := handTemplate(handRow(leftoverChild))
	targeted.Roster.TargetStudentIDs = []int64{leftoverChild}
	for name, template := range map[string]timetable.BookingLeftoverTemplate{
		"booking row":      handTemplate(booked),
		"row ended":        handTemplate(ended),
		"via Klasse":       targeted,
		"Thursday-free":    {ID: leftoverTemplate, Name: "Mo", Kind: timetable.RosterOfferingCare, Schedules: weeklySchedules(1), Roster: timetable.TemplatePlannedRoster{Entries: []timetable.TemplateRosterEntry{handRow(leftoverChild)}}},
		"other period pin": handTemplate(timetable.TemplateRosterEntry{StudentID: leftoverChild, CalendarPeriodID: new(int64(9))}),
	} {
		leftovers := timetable.DetectBookingLeftovers(leftoverScope(), []timetable.BookingLeftoverTemplate{template},
			[]timetable.OfferingWeekdayBooking{thursdayDropped(leftoverChild)})
		assert.Empty(t, leftovers, name)
	}
}

func TestDetectBookingLeftoversStartsAFutureChangeOnItsDay(t *testing.T) {
	t.Parallel()
	future := calendar.NewDate(2026, 12, 1)
	booking := leftoverBooking(leftoverGanztag, leftoverChild, nil,
		leftoverWindow(phaseStart, future, true, 1, 4),
		leftoverWindow(future, phaseEnd, false, 1))
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild))},
		[]timetable.OfferingWeekdayBooking{booking})
	require.Len(t, leftovers, 1)
	assert.Equal(t, calendar.NewDate(2026, 12, 3), leftovers[0].From, "the removal starts on the first Thursday after the booking ends")
	assert.Equal(t, future, leftovers[0].BookingEndedOn)
}

// A link that ends where the same offering continues on the weekday is a
// rebooking, not a drop: the save response would name no dropped day, and
// a Regeltermin of another kind must not show up for it (seeded AG lists).
func TestDetectBookingLeftoversIgnoresARebookedOffering(t *testing.T) {
	t.Parallel()
	rebooked := leftoverBooking(leftoverGanztag, leftoverChild, map[int]string{4: "16:00"},
		leftoverWindow(phaseStart, leftoverChange, true, 1, 2, 3, 4, 5),
		leftoverWindow(leftoverChange, phaseEnd, false, 1, 2, 3, 4, 5))
	course := handTemplate(handRow(leftoverChild))
	course.Kind = timetable.RosterOfferingCourse
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{course}, []timetable.OfferingWeekdayBooking{rebooked})
	assert.Empty(t, leftovers)
}

// A weekday whose schedules end before the booking does plans nothing the
// change touches (review of #5452): no hint and no removal to offer.
func TestDetectBookingLeftoversNeedsAnOccurrenceAfterTheDrop(t *testing.T) {
	t.Parallel()
	future := calendar.NewDate(2026, 12, 1)
	booking := leftoverBooking(leftoverGanztag, leftoverChild, nil,
		leftoverWindow(phaseStart, future, true, 4),
		leftoverWindow(future, phaseEnd, false, 1))
	template := handTemplate(handRow(leftoverChild))
	endThursdays(&template, calendar.NewDate(2026, 11, 27))
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{template}, []timetable.OfferingWeekdayBooking{booking})
	assert.Empty(t, leftovers, "the last Thursday is 26.11., before the booking ends")

	endThursdays(&template, calendar.NewDate(2026, 12, 4))
	leftovers = timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{template}, []timetable.OfferingWeekdayBooking{booking})
	require.Len(t, leftovers, 1, "Thursday 03.12. still runs")
}

// A row must plan the weekday on a day the series still runs (quorum review
// of #5452): a row ending Wednesday after a Tuesday start plans no Thursday,
// and a row starting after the series ended plans nothing.
func TestDetectBookingLeftoversNeedsADayOfTheRow(t *testing.T) {
	t.Parallel()
	booking := thursdayDropped(leftoverChild) // tomorrow is Thursday 05.11.
	endsWednesday := handRow(leftoverChild)
	wednesday := calendar.NewDate(2026, 11, 11)
	tuesday := calendar.NewDate(2026, 11, 10)
	endsWednesday.ValidFrom, endsWednesday.ValidUntil = &tuesday, &wednesday
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(endsWednesday)}, []timetable.OfferingWeekdayBooking{booking})
	assert.Empty(t, leftovers, "the row runs Tuesday 10.11. only, no Thursday")

	afterSeries := handRow(leftoverChild)
	late := calendar.NewDate(2026, 12, 1)
	afterSeries.ValidFrom = &late
	template := handTemplate(afterSeries)
	endThursdays(&template, calendar.NewDate(2026, 11, 27))
	leftovers = timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{template}, []timetable.OfferingWeekdayBooking{booking})
	assert.Empty(t, leftovers, "the row starts after the last Thursday of the series")
}

// A hand row that starts later is judged on its first planned day (quorum
// review of #5452): a booking starting with the row carries every Thursday
// the row plans, so nothing is left over.
func TestDetectBookingLeftoversJudgesTheRowsFirstDay(t *testing.T) {
	t.Parallel()
	rowStart := calendar.NewDate(2026, 12, 1)
	row := handRow(leftoverChild)
	row.ValidFrom = &rowStart
	bookings := []timetable.OfferingWeekdayBooking{
		leftoverBooking(leftoverGanztag, leftoverChild, nil, leftoverWindow(phaseStart, leftoverChange, true, 4)),
		leftoverBooking(leftoverMensa, leftoverChild, nil, leftoverWindow(rowStart, phaseEnd, false, 4)),
	}
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(row)}, bookings)
	assert.Empty(t, leftovers, "the Mensa booking carries every Thursday from 03.12. on")
}

// A current gap is not hidden by a later booking that ends as well (quorum
// review of #5452): the removal starts with the first uncarried Thursday.
func TestDetectBookingLeftoversStartsWithTheEarliestGap(t *testing.T) {
	t.Parallel()
	later, laterEnds := calendar.NewDate(2026, 12, 1), calendar.NewDate(2027, 1, 5)
	bookings := []timetable.OfferingWeekdayBooking{
		leftoverBooking(leftoverGanztag, leftoverChild, nil, leftoverWindow(phaseStart, leftoverChange, true, 4)),
		leftoverBooking(leftoverMensa, leftoverChild, nil, leftoverWindow(later, laterEnds, true, 4)),
	}
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild))}, bookings)
	require.Len(t, leftovers, 1)
	assert.Equal(t, calendar.NewDate(2026, 11, 5), leftovers[0].From, "the first Thursday without a booking")
	assert.Equal(t, laterEnds, leftovers[0].BookingEndedOn, "Bleibt so covers every ended booking of the day")
}

// Adjacent cases of the same rule (quorum review of #5452): the first
// planned day follows A/B weeks, a later gap counts when an earlier one is
// carried, a later row starts on its own first day, and nothing after the
// planning period is planned.
func TestDetectBookingLeftoversFollowsThePlannedDays(t *testing.T) {
	t.Parallel()
	// Week B: Thursdays on even weeks since Monday 02.11.2026.
	anchor := calendar.NewDate(2026, 11, 2)
	scope := leftoverScope()
	scope.WeekPatternApplies = func(weekPattern int, date calendar.Date) bool {
		return weekPattern == 0 || (anchor.DaysUntil(date)/7)%2+1 == weekPattern
	}
	weekB := handTemplate(handRow(leftoverChild))
	for index := range weekB.Schedules {
		weekB.Schedules[index].WeekPattern = 2
	}
	leftovers := timetable.DetectBookingLeftovers(scope,
		[]timetable.BookingLeftoverTemplate{weekB}, []timetable.OfferingWeekdayBooking{thursdayDropped(leftoverChild)})
	require.Len(t, leftovers, 1)
	assert.Equal(t, calendar.NewDate(2026, 11, 12), leftovers[0].From, "05.11. is week A, the first Thursday of week B is 12.11.")

	endThursdays(&weekB, calendar.NewDate(2026, 11, 12))
	leftovers = timetable.DetectBookingLeftovers(scope,
		[]timetable.BookingLeftoverTemplate{weekB}, []timetable.OfferingWeekdayBooking{thursdayDropped(leftoverChild)})
	assert.Empty(t, leftovers, "the series ends before its next week-B Thursday")

	// An earlier gap carried by a replacement, a later one open.
	replacementEnds := calendar.NewDate(2026, 12, 1)
	bookings := []timetable.OfferingWeekdayBooking{
		leftoverBooking(leftoverGanztag, leftoverChild, nil, leftoverWindow(phaseStart, leftoverChange, true, 4)),
		leftoverBooking(leftoverMensa, leftoverChild, nil, leftoverWindow(leftoverChange, replacementEnds, true, 4)),
	}
	leftovers = timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild))}, bookings)
	require.Len(t, leftovers, 1)
	assert.Equal(t, calendar.NewDate(2026, 12, 3), leftovers[0].From, "Thursdays up to 26.11. are carried")
	assert.Equal(t, replacementEnds, leftovers[0].BookingEndedOn)

	// A later row without a replacement starts on its own first Thursday.
	rowStart := calendar.NewDate(2026, 12, 1)
	later := handRow(leftoverChild)
	later.ValidFrom = &rowStart
	leftovers = timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(later)}, []timetable.OfferingWeekdayBooking{thursdayDropped(leftoverChild)})
	require.Len(t, leftovers, 1)
	assert.Equal(t, calendar.NewDate(2026, 12, 3), leftovers[0].From)

	// The planning period ends before the next Thursday.
	short := leftoverScope()
	short.Until = calendar.NewDate(2026, 11, 5)
	leftovers = timetable.DetectBookingLeftovers(short,
		[]timetable.BookingLeftoverTemplate{handTemplate(handRow(leftoverChild))}, []timetable.OfferingWeekdayBooking{thursdayDropped(leftoverChild)})
	assert.Empty(t, leftovers, "no Thursday is left in the period")
}
