package timetable

import (
	"errors"
	"maps"
	"slices"
	"strings"
)

// ErrOfferingGapNotFound refuses a decision on an offering the period shows
// no hint for: every booked child has a Regeltermin, or the offering has no
// bookings.
var ErrOfferingGapNotFound = errors.New("offering has no booked children without a Regeltermin")

// Offering without a matching Regeltermin (#5140): a child booked an
// offering on a weekday, but no Regeltermin of the planning period takes it
// over on that day. Only Regeltermine the offering feeds count: the source
// rule with its Jahrgang or Klassen filter, or the offering's own link. The
// same children on a hand-kept list or in another offering's Regeltermin do
// not cover the booking; the roster review (#5103) handles hand-kept lists.
// A Regeltermin for Klasse 1/2 therefore never hides Klasse 3/4.
//
// The school may decide that the uncovered children need no Regeltermin of
// their own. Care Plan stores that decision on the offering as the classes
// it covers; the hint returns once a child of another class is uncovered.

// offeringGapWeekdays are the school days the check covers. A booking
// without weekday restriction books every day, but no school plans
// Regeltermine on Saturday or Sunday.
var offeringGapWeekdays = []int{1, 2, 3, 4, 5}

// OfferingGapTemplate is one Regeltermin of the period as the gap check sees
// it.
type OfferingGapTemplate struct {
	// Weekdays are the ISO weekdays the series still runs on from today.
	Weekdays []int
	// Sources are the offerings feeding the roster through the source rule,
	// with its Jahrgang or Klassen filter.
	Sources OfferingSourceFilter
	// LinkedOfferingIDs are offerings whose own timetable link points at the
	// series; they feed every booked child.
	LinkedOfferingIDs []int64
}

// OfferingGap is one offering with booked children no Regeltermin takes
// over.
type OfferingGap struct {
	OfferingID   int64
	OfferingName string
	// StudentCount is the number of children with at least one uncovered
	// weekday.
	StudentCount int
	// SchoolClasses are the classes of those children, sorted; "" stands for
	// children without a class.
	SchoolClasses []string
	// Weekdays are the uncovered ISO weekdays, ascending.
	Weekdays []int
	// Acknowledged is true when the school decided that every listed class
	// needs no Regeltermin of the offering.
	Acknowledged bool
	// AcknowledgedClasses are the classes of the school's stored decision.
	AcknowledgedClasses []string
}

// DetectOfferingGaps lists, per offering, the booked children no
// Regeltermin covers on a booked school day, ordered by offering name.
func DetectOfferingGaps(templates []OfferingGapTemplate, bookings []OfferingWeekdayBooking) []OfferingGap {
	byOffering := make(map[int64]*offeringGapCollector)
	order := make([]int64, 0)
	for _, booking := range bookings {
		uncovered := uncoveredWeekdays(templates, booking)
		if len(uncovered) == 0 {
			continue
		}
		collector := byOffering[booking.OfferingID]
		if collector == nil {
			collector = &offeringGapCollector{
				gap:      OfferingGap{OfferingID: booking.OfferingID, OfferingName: booking.OfferingName},
				students: make(map[int64]bool),
				classes:  make(map[string]bool),
				weekdays: make(map[int]bool),
				acked:    booking.TemplateGapAckClasses,
			}
			byOffering[booking.OfferingID] = collector
			order = append(order, booking.OfferingID)
		}
		collector.add(booking, uncovered)
	}
	gaps := make([]OfferingGap, 0, len(order))
	for _, offeringID := range order {
		gaps = append(gaps, byOffering[offeringID].result())
	}
	slices.SortFunc(gaps, func(left, right OfferingGap) int {
		if order := strings.Compare(strings.ToLower(left.OfferingName), strings.ToLower(right.OfferingName)); order != 0 {
			return order
		}
		switch {
		case left.OfferingID < right.OfferingID:
			return -1
		case left.OfferingID > right.OfferingID:
			return 1
		}
		return 0
	})
	return gaps
}

// uncoveredWeekdays are the booked school days of the booking no
// Regeltermin fed by its offering runs on.
func uncoveredWeekdays(templates []OfferingGapTemplate, booking OfferingWeekdayBooking) []int {
	uncovered := make([]int, 0)
	for _, weekday := range offeringGapWeekdays {
		if !slices.Contains(booking.Weekdays, weekday) {
			continue
		}
		if !slices.ContainsFunc(templates, func(template OfferingGapTemplate) bool {
			return template.covers(booking, weekday)
		}) {
			uncovered = append(uncovered, weekday)
		}
	}
	return uncovered
}

func (t OfferingGapTemplate) covers(booking OfferingWeekdayBooking, weekday int) bool {
	if !slices.Contains(t.Weekdays, weekday) {
		return false
	}
	if slices.Contains(t.LinkedOfferingIDs, booking.OfferingID) {
		return true
	}
	return slices.Contains(t.Sources.OfferingIDs, booking.OfferingID) && bookingPassesSourceFilter(t.Sources, booking)
}

type offeringGapCollector struct {
	gap      OfferingGap
	students map[int64]bool
	classes  map[string]bool
	weekdays map[int]bool
	acked    []string
}

func (c *offeringGapCollector) add(booking OfferingWeekdayBooking, uncovered []int) {
	c.students[booking.StudentID] = true
	c.classes[strings.TrimSpace(booking.SchoolClass)] = true
	for _, weekday := range uncovered {
		c.weekdays[weekday] = true
	}
}

func (c *offeringGapCollector) result() OfferingGap {
	gap := c.gap
	gap.StudentCount = len(c.students)
	gap.SchoolClasses = slices.Sorted(maps.Keys(c.classes))
	gap.Weekdays = slices.Sorted(maps.Keys(c.weekdays))
	gap.AcknowledgedClasses = append([]string{}, c.acked...)
	gap.Acknowledged = OfferingGapClassesAcknowledged(gap.SchoolClasses, c.acked)
	return gap
}

// OfferingGapClassesAcknowledged reports whether the school's decision
// covers every uncovered class; without a decision it covers none.
func OfferingGapClassesAcknowledged(classes, acknowledged []string) bool {
	if len(acknowledged) == 0 {
		return false
	}
	for _, class := range classes {
		if !slices.ContainsFunc(acknowledged, func(ack string) bool {
			return strings.EqualFold(strings.TrimSpace(ack), class)
		}) {
			return false
		}
	}
	return true
}
