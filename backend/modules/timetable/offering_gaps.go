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
// it over on that date. A Regeltermin the offering feeds covers the date
// whenever materialization creates an occurrence there: the source rule
// with its Jahrgang or Klassen filter, or the offering's own link. A
// Regeltermin for Klasse 1/2 therefore never hides Klasse 3/4, and a series
// that only runs in November never hides a booking in December.
//
// A date the offering's Regeltermine leave open is checked against the
// rest of the plan (#5343), so a class that the school plans through its
// Klasse, Jahrgang or by hand is not reported as unplanned:
//
//   - With clock times (the class's arrival from the Klassentag and the
//     offering's pickup time that day), the child needs care from arrival to
//     pickup. A class that arrives at the pickup time or later needs none
//     from this offering that day; the date is not reported. Otherwise any
//     Regeltermin that plans the child on that date at a time overlapping
//     the need covers it, whatever feeds it; one that starts only when the
//     need ends does not.
//   - Without both times, or for an offering outside school days (a holiday
//     phase or holiday care, where the Klassentag says nothing), the strict
//     day rule holds: only a Regeltermin without any offering source that
//     plans the child on that date and is of the offering's kind (#5103: a
//     care block for care, a Mensa list for lunch, an AG for courses) covers
//     it. Another offering's Regeltermin never does.
//
// Every date the offering's Regeltermine cover stays covered, so the check
// reports a subset of what #5140 reported; it never adds a child or a day.
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
// inclusive. ClosedDays (holidays and closing days) carry no care and are
// not checked.
type OfferingGapScope struct {
	From       calendar.Date
	Until      calendar.Date
	ClosedDays map[calendar.Date]bool
	// PeriodID is the planning period; enrollment rows pinned to another
	// period plan nobody here. 0 skips that check.
	PeriodID int64
	// Arrivals is the Klassentag of the booked classes.
	Arrivals OfferingGapArrivals
}

// OfferingGapArrivals is the Klassentag as the gap check reads it: when a
// class reaches the OGS, in minutes after midnight, keyed by the trimmed,
// lower-case class. A class-wide change on a single date wins over the
// weekly time.
type OfferingGapArrivals struct {
	Weekly map[string]map[int]int
	Dated  map[string]map[calendar.Date]int
}

// on is the class's arrival on date; false when the Klassentag has none.
func (a OfferingGapArrivals) on(class string, date calendar.Date) (int, bool) {
	key := OfferingGapClassKey(class)
	if key == "" {
		return 0, false
	}
	if minute, ok := a.Dated[key][date]; ok {
		return minute, true
	}
	minute, ok := a.Weekly[key][calendar.ISOWeekday(date)]
	return minute, ok
}

// OfferingGapClassKey is the class as the Klassentag keys it.
func OfferingGapClassKey(class string) string {
	return strings.ToLower(strings.TrimSpace(class))
}

// OfferingGapTemplate is one Regeltermin of the period as the gap check sees
// it.
type OfferingGapTemplate struct {
	// Occurrences are the dates materialization creates an occurrence on,
	// as MaterializationCapability.ProjectOccurrences projects them: the
	// same period selection, A/B anchor and skipped days.
	Occurrences map[calendar.Date]bool
	// Sources are the offerings feeding the roster through the source rule,
	// with its Jahrgang or Klassen filter.
	Sources OfferingSourceFilter
	// LinkedOfferingIDs are offerings whose own timetable link points at the
	// series; they feed every booked child.
	LinkedOfferingIDs []int64
	// HasOfferingSource is true when an offering feeds the Regeltermin, also
	// one the school switched off; such a Regeltermin never stands in for
	// another offering under the strict day rule.
	HasOfferingSource bool
	// Kind names the offerings it stands in for under the strict day rule.
	Kind RosterOfferingKind
	// Times are the clock windows of its occurrences per date, as
	// ProjectOccurrences reports them: one per schedule materialized there,
	// an exception's moved time applied.
	Times map[calendar.Date][]OccurrenceClock
	// Roster is who it plans on which date; nil plans nobody.
	Roster *TemplatePlannedRoster
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
	// NewSeriesClasses is the Klassenfilter of a new Regeltermin started
	// from the hint: the listed classes, or none when a child without a class
	// is listed, since a Klassenfilter never admits one.
	NewSeriesClasses []string
	// NewSeriesPlansCovered is true when that Regeltermin, fed by the
	// offering on the listed weekdays, would also plan a booked child on a
	// date that needs no Regeltermin of its own: covered already, or the
	// class arrives only at the pickup time. The hint then offers no
	// creation, so no second list arises.
	NewSeriesPlansCovered bool
}

// DetectOfferingGaps lists, per offering, the booked children with a booked
// school day in scope that no Regeltermin covers, ordered by offering name.
func DetectOfferingGaps(scope OfferingGapScope, templates []OfferingGapTemplate, bookings []OfferingWeekdayBooking) []OfferingGap {
	plan := newOfferingGapPlan(scope, templates)
	byOffering := make(map[int64]*offeringGapCollector)
	order := make([]int64, 0)
	for _, booking := range bookings {
		uncovered := plan.uncoveredWeekdays(booking)
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
		gap := byOffering[offeringID].result()
		gap.NewSeriesPlansCovered = plan.newSeriesPlansCovered(gap, bookings)
		gaps = append(gaps, gap)
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

// feeds reports whether the Regeltermin takes its children from the
// booking's offering: through the offering's own link, or through the source
// rule when the child passes its filter.
func (t OfferingGapTemplate) feeds(booking OfferingWeekdayBooking) bool {
	if slices.Contains(t.LinkedOfferingIDs, booking.OfferingID) {
		return true
	}
	return slices.Contains(t.Sources.OfferingIDs, booking.OfferingID) && bookingPassesSourceFilter(t.Sources, booking)
}

// bookedOpenDates calls visit for every open date of the weekday in scope
// the window books, until visit returns true; it reports whether one did.
func (w OfferingBookingWindow) bookedOpenDates(scope OfferingGapScope, weekday int, visit func(calendar.Date) bool) bool {
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
		if !scope.ClosedDays[date] && visit(date) {
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
	gap.NewSeriesClasses = []string{}
	if !c.classes[""] {
		gap.NewSeriesClasses = slices.Clone(gap.SchoolClasses)
	}
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
