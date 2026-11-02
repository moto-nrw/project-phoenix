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

// BookingLeftoverScope is the first day a removal may change (tomorrow).
type BookingLeftoverScope struct {
	Tomorrow calendar.Date
}

// BookingLeftoverOccurrence is one occurrence materialization plans for a
// Regeltermin (ProjectOccurrences): its date, latest end ("HH:MM", empty
// when unknown) and the planning period whose roster rows fill it.
type BookingLeftoverOccurrence struct {
	Date     calendar.Date
	End      string
	PeriodID int64
}

// BookingLeftoverTemplate is one Regeltermin of the period with its planned
// occurrences from tomorrow on, ascending, and its roster rows and target
// members.
type BookingLeftoverTemplate struct {
	ID          int64
	Name        string
	Kind        RosterOfferingKind
	Occurrences []BookingLeftoverOccurrence
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
	// without a carrying one (exclusive end); "Bleibt so" records it, so the
	// decision covers every gap the hint names.
	BookingEndedOn calendar.Date
	// From is the first planned day without a carrying booking, tomorrow at
	// the earliest; "Entfernen" starts there.
	From calendar.Date
	// Until ends the gap (exclusive, #5497): a later booking carries the
	// Regeltermin again from this day on, so "Entfernen" keeps the child
	// planned from there. Nil when no later booking carries it.
	Until *calendar.Date
	// Kept is the school's decision to keep the child planned.
	Kept bool
}

type leftoverKey struct {
	templateID int64
	studentID  int64
	weekday    int
}

// leftoverDay is one gap of one weekday: start is the later of its booking
// end and tomorrow, from its first planned day, until the day a later booking
// carries the weekday again (nil: none does).
type leftoverDay struct {
	start, from, endedOn calendar.Date
	until                *calendar.Date
	kept                 bool
}

type leftoverGroupKey struct {
	templateID, studentID int64
	start, until          calendar.Date
	kept                  bool
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
	days := make(map[leftoverKey][]leftoverDay)
	for _, template := range templates {
		addTemplateLeftoverDays(days, scope, template, byStudent)
	}
	return groupLeftoverDays(templates, days)
}

// addTemplateLeftoverDays adds the weekdays each hand row of the
// Regeltermin leaves without a carrying booking.
func addTemplateLeftoverDays(
	days map[leftoverKey][]leftoverDay, scope BookingLeftoverScope, template BookingLeftoverTemplate,
	byStudent map[int64][]OfferingWeekdayBooking,
) {
	weekdays := occurrenceWeekdays(template.Occurrences)
	for _, entry := range template.Roster.Entries {
		if entry.FromBooking || slices.Contains(template.Roster.TargetStudentIDs, entry.StudentID) {
			continue
		}
		for _, weekday := range weekdays {
			key := leftoverKey{templateID: template.ID, studentID: entry.StudentID, weekday: weekday}
			for _, day := range entryLeftoverDays(scope, template, entry, weekday, byStudent[entry.StudentID]) {
				days[key] = mergeLeftoverDay(days[key], day)
			}
		}
	}
}

func occurrenceWeekdays(occurrences []BookingLeftoverOccurrence) []int {
	weekdays := make([]int, 0, 7)
	for _, occurrence := range occurrences {
		weekdays = append(weekdays, calendar.ISOWeekday(occurrence.Date))
	}
	slices.Sort(weekdays)
	return slices.Compact(weekdays)
}

// entryLeftoverDays looks at every day a booking of the child dropped the
// weekday: from the later of that day and tomorrow, the row's first planned
// day of each uncovered interval starts a gap, which lasts until a later
// planned day is carried again. A later rebooking can itself end, creating a
// separate gap for the same weekday.
func entryLeftoverDays(
	scope BookingLeftoverScope, template BookingLeftoverTemplate, entry TemplateRosterEntry,
	weekday int, bookings []OfferingWeekdayBooking,
) []leftoverDay {
	result := make([]leftoverDay, 0)
	for _, endedOn := range endedBookingDays(bookings, weekday) {
		start := max(endedOn, scope.Tomorrow)
		first, end, planned := firstPlannedDay(template, entry, weekday, start)
		if !planned || bookingsCarry(bookings, template.Kind, weekday, end, first, endedOn) {
			continue
		}
		decided, hasDecision := entry.KeptWithoutBooking[weekday]
		result = mergeLeftoverDay(result, leftoverDay{
			start: start, from: first, endedOn: endedOn,
			until: gapUntil(template, entry, weekday, first, endedOn, bookings),
			kept:  hasDecision && !decided.Before(endedOn),
		})
	}
	return result
}

// mergeLeftoverDay joins descriptions of the same gap. Booking changes before
// tomorrow collapse into its current gap, but distinct future starts stay
// separate. The earliest end is safe for every row the removal narrows.
func mergeLeftoverDay(days []leftoverDay, next leftoverDay) []leftoverDay {
	for index, current := range days {
		if current.start != next.start {
			continue
		}
		if next.from.Before(current.from) {
			current.from = next.from
		}
		current.endedOn = max(current.endedOn, next.endedOn)
		current.until = earlierUntil(current.until, next.until)
		current.kept = current.kept && next.kept
		days[index] = current
		return days
	}
	return append(days, next)
}

// earlierUntil is the earlier of two gap ends; nil is open and comes last. A
// removal that ends earlier takes nothing a booking carries.
func earlierUntil(a, b *calendar.Date) *calendar.Date {
	if a == nil || (b != nil && b.Before(*a)) {
		return b
	}
	return a
}

// gapUntil ends the gap that starts on the planned day first (#5497): the
// row's next planned day of the weekday that a booking carries again ends
// it, on the day the latest booking in force then started, but after the
// last planned day without a carrying booking. Nil when no later planned day
// is carried: the gap stays open.
func gapUntil(
	template BookingLeftoverTemplate, entry TemplateRosterEntry, weekday int, first, endedOn calendar.Date,
	bookings []OfferingWeekdayBooking,
) *calendar.Date {
	lastLost := first
	for _, occurrence := range template.Occurrences {
		if !occurrence.Date.After(first) || calendar.ISOWeekday(occurrence.Date) != weekday ||
			!entry.appliesOn(occurrence.Date, occurrence.PeriodID) {
			continue
		}
		if !bookingsCarry(bookings, template.Kind, weekday, occurrence.End, occurrence.Date, endedOn) {
			lastLost = occurrence.Date
			continue
		}
		until := max(latestBookingStart(bookings, template.Kind, weekday, occurrence.End, occurrence.Date), lastLost.AddDays(1))
		return &until
	}
	return nil
}

// latestBookingStart is the latest start of a link that carries the
// Regeltermin on the day it is carried again. When pickup times decide, a
// booking of another kind or with an early pickup cannot move that boundary.
// Where the times do not decide, the carry rule is the weekday's net booking
// count, so every booking in force belongs to that calculation.
func latestBookingStart(
	bookings []OfferingWeekdayBooking, kind RosterOfferingKind, weekday int, end string, on calendar.Date,
) calendar.Date {
	inForce := bookingsInForce(bookings, weekday, on)
	endClock, endKnown := wallClock(end)
	usesPickupRule := endKnown && everyBookingHasPickup(inForce, weekday)
	var latest calendar.Date
	for _, booking := range inForce {
		if usesPickupRule {
			pickup, _ := wallClock(booking.PickupTimes[weekday])
			if !kind.admits(booking) || pickup.Before(endClock) {
				continue
			}
		}
		for _, window := range booking.Windows {
			if slices.Contains(window.Weekdays, weekday) && window.ValidFrom != nil && !window.ValidFrom.After(on) &&
				(window.ValidUntil == nil || window.ValidUntil.After(on)) {
				latest = max(latest, *window.ValidFrom)
			}
		}
	}
	return latest
}

// endedBookingDays lists, ascending, the days on which a change or a
// withdrawal dropped an offering from the weekday: a link of it ended and no
// link of the same offering plans the weekday from that day on. A rebooking
// of the same offering drops nothing, as in the save response.
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
	slices.Sort(result)
	return slices.Compact(result)
}

// firstPlannedDay is the first occurrence of the weekday on or after start
// that the row fills, by materialization's rule for roster rows (appliesOn:
// validity, period and weekday scope), and that occurrence's end.
// Materialization's projection already left out holidays, closing days,
// exceptions, A/B weeks and the series' end.
func firstPlannedDay(template BookingLeftoverTemplate, entry TemplateRosterEntry, weekday int, start calendar.Date) (calendar.Date, string, bool) {
	for _, occurrence := range template.Occurrences {
		if occurrence.Date.Before(start) || calendar.ISOWeekday(occurrence.Date) != weekday ||
			!entry.appliesOn(occurrence.Date, occurrence.PeriodID) {
			continue
		}
		return occurrence.Date, occurrence.End, true
	}
	return "", "", false
}

// bookingsCarry applies ADR 0128's carry rule to the bookings in force on
// the planned day: the times decide when the Regeltermin has an end and
// every such booking has a pickup time that weekday; a fitting booking
// picked up no earlier than the end carries. Otherwise the day counts as
// lost only when it has fewer offerings on the weekday than the day before
// the booking ended. A day without any booking is lost.
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
	return len(bookingsInForce(bookings, weekday, endedOn.AddDays(-1))) <= len(inForce)
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

// groupLeftoverDays merges the weekdays of one Regeltermin and child whose
// gaps start and end on the same days and share the decision. A removal of
// the group starts on its earliest planned day; a weekday planned later has
// no day before its own, so the removal takes nothing a booking still
// carries.
func groupLeftoverDays(templates []BookingLeftoverTemplate, days map[leftoverKey][]leftoverDay) []BookingLeftover {
	names := make(map[int64]string, len(templates))
	for _, template := range templates {
		names[template.ID] = template.Name
	}
	groups := make(map[leftoverGroupKey]*BookingLeftover)
	for key, entries := range days {
		for _, day := range entries {
			addGroupedLeftover(groups, names, key, day)
		}
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
		if c := a.From.Compare(b.From); c != 0 {
			return c
		}
		return untilKey(a.Until).Compare(untilKey(b.Until))
	})
	return result
}

func addGroupedLeftover(groups map[leftoverGroupKey]*BookingLeftover, names map[int64]string, key leftoverKey, day leftoverDay) {
	group := leftoverGroupKey{templateID: key.templateID, studentID: key.studentID, start: day.start, until: untilKey(day.until), kept: day.kept}
	leftover := groups[group]
	if leftover == nil {
		leftover = &BookingLeftover{TemplateID: key.templateID, TemplateName: names[key.templateID],
			StudentID: key.studentID, From: day.from, Until: day.until, Kept: day.kept}
		groups[group] = leftover
	}
	leftover.Weekdays = append(leftover.Weekdays, key.weekday)
	leftover.BookingEndedOn = max(leftover.BookingEndedOn, day.endedOn)
	leftover.From = min(leftover.From, day.from)
}

// untilKey orders and groups gap ends; an open end sorts last.
func untilKey(until *calendar.Date) calendar.Date {
	if until == nil {
		return "9999-12-31"
	}
	return *until
}
