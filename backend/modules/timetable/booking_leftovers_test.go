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
		WeekdayEnds: map[int]string{1: "16:00:00", 2: "16:00:00", 3: "16:00:00", 4: "16:00:00", 5: "16:00:00"},
		Roster:      timetable.TemplatePlannedRoster{Entries: entries},
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
	row.KeptWithoutBookingThrough = &leftoverChange
	leftovers := timetable.DetectBookingLeftovers(leftoverScope(),
		[]timetable.BookingLeftoverTemplate{handTemplate(row)},
		[]timetable.OfferingWeekdayBooking{thursdayDropped(leftoverChild)})
	require.Len(t, leftovers, 1)
	assert.True(t, leftovers[0].Kept, "a kept leftover is listed as decided")

	earlier := leftoverChange.AddDays(-7)
	row.KeptWithoutBookingThrough = &earlier
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
		"Thursday-free":    {ID: leftoverTemplate, Name: "Mo", Kind: timetable.RosterOfferingCare, WeekdayEnds: map[int]string{1: "16:00:00"}, Roster: timetable.TemplatePlannedRoster{Entries: []timetable.TemplateRosterEntry{handRow(leftoverChild)}}},
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
	assert.Equal(t, future, leftovers[0].From, "the removal starts when the booking ends")
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
