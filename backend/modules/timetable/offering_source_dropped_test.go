package timetable_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// TestDroppedChildrenBookings: Schule am Berg connected the Randstunde of 1c
// (Tuesday, Wednesday, Friday) with all its offerings and asked why children
// still dropped. Each dropped child now carries the weekdays the selection
// books it, or the filter that leaves it out.
func TestDroppedChildrenBookings(t *testing.T) {
	t.Parallel()
	const (
		monOnly    = int64(2001)
		monToThu   = int64(2002)
		notBooked  = int64(2003)
		otherClass = int64(2004)
		kept       = int64(2005)
		otherKept  = int64(2006)
	)
	roster := classRoster([]int{2, 3, 5}, []int64{monOnly, monToThu, notBooked, otherClass, kept}, "1c")
	roster.Weekdays[5] = []int64{monOnly, monToThu, notBooked, otherClass, kept, otherKept}
	bookings := []timetable.OfferingWeekdayBooking{
		// Booked on Monday in two offerings: listed once.
		booking(randstundeID, "Randstunde", monOnly, "1c", 1),
		booking(ganztagID, "Ganztag bis 14.30", monOnly, "1c", 1),
		booking(randstundeID, "Randstunde", monToThu, "1c", 1, 2, 3, 4),
		// Booked every day, but in 2a while the filter takes 1c.
		booking(randstundeID, "Randstunde", otherClass, "2a", 2, 3, 5),
		booking(randstundeID, "Randstunde", kept, "1c", 2, 3, 5),
		// An offering outside the selection does not count as booked.
		booking(int64(999), "Fußball-AG", notBooked, "1c", 5),
	}
	filter := timetable.OfferingSourceFilter{OfferingIDs: []int64{randstundeID, ganztagID}, SchoolClasses: []string{"1c"}}
	changes := timetable.PreviewOfferingSource(roster, filter, bookings)

	dropped := timetable.DroppedChildrenBookings(filter, bookings, changes)

	assert.Equal(t, map[int64]timetable.DroppedChildBooking{
		monOnly:    {Weekdays: []int{1}},
		monToThu:   {Weekdays: []int{1, 2, 3, 4}},
		notBooked:  {Weekdays: []int{}},
		otherClass: {Weekdays: []int{}, ExcludedBy: timetable.DroppedExcludedBySchoolClass},
		otherKept:  {Weekdays: []int{}},
	}, dropped)
}

// TestDroppedChildrenBookingsGradeFilter: a Jahrgang filter that leaves out
// a booked child names the Jahrgang, not the Klasse.
func TestDroppedChildrenBookingsGradeFilter(t *testing.T) {
	t.Parallel()
	grade := func(b timetable.OfferingWeekdayBooking, level int) timetable.OfferingWeekdayBooking {
		b.GradeLevel = &level
		return b
	}
	roster := classRoster([]int{1}, []int64{3001, 3002})
	bookings := []timetable.OfferingWeekdayBooking{
		grade(booking(randstundeID, "Randstunde", 3001, "1a", 1), 1),
		grade(booking(randstundeID, "Randstunde", 3002, "2a", 1), 2),
		booking(randstundeID, "Randstunde", 3002, "", 1),
	}
	filter := timetable.OfferingSourceFilter{OfferingIDs: []int64{randstundeID}, GradeLevels: []int{1}}
	changes := timetable.PreviewOfferingSource(roster, filter, bookings)

	dropped := timetable.DroppedChildrenBookings(filter, bookings, changes)

	assert.Equal(t, map[int64]timetable.DroppedChildBooking{
		3002: {Weekdays: []int{}, ExcludedBy: timetable.DroppedExcludedByGradeLevel},
	}, dropped)
}
