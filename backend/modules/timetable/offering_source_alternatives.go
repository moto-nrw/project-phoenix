package timetable

import (
	"slices"
	"strings"
)

// OfferingSourceAlternative is an offering outside the source selection that
// books children the selection drops, on a day they drop.
type OfferingSourceAlternative struct {
	OfferingID   int64
	OfferingName string
	// ChildCount counts each dropped child once, however many days it covers.
	ChildCount int
}

// DroppedChildrenAlternatives names the offerings that would keep children
// the preview drops. Before, the preview listed them without a reason, and a
// school could not see that a second offering of the same phase was missing
// from the selection. Only offerings of the selection's phase qualify, as a
// combined source must stay within one phase, never a holiday phase, only of
// the selection's kind (care, lunch or course: a Randstunde is never kept by
// the lunch or an AG), and only bookings the selection's Klassen or Jahrgang
// filter admits. Ordered by child count, then name.
func DroppedChildrenAlternatives(filter OfferingSourceFilter, bookings []OfferingWeekdayBooking, changes []OfferingSourceWeekdayChange) []OfferingSourceAlternative {
	dropped := droppedSlots(changes)
	phases := selectionPhases(filter, bookings)
	if len(dropped) == 0 || len(phases) == 0 {
		return []OfferingSourceAlternative{}
	}
	kind := selectionKind(filter, bookings)
	names := make(map[int64]string)
	children := make(map[int64]map[int64]bool)
	for _, booking := range bookings {
		if slices.Contains(filter.OfferingIDs, booking.OfferingID) || booking.HolidayPhase ||
			!phases[booking.PhaseID] || !kind.admits(booking) || !bookingPassesSourceFilter(filter, booking) ||
			!keepsDroppedSlot(booking, dropped) {
			continue
		}
		if children[booking.OfferingID] == nil {
			children[booking.OfferingID] = make(map[int64]bool)
			names[booking.OfferingID] = booking.OfferingName
		}
		children[booking.OfferingID][booking.StudentID] = true
	}
	alternatives := make([]OfferingSourceAlternative, 0, len(children))
	for offeringID, students := range children {
		alternatives = append(alternatives, OfferingSourceAlternative{
			OfferingID: offeringID, OfferingName: names[offeringID], ChildCount: len(students),
		})
	}
	slices.SortFunc(alternatives, func(left, right OfferingSourceAlternative) int {
		if left.ChildCount != right.ChildCount {
			return right.ChildCount - left.ChildCount
		}
		if byName := strings.Compare(left.OfferingName, right.OfferingName); byName != 0 {
			return byName
		}
		return int(left.OfferingID - right.OfferingID)
	})
	return alternatives
}

func droppedSlots(changes []OfferingSourceWeekdayChange) map[rosterSlot]bool {
	dropped := make(map[rosterSlot]bool)
	for _, change := range changes {
		for _, studentID := range change.Removed {
			dropped[rosterSlot{studentID: studentID, weekday: change.Weekday}] = true
		}
	}
	return dropped
}

// keepsDroppedSlot reports whether the booking covers a dropped child on a
// day it drops.
func keepsDroppedSlot(booking OfferingWeekdayBooking, dropped map[rosterSlot]bool) bool {
	return slices.ContainsFunc(booking.Weekdays, func(weekday int) bool {
		return dropped[rosterSlot{studentID: booking.StudentID, weekday: weekday}]
	})
}

// selectionPhases are the enrollment phases of the selected offerings, as
// their bookings carry them.
func selectionPhases(filter OfferingSourceFilter, bookings []OfferingWeekdayBooking) map[int64]bool {
	phases := make(map[int64]bool)
	for _, booking := range bookings {
		if slices.Contains(filter.OfferingIDs, booking.OfferingID) {
			phases[booking.PhaseID] = true
		}
	}
	return phases
}

// selectionKind is the kind of offering the selection stands for: care when
// one selected offering counts as care, else lunch when one includes lunch,
// else a course.
func selectionKind(filter OfferingSourceFilter, bookings []OfferingWeekdayBooking) RosterOfferingKind {
	kind := RosterOfferingCourse
	for _, booking := range bookings {
		if !slices.Contains(filter.OfferingIDs, booking.OfferingID) {
			continue
		}
		if booking.CountsAsCare {
			return RosterOfferingCare
		}
		if booking.IncludesLunch {
			kind = RosterOfferingLunch
		}
	}
	return kind
}
