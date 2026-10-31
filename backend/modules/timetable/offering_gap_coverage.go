package timetable

import (
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// offeringGapPlan answers, for one booking and date, whether the plan
// covers it (#5140, #5343; the rule is on DetectOfferingGaps).
type offeringGapPlan struct {
	scope     OfferingGapScope
	templates []offeringGapIndexedTemplate
}

// offeringGapIndexedTemplate is a Regeltermin with its roster keyed by
// child, read once per check.
type offeringGapIndexedTemplate struct {
	OfferingGapTemplate
	targets map[int64]bool
	entries map[int64][]TemplateRosterEntry
}

func newOfferingGapPlan(scope OfferingGapScope, templates []OfferingGapTemplate) offeringGapPlan {
	indexed := make([]offeringGapIndexedTemplate, 0, len(templates))
	for _, template := range templates {
		entry := offeringGapIndexedTemplate{OfferingGapTemplate: template, targets: map[int64]bool{}, entries: map[int64][]TemplateRosterEntry{}}
		if template.Roster != nil {
			for _, studentID := range template.Roster.TargetStudentIDs {
				entry.targets[studentID] = true
			}
			for _, row := range template.Roster.Entries {
				entry.entries[row.StudentID] = append(entry.entries[row.StudentID], row)
			}
		}
		indexed = append(indexed, entry)
	}
	return offeringGapPlan{scope: scope, templates: indexed}
}

// offeringGapNeed is the stretch of a day a child needs care from the
// offering, from the class's arrival to the offering's pickup, in minutes
// after midnight; empty when the class arrives at the pickup or later.
type offeringGapNeed struct {
	from, until int
}

func (n offeringGapNeed) empty() bool { return n.from >= n.until }

// uncoveredWeekdays are the school weekdays with a booked open date in scope
// the plan does not cover.
func (p offeringGapPlan) uncoveredWeekdays(booking OfferingWeekdayBooking) []int {
	uncovered := make([]int, 0)
	for _, weekday := range offeringGapWeekdays {
		if p.hasUncoveredDate(booking, weekday) {
			uncovered = append(uncovered, weekday)
		}
	}
	return uncovered
}

func (p offeringGapPlan) hasUncoveredDate(booking OfferingWeekdayBooking, weekday int) bool {
	return slices.ContainsFunc(bookingWindows(booking), func(window OfferingBookingWindow) bool {
		return window.bookedOpenDates(p.scope, weekday, func(date calendar.Date) bool {
			return !p.covers(booking, date)
		})
	})
}

// bookingWindows are the booking's links; without any, its weekdays hold for
// the whole period.
func bookingWindows(booking OfferingWeekdayBooking) []OfferingBookingWindow {
	if len(booking.Windows) == 0 {
		return []OfferingBookingWindow{{Weekdays: booking.Weekdays}}
	}
	return booking.Windows
}

// covers reports whether the booked date needs no Regeltermin of its own: a
// Regeltermin of the offering has an occurrence there, the class arrives
// only at the pickup, or the Regeltermine that plan the child that day cover
// the whole stretch from arrival to pickup together.
func (p offeringGapPlan) covers(booking OfferingWeekdayBooking, date calendar.Date) bool {
	if slices.ContainsFunc(p.templates, func(t offeringGapIndexedTemplate) bool { return t.Occurrences[date] && t.feeds(booking) }) {
		return true
	}
	need, known := p.need(booking, date)
	if !known {
		return slices.ContainsFunc(p.templates, func(t offeringGapIndexedTemplate) bool {
			return !t.HasOfferingSource && t.Kind.standsInFor(booking) && len(t.plannedClocks(booking.StudentID, date)) > 0
		})
	}
	if need.empty() {
		return true
	}
	clocks := make([]OccurrenceClock, 0)
	for _, t := range p.templates {
		clocks = append(clocks, t.plannedClocks(booking.StudentID, date)...)
	}
	return need.coveredBy(clocks)
}

// need is the child's care stretch on date; false when the offering does
// not follow the school days or the class's arrival or the offering's
// pickup is unknown that day.
func (p offeringGapPlan) need(booking OfferingWeekdayBooking, date calendar.Date) (offeringGapNeed, bool) {
	if !booking.ClassDayApplies {
		return offeringGapNeed{}, false
	}
	until, ok := ParseOfferingGapClock(booking.PickupTimes[calendar.ISOWeekday(date)])
	if !ok {
		return offeringGapNeed{}, false
	}
	from, ok := p.scope.Arrivals.on(booking.SchoolClass, date)
	if !ok {
		return offeringGapNeed{}, false
	}
	return offeringGapNeed{from: from, until: until}, true
}

// standsInFor is the kind rule of #5103 for the strict day rule; a
// Regeltermin of no known kind stands in for no offering.
func (kind RosterOfferingKind) standsInFor(booking OfferingWeekdayBooking) bool {
	return kind != RosterOfferingAny && kind.admits(booking)
}

// plannedClocks are the Regeltermin's occurrences on date that have the
// child on them, with TemplatePlannedRoster.Plans' rule: each occurrence is
// filled from the period materialization selects for it, so a row pinned to
// an overlapping period does not count there. A date without a projected
// occurrence plans nobody.
func (t offeringGapIndexedTemplate) plannedClocks(studentID int64, date calendar.Date) []OccurrenceClock {
	if !t.Occurrences[date] {
		return nil
	}
	planned := make([]OccurrenceClock, 0, len(t.Times[date]))
	for _, clock := range t.Times[date] {
		if t.targets[studentID] || slices.ContainsFunc(t.entries[studentID], func(entry TemplateRosterEntry) bool {
			return entry.appliesOn(date, clock.PeriodID)
		}) {
			planned = append(planned, clock)
		}
	}
	return planned
}

// coveredBy reports whether the occurrences cover the need without a gap
// (#5343): their union contains every minute from arrival to pickup. All
// windows are half-open wall-clock minutes, [start, end), so 11:45–12:45 and
// 12:45–16:00 meet seamlessly, while a mere overlap such as a Lernzeit from
// 13:00 to 14:00 leaves 11:45–13:00 and 14:00–16:00 open. A window without a
// positive length or a known time covers nothing.
func (n offeringGapNeed) coveredBy(clocks []OccurrenceClock) bool {
	sorted := slices.Clone(clocks)
	slices.SortFunc(sorted, func(a, b OccurrenceClock) int { return a.StartMinute - b.StartMinute })
	reached := n.from
	for _, clock := range sorted {
		if clock.StartMinute >= clock.EndMinute || clock.EndMinute <= reached {
			continue
		}
		if clock.StartMinute > reached {
			return false
		}
		reached = clock.EndMinute
		if reached >= n.until {
			return true
		}
	}
	return reached >= n.until
}

// newSeriesPlansCovered reports whether a new Regeltermin started from the
// gap would duplicate existing planning or plan a child who needs none. It
// takes the offering's children of NewSeriesClasses (every child without a filter)
// on the gap's weekdays.
func (p offeringGapPlan) newSeriesPlansCovered(gap OfferingGap, bookings []OfferingWeekdayBooking) bool {
	filter := OfferingSourceFilter{OfferingIDs: []int64{gap.OfferingID}, SchoolClasses: gap.NewSeriesClasses}
	return slices.ContainsFunc(bookings, func(booking OfferingWeekdayBooking) bool {
		if booking.OfferingID != gap.OfferingID || !bookingPassesSourceFilter(filter, booking) {
			return false
		}
		return slices.ContainsFunc(gap.Weekdays, func(weekday int) bool {
			return slices.ContainsFunc(bookingWindows(booking), func(window OfferingBookingWindow) bool {
				return window.bookedOpenDates(p.scope, weekday, func(date calendar.Date) bool {
					return p.covers(booking, date) || p.plansDuringNeed(booking, date)
				})
			})
		})
	})
}

// plansDuringNeed keeps partial planning separate from full coverage: it
// leaves the gap visible but prevents a second list over an existing part
// of the care stretch. Unknown times and touching boundaries do not overlap.
func (p offeringGapPlan) plansDuringNeed(booking OfferingWeekdayBooking, date calendar.Date) bool {
	need, known := p.need(booking, date)
	if !known || need.empty() {
		return false
	}
	return slices.ContainsFunc(p.templates, func(t offeringGapIndexedTemplate) bool {
		return slices.ContainsFunc(t.plannedClocks(booking.StudentID, date), func(clock OccurrenceClock) bool {
			return clock.StartMinute < clock.EndMinute && clock.StartMinute < need.until && clock.EndMinute > need.from
		})
	})
}

// ParseOfferingGapClock reads "HH:MM" or "HH:MM:SS" as minutes after
// midnight; false for anything else.
func ParseOfferingGapClock(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}
	return hour*60 + minute, true
}
