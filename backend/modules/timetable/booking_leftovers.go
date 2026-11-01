package timetable

import (
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Booking leftovers (#5102, ADR 0128 addendum of 2026-11-01): the save
// response of a booking change names the hand-maintained Regeltermine that
// keep the child on a weekday no remaining booking carries. That list is
// gone after a reload. This derivation finds the same leftovers again from
// what is stored: a booking link that ended on a weekday, a hand roster row
// that still plans the child there after that day, and no booking in force
// from then on that carries the Regeltermin. Nothing is stored for the
// hint; the school's "Bleibt so" sits on the hand row.

// BookingLeftoverScope is the first day a removal may change (tomorrow)
// and the planning period the templates belong to.
type BookingLeftoverScope struct {
	Tomorrow calendar.Date
	PeriodID int64
}

// BookingLeftoverTemplate is one Regeltermin of the period with the latest
// end of its occurrences per ISO weekday ("HH:MM" or "HH:MM:SS", empty when
// unknown) and its roster rows and target members.
type BookingLeftoverTemplate struct {
	ID          int64
	Name        string
	Kind        RosterOfferingKind
	WeekdayEnds map[int]string
	Roster      TemplatePlannedRoster
}

// BookingLeftover names a child a hand row keeps on a Regeltermin on
// weekdays whose booking ended without another one that carries it.
type BookingLeftover struct {
	TemplateID   int64
	TemplateName string
	StudentID    int64
	// Weekdays are the ISO weekdays, ascending.
	Weekdays []int
	// BookingEndedOn is the latest day a booking of these weekdays ended
	// (exclusive end); "Bleibt so" records it.
	BookingEndedOn calendar.Date
	// From is the first day a removal changes: the booking end, tomorrow at
	// the earliest.
	From calendar.Date
	// Kept is the school's decision to keep the child planned.
	Kept bool
}

type leftoverKey struct {
	templateID int64
	studentID  int64
	weekday    int
}

type leftoverDay struct {
	endedOn calendar.Date
	kept    bool
}

// DetectBookingLeftovers lists the leftovers per Regeltermin and child,
// ordered by Regeltermin name, then child, then start. A child the
// Regeltermin plans through a Klasse, Jahrgang or Gruppe target is left out:
// removing the hand row would not take it off. Bookings of a holiday phase
// stand behind no Regeltermin and are ignored.
func DetectBookingLeftovers(scope BookingLeftoverScope, templates []BookingLeftoverTemplate, bookings []OfferingWeekdayBooking) []BookingLeftover {
	byStudent := make(map[int64][]OfferingWeekdayBooking)
	for _, booking := range bookings {
		if !booking.HolidayPhase {
			byStudent[booking.StudentID] = append(byStudent[booking.StudentID], booking)
		}
	}
	days := make(map[leftoverKey]leftoverDay)
	for _, template := range templates {
		for _, entry := range template.Roster.Entries {
			if entry.FromBooking || slices.Contains(template.Roster.TargetStudentIDs, entry.StudentID) {
				continue
			}
			for weekday, end := range template.WeekdayEnds {
				addLeftoverDay(days, scope, template, entry, weekday, end, byStudent[entry.StudentID])
			}
		}
	}
	return groupLeftoverDays(scope, templates, days)
}

// addLeftoverDay records the weekday when a booking of the child on it ended
// and the row still plans the child there afterwards without a carrying
// booking. The latest such end counts; the day is kept only when every row
// planning it carries a decision reaching that end.
func addLeftoverDay(
	days map[leftoverKey]leftoverDay, scope BookingLeftoverScope, template BookingLeftoverTemplate,
	entry TemplateRosterEntry, weekday int, end string, bookings []OfferingWeekdayBooking,
) {
	key := leftoverKey{templateID: template.ID, studentID: entry.StudentID, weekday: weekday}
	for _, endedOn := range endedBookingDays(bookings, weekday) {
		from := max(endedOn, scope.Tomorrow)
		if !entryPlansWeekdayFrom(entry, weekday, from, scope.PeriodID) ||
			bookingsCarry(bookings, template.Kind, weekday, end, from, endedOn) {
			continue
		}
		kept := entry.KeptWithoutBookingThrough != nil && !entry.KeptWithoutBookingThrough.Before(endedOn)
		current, found := days[key]
		switch {
		case !found || current.endedOn.Before(endedOn):
			days[key] = leftoverDay{endedOn: endedOn, kept: kept}
		case current.endedOn == endedOn:
			days[key] = leftoverDay{endedOn: endedOn, kept: current.kept && kept}
		}
	}
}

// endedBookingDays lists the days on which a change or a withdrawal dropped
// an offering from the weekday: a link of it ended and no link of the same
// offering plans the weekday from that day on. A rebooking of the same
// offering drops nothing, as in the save response.
func endedBookingDays(bookings []OfferingWeekdayBooking, weekday int) []calendar.Date {
	var result []calendar.Date
	for _, booking := range bookings {
		for _, window := range booking.Windows {
			if window.Ended && window.ValidUntil != nil && slices.Contains(window.Weekdays, weekday) &&
				len(bookingsInForce([]OfferingWeekdayBooking{booking}, weekday, *window.ValidUntil)) == 0 {
				result = append(result, *window.ValidUntil)
			}
		}
	}
	return result
}

// entryPlansWeekdayFrom reports whether the row plans its child on the
// weekday on some day from the date on, in the period.
func entryPlansWeekdayFrom(entry TemplateRosterEntry, weekday int, from calendar.Date, periodID int64) bool {
	switch {
	case entry.ValidUntil != nil && !entry.ValidUntil.After(from),
		entry.CalendarPeriodID != nil && *entry.CalendarPeriodID != periodID,
		entry.Weekday != nil && *entry.Weekday != weekday:
		return false
	}
	return len(entry.SelectedWeekdays) == 0 || slices.Contains(entry.SelectedWeekdays, weekday)
}

// bookingsCarry applies ADR 0128's carry rule to the bookings in force on
// the date: the times decide when the Regeltermin has an end and every such
// booking has a pickup time that weekday; a fitting booking picked up no
// earlier than the end carries. Otherwise the day counts as lost only when
// the booking end left fewer offerings on the weekday than before. A day
// without any booking is lost.
func bookingsCarry(bookings []OfferingWeekdayBooking, kind RosterOfferingKind, weekday int, end string, from, endedOn calendar.Date) bool {
	inForce := bookingsInForce(bookings, weekday, from)
	if len(inForce) == 0 {
		return false
	}
	endClock, endKnown := wallClock(end)
	if endKnown && everyBookingHasPickup(inForce, weekday) {
		for _, booking := range inForce {
			pickup, _ := wallClock(booking.PickupTimes[weekday])
			if kind.admits(booking) && !pickup.Before(endClock) {
				return true
			}
		}
		return false
	}
	return len(bookingsInForce(bookings, weekday, endedOn.AddDays(-1))) <= len(bookingsInForce(bookings, weekday, endedOn))
}

// bookingsInForce keeps the bookings with a link that plans the weekday on
// the date, one per offering.
func bookingsInForce(bookings []OfferingWeekdayBooking, weekday int, on calendar.Date) []OfferingWeekdayBooking {
	result := make([]OfferingWeekdayBooking, 0, len(bookings))
	for _, booking := range bookings {
		for _, window := range booking.Windows {
			if slices.Contains(window.Weekdays, weekday) &&
				(window.ValidFrom == nil || !window.ValidFrom.After(on)) &&
				(window.ValidUntil == nil || window.ValidUntil.After(on)) {
				result = append(result, booking)
				break
			}
		}
	}
	return result
}

func everyBookingHasPickup(bookings []OfferingWeekdayBooking, weekday int) bool {
	for _, booking := range bookings {
		if _, ok := wallClock(booking.PickupTimes[weekday]); !ok {
			return false
		}
	}
	return true
}

// wallClock parses "HH:MM" or "HH:MM:SS".
func wallClock(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"15:04:05", "15:04"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// groupLeftoverDays merges the weekdays of one Regeltermin and child that
// start on the same day and share the decision.
func groupLeftoverDays(scope BookingLeftoverScope, templates []BookingLeftoverTemplate, days map[leftoverKey]leftoverDay) []BookingLeftover {
	names := make(map[int64]string, len(templates))
	for _, template := range templates {
		names[template.ID] = template.Name
	}
	type groupKey struct {
		templateID, studentID int64
		from                  calendar.Date
		kept                  bool
	}
	groups := make(map[groupKey]*BookingLeftover)
	for key, day := range days {
		from := max(day.endedOn, scope.Tomorrow)
		group := groupKey{templateID: key.templateID, studentID: key.studentID, from: from, kept: day.kept}
		leftover := groups[group]
		if leftover == nil {
			leftover = &BookingLeftover{TemplateID: key.templateID, TemplateName: names[key.templateID],
				StudentID: key.studentID, From: from, Kept: day.kept}
			groups[group] = leftover
		}
		leftover.Weekdays = append(leftover.Weekdays, key.weekday)
		leftover.BookingEndedOn = max(leftover.BookingEndedOn, day.endedOn)
	}
	result := make([]BookingLeftover, 0, len(groups))
	for _, leftover := range groups {
		slices.Sort(leftover.Weekdays)
		result = append(result, *leftover)
	}
	slices.SortFunc(result, func(a, b BookingLeftover) int {
		if c := strings.Compare(strings.ToLower(a.TemplateName), strings.ToLower(b.TemplateName)); c != 0 {
			return c
		}
		if a.TemplateID != b.TemplateID {
			return int(a.TemplateID - b.TemplateID)
		}
		if a.StudentID != b.StudentID {
			return int(a.StudentID - b.StudentID)
		}
		return a.From.Compare(b.From)
	})
	return result
}
