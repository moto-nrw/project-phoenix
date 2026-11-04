package timetable_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// TestDroppedChildrenAlternatives: Schule am Berg chose only "Randstunde" as
// source for class 1a; children booked in a Ganztag offering dropped from the
// preview without a reason. The alternatives name the offerings that would
// keep them, counting each child once.
func TestDroppedChildrenAlternatives(t *testing.T) {
	t.Parallel()
	const (
		ganztag16ID = int64(303)
		otherPhase  = int64(304)
		holidayID   = int64(305)
	)
	roster := classRoster([]int{1, 3}, studentRange(1001, 5), "1a")
	inPhase := func(b timetable.OfferingWeekdayBooking, phase int64) timetable.OfferingWeekdayBooking {
		b.PhaseID = phase
		return b
	}
	holiday := inPhase(booking(holidayID, "Ferien", 1005, "1a", 1, 3), 7)
	holiday.HolidayPhase = true
	bookings := []timetable.OfferingWeekdayBooking{
		inPhase(booking(randstundeID, "Randstunde", 1001, "1a", 1, 3), 3),
		// 1002 is in Ganztag on both days: counted once.
		inPhase(booking(ganztagID, "Ganztag bis 14.30", 1002, "1a", 1, 3), 3),
		// 1003 is in Randstunde on Monday and Ganztag 16 on Wednesday.
		inPhase(booking(randstundeID, "Randstunde", 1003, "1a", 1), 3),
		inPhase(booking(ganztag16ID, "Ganztag bis 16", 1003, "1a", 3), 3),
		// 1004 is booked in Ganztag 16 only on Friday, not a roster day.
		inPhase(booking(ganztag16ID, "Ganztag bis 16", 1004, "1a", 5), 3),
		// Another phase and a holiday phase never bring a child back.
		inPhase(booking(otherPhase, "Andere Phase", 1004, "1a", 1, 3), 4),
		holiday,
		// A booking outside the class filter would not pass the source.
		inPhase(booking(ganztag16ID, "Ganztag bis 16", 1005, "2b", 1), 3),
	}
	filter := timetable.OfferingSourceFilter{OfferingIDs: []int64{randstundeID}, SchoolClasses: []string{"1a"}}
	changes := timetable.PreviewOfferingSource(roster, filter, bookings)

	alternatives := timetable.DroppedChildrenAlternatives(filter, bookings, changes)

	assert.Equal(t, []timetable.OfferingSourceAlternative{
		{OfferingID: ganztagID, OfferingName: "Ganztag bis 14.30", ChildCount: 1},
		{OfferingID: ganztag16ID, OfferingName: "Ganztag bis 16", ChildCount: 1},
	}, alternatives)
}

// TestDroppedChildrenAlternativesNone: without dropped children, or without a
// booking of the selection to fix the phase, nothing is suggested.
func TestDroppedChildrenAlternativesNone(t *testing.T) {
	t.Parallel()
	roster := classRoster([]int{1}, []int64{1001}, "1a")
	keep := booking(randstundeID, "Randstunde", 1001, "1a", 1)
	other := booking(ganztagID, "Ganztag", 1001, "1a", 1)

	t.Run("nobody drops", func(t *testing.T) {
		t.Parallel()
		filter := timetable.OfferingSourceFilter{OfferingIDs: []int64{randstundeID}}
		bookings := []timetable.OfferingWeekdayBooking{keep, other}
		changes := timetable.PreviewOfferingSource(roster, filter, bookings)
		assert.Empty(t, timetable.DroppedChildrenAlternatives(filter, bookings, changes))
	})

	t.Run("selection books nobody", func(t *testing.T) {
		t.Parallel()
		filter := timetable.OfferingSourceFilter{OfferingIDs: []int64{randstundeID}}
		bookings := []timetable.OfferingWeekdayBooking{other}
		changes := timetable.PreviewOfferingSource(roster, filter, bookings)
		assert.Empty(t, timetable.DroppedChildrenAlternatives(filter, bookings, changes))
	})
}
