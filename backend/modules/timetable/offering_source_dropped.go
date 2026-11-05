package timetable

import (
	"maps"
	"slices"
)

// The filter that leaves out a child the selected offerings book.
const (
	DroppedExcludedBySchoolClass = "school_class"
	DroppedExcludedByGradeLevel  = "grade_level"
)

// DroppedChildBooking tells why a child drops from a Regeltermin when the
// offerings become its source.
type DroppedChildBooking struct {
	// Weekdays are the ISO weekdays, ascending, on which the selection books
	// the child within the Klassen or Jahrgang filter.
	Weekdays []int
	// ExcludedBy names the filter that leaves out a child the selection
	// books otherwise; empty when the filter plays no part.
	ExcludedBy string
}

// DroppedChildrenBookings explains every child the preview drops. Schule am
// Berg connected a Randstunde with all its offerings and still saw children
// drop: they were booked on Monday only, the Regeltermin runs on other days.
// The preview named them without that reason.
func DroppedChildrenBookings(filter OfferingSourceFilter, bookings []OfferingWeekdayBooking, changes []OfferingSourceWeekdayChange) map[int64]DroppedChildBooking {
	dropped := make(map[int64]DroppedChildBooking)
	for _, change := range changes {
		for _, studentID := range change.Removed {
			dropped[studentID] = DroppedChildBooking{Weekdays: []int{}}
		}
	}
	for _, booking := range bookings {
		reason, ok := dropped[booking.StudentID]
		if !ok || !slices.Contains(filter.OfferingIDs, booking.OfferingID) {
			continue
		}
		if bookingPassesSourceFilter(filter, booking) {
			reason.Weekdays = append(reason.Weekdays, booking.Weekdays...)
		} else if reason.ExcludedBy == "" {
			reason.ExcludedBy = excludingFilter(filter, booking)
		}
		dropped[booking.StudentID] = reason
	}
	for studentID := range maps.Keys(dropped) {
		reason := dropped[studentID]
		slices.Sort(reason.Weekdays)
		reason.Weekdays = slices.Compact(reason.Weekdays)
		if len(reason.Weekdays) > 0 {
			reason.ExcludedBy = ""
		}
		dropped[studentID] = reason
	}
	return dropped
}

// excludingFilter names the filter a booking fails: the Jahrgang when one is
// set and does not match, else the Klasse.
func excludingFilter(filter OfferingSourceFilter, booking OfferingWeekdayBooking) string {
	if len(filter.GradeLevels) > 0 && (booking.GradeLevel == nil || !slices.Contains(filter.GradeLevels, *booking.GradeLevel)) {
		return DroppedExcludedByGradeLevel
	}
	return DroppedExcludedBySchoolClass
}
