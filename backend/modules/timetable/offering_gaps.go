package timetable

import (
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrOfferingGapNotFound refuses a decision on an offering the period shows
// no hint for: every booked child has a Regeltermin, or the offering has no
// bookings.
var ErrOfferingGapNotFound = errors.New("offering has no booked children without a Regeltermin")

// Offering without a matching Regeltermin (#5140): a child booked an
// offering on a school day, but no Regeltermin of the planning period takes
// it over on that date. Only Regeltermine the offering feeds count: the
// source rule with its Jahrgang or Klassen filter, or the offering's own
// link. The same children on a hand-kept list or in another offering's
// Regeltermin do not cover the booking; the roster review (#5103) handles
// hand-kept lists. A Regeltermin for Klasse 1/2 therefore never hides
// Klasse 3/4, and a series that only runs in November never hides a
// booking in December.
//
// The school may decide that the uncovered children need no Regeltermin of
// their own. Care Plan stores that decision per offering and planning
// period as the classes it covers; the hint returns once a child of another
// class is uncovered in that period.

// offeringGapWeekdays are the school days the check covers. A booking
// without weekday restriction books every day, but no school plans
// Regeltermine on Saturday or Sunday.
var offeringGapWeekdays = []int{1, 2, 3, 4, 5}

// OfferingGapScope is the stretch of the planning period the check looks
// at: from today, or the period start if later, to the period end, both
// inclusive.
type OfferingGapScope struct {
	From  calendar.Date
	Until calendar.Date
}

// OfferingGapTemplate is one Regeltermin of the period as the gap check sees
// it.
type OfferingGapTemplate struct {
	// Runs are the dated stretches the series runs on, one per schedule.
	Runs []OfferingGapRun
	// Sources are the offerings feeding the roster through the source rule,
	// with its Jahrgang or Klassen filter.
	Sources OfferingSourceFilter
	// LinkedOfferingIDs are offerings whose own timetable link points at the
	// series; they feed every booked child.
	LinkedOfferingIDs []int64
}

// OfferingGapRun is one weekly schedule from its first to its last
// occurrence in the period, both inclusive. Weeks between them that an A/B
// pattern or a closing day skips count as covered: the child still has its
// Regeltermin.
type OfferingGapRun struct {
	Weekday int
	From    calendar.Date
	Until   calendar.Date
}

// OfferingGap is one offering with booked children no Regeltermin takes
// over.
type OfferingGap struct {
	OfferingID   int64
	OfferingName string
	// StudentCount is the number of children with at least one uncovered
	// booked date.
	StudentCount int
	// SchoolClasses are the classes of those children, sorted; "" stands for
	// children without a class.
	SchoolClasses []string
	// Weekdays are the ISO weekdays with an uncovered booked date, ascending.
	Weekdays []int
	// Acknowledged is true when the school decided for this period that every
	// listed class needs no Regeltermin of the offering.
	Acknowledged bool
	// AcknowledgedClasses are the classes of the school's stored decision
	// for this period.
	AcknowledgedClasses []string
}

// DetectOfferingGaps lists, per offering, the booked children with a booked
// school day in scope that no Regeltermin fed by the offering covers,
// ordered by offering name.
func DetectOfferingGaps(scope OfferingGapScope, templates []OfferingGapTemplate, bookings []OfferingWeekdayBooking) []OfferingGap {
	byOffering := make(map[int64]*offeringGapCollector)
	order := make([]int64, 0)
	for _, booking := range bookings {
		uncovered := uncoveredWeekdays(scope, templates, booking)
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
	slices.SortFunc(gaps, compareOfferingGaps)
	return gaps
}

func compareOfferingGaps(left, right OfferingGap) int {
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
}

// uncoveredWeekdays are the school weekdays with a booked date in scope that
// no run of a Regeltermin fed by the booking's offering covers.
func uncoveredWeekdays(scope OfferingGapScope, templates []OfferingGapTemplate, booking OfferingWeekdayBooking) []int {
	runs := make([]OfferingGapRun, 0)
	for _, template := range templates {
		if template.feeds(booking) {
			runs = append(runs, template.Runs...)
		}
	}
	windows := booking.Windows
	if len(windows) == 0 {
		windows = []OfferingBookingWindow{{Weekdays: booking.Weekdays}}
	}
	uncovered := make([]int, 0)
	for _, weekday := range offeringGapWeekdays {
		if slices.ContainsFunc(windows, func(window OfferingBookingWindow) bool {
			return window.hasUncoveredDate(scope, weekday, runs)
		}) {
			uncovered = append(uncovered, weekday)
		}
	}
	return uncovered
}

// feeds reports whether the Regeltermin takes its children from the
// booking's offering: through the offering's own link, or through the source
// rule when the child passes its filter.
func (t OfferingGapTemplate) feeds(booking OfferingWeekdayBooking) bool {
	if slices.Contains(t.LinkedOfferingIDs, booking.OfferingID) {
		return true
	}
	return slices.Contains(t.Sources.OfferingIDs, booking.OfferingID) && bookingPassesSourceFilter(t.Sources, booking)
}

// hasUncoveredDate reports whether the window books a date of the weekday in
// scope that no run of that weekday covers.
func (w OfferingBookingWindow) hasUncoveredDate(scope OfferingGapScope, weekday int, runs []OfferingGapRun) bool {
	if !slices.Contains(w.Weekdays, weekday) {
		return false
	}
	first, last := scope.From, scope.Until
	if w.ValidFrom != nil && w.ValidFrom.After(first) {
		first = *w.ValidFrom
	}
	if w.ValidUntil != nil && w.ValidUntil.AddDays(-1).Before(last) {
		last = w.ValidUntil.AddDays(-1)
	}
	for date := NextWeekdayOnOrAfter(first, weekday); !date.After(last); date = date.AddDays(7) {
		if !slices.ContainsFunc(runs, func(run OfferingGapRun) bool {
			return run.Weekday == weekday && !date.Before(run.From) && !date.After(run.Until)
		}) {
			return true
		}
	}
	return false
}

// NextWeekdayOnOrAfter is the first date on or after d that falls on the ISO
// weekday (1 = Monday … 7 = Sunday).
func NextWeekdayOnOrAfter(d calendar.Date, weekday int) calendar.Date {
	return d.AddDays((weekday - int(d.Weekday()) + 14) % 7)
}

// PreviousWeekdayOnOrBefore is the last date on or before d that falls on
// the ISO weekday.
func PreviousWeekdayOnOrBefore(d calendar.Date, weekday int) calendar.Date {
	return d.AddDays(-((int(d.Weekday()) - weekday + 14) % 7))
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
