package domain

import (
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func stamp() time.Time {
	return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
}

func date(year int, month time.Month, day int) string {
	return testpkg.Date(year, month, day).String()
}

func TestRenderTimedEvent(t *testing.T) {
	t.Parallel()

	out := RenderCalendar("Familienkalender", []CalendarEvent{{
		UID:        "appt-1@moto",
		Summary:    "Elternabend",
		Location:   "Aula",
		StartDate:  date(2026, 4, 2),
		EndDate:    date(2026, 4, 2),
		StartClock: testpkg.WallClock(18, 0),
		EndClock:   testpkg.WallClock(19, 30),
		Stamp:      stamp(),
	}})

	for _, want := range []string{
		"BEGIN:VCALENDAR",
		"BEGIN:VEVENT",
		"UID:appt-1@moto",
		"DTSTART;TZID=Europe/Berlin:20260402T180000",
		"DTEND;TZID=Europe/Berlin:20260402T193000",
		"SUMMARY:Elternabend",
		"LOCATION:Aula",
		"STATUS:CONFIRMED",
		"END:VEVENT",
		"END:VCALENDAR",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
	if !strings.Contains(out, "\r\n") {
		t.Error("lines must be CRLF-terminated")
	}
}

func TestRenderAllDayEvent(t *testing.T) {
	t.Parallel()

	out := RenderCalendar("", []CalendarEvent{{
		UID:       "appt-2@moto",
		Summary:   "Wandertag",
		StartDate: date(2026, 5, 4),
		EndDate:   date(2026, 5, 4),
		AllDay:    true,
		Stamp:     stamp(),
	}})
	if !strings.Contains(out, "DTSTART;VALUE=DATE:20260504") {
		t.Errorf("missing all-day DTSTART\n%s", out)
	}
	// DTEND is non-inclusive: the day after.
	if !strings.Contains(out, "DTEND;VALUE=DATE:20260505") {
		t.Errorf("missing non-inclusive DTEND\n%s", out)
	}
}

func TestRenderRecurrence(t *testing.T) {
	t.Parallel()

	until := date(2026, 6, 30)
	out := RenderCalendar("", []CalendarEvent{{
		UID:        "appt-3@moto",
		Summary:    "Wöchentliche AG",
		StartDate:  date(2026, 4, 6),
		EndDate:    date(2026, 4, 6),
		StartClock: testpkg.WallClock(14, 0),
		EndClock:   testpkg.WallClock(15, 0),
		Stamp:      stamp(),
		Recurrence: &CalendarRecurrence{
			Frequency: "weekly",
			Interval:  2,
			Weekdays:  []string{"monday", "wednesday"},
			Until:     until,
		},
	}})
	if !strings.Contains(out, "RRULE:FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE;UNTIL=20260630T") {
		t.Errorf("missing/incorrect RRULE\n%s", out)
	}
}

func TestRenderFoldsLongLines(t *testing.T) {
	t.Parallel()

	// A long, multibyte summary forces RFC 5545 line folding (continuation
	// lines start with a space) without splitting a UTF-8 rune.
	long := "Sehr langer Terminname mit Umlauten äöü und vielen Wörtern zur Prüfung der Zeilenfaltung über fünfundsiebzig Oktette hinaus"
	out := RenderCalendar("", []CalendarEvent{{
		UID:       "appt-fold@moto",
		Summary:   long,
		StartDate: date(2026, 5, 4),
		EndDate:   date(2026, 5, 4),
		AllDay:    true,
		Stamp:     stamp(),
	}})

	// Folded continuation lines exist, and no content line exceeds RFC 5545's
	// 75-octet limit — the leading continuation space counts toward that limit.
	if !strings.Contains(out, "\r\n ") {
		t.Errorf("expected folded continuation line\n%s", out)
	}
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line exceeds 75-octet fold limit (%d): %q", len(line), line)
		}
	}
}

func TestRenderIncludesVTimezoneForTimedEvents(t *testing.T) {
	t.Parallel()

	timed := RenderCalendar("", []CalendarEvent{{
		UID:        "appt-tz@moto",
		Summary:    "Besprechung",
		StartDate:  date(2026, 5, 4),
		EndDate:    date(2026, 5, 4),
		StartClock: testpkg.WallClock(14, 0),
		EndClock:   testpkg.WallClock(15, 0),
		Stamp:      stamp(),
	}})
	// A timed event references TZID=Europe/Berlin, which must be backed by a
	// matching VTIMEZONE with both DST arms.
	if !strings.Contains(timed, "BEGIN:VTIMEZONE") || !strings.Contains(timed, "TZID:Europe/Berlin") {
		t.Errorf("timed export must include a VTIMEZONE\n%s", timed)
	}
	if !strings.Contains(timed, "TZNAME:CEST") || !strings.Contains(timed, "TZNAME:CET") {
		t.Errorf("VTIMEZONE must define both CET and CEST\n%s", timed)
	}
	// The VTIMEZONE must precede the VEVENT that references it.
	if strings.Index(timed, "BEGIN:VTIMEZONE") > strings.Index(timed, "BEGIN:VEVENT") {
		t.Errorf("VTIMEZONE must come before the VEVENT\n%s", timed)
	}

	// An all-day-only calendar references no TZID, so no VTIMEZONE is emitted.
	allDay := RenderCalendar("", []CalendarEvent{{
		UID:       "appt-allday-tz@moto",
		Summary:   "Wandertag",
		StartDate: date(2026, 5, 4),
		EndDate:   date(2026, 5, 4),
		AllDay:    true,
		Stamp:     stamp(),
	}})
	if strings.Contains(allDay, "VTIMEZONE") {
		t.Errorf("all-day-only export should not emit a VTIMEZONE\n%s", allDay)
	}
}

func TestRenderEmptyCalendarIncludesComponent(t *testing.T) {
	t.Parallel()

	out := RenderCalendar("Leerer Kalender", nil)

	if !strings.Contains(out, "BEGIN:VTIMEZONE") {
		t.Fatalf("RFC 5545 requires at least one calendar component\n%s", out)
	}
	if strings.Contains(out, "BEGIN:VEVENT") {
		t.Fatalf("empty calendar must not invent an event\n%s", out)
	}
}

func TestRenderRecurrenceWithExDates(t *testing.T) {
	t.Parallel()

	out := RenderCalendar("", []CalendarEvent{{
		UID:        "appt-ex@moto",
		Summary:    "Wöchentliche AG",
		StartDate:  date(2026, 4, 6),
		EndDate:    date(2026, 4, 6),
		StartClock: testpkg.WallClock(14, 0),
		EndClock:   testpkg.WallClock(15, 0),
		Stamp:      stamp(),
		Recurrence: &CalendarRecurrence{Frequency: "weekly", Interval: 1, Weekdays: []string{"monday"}},
		ExDates:    listOf(date(2026, 4, 20)),
	}})
	// The excluded occurrence matches the DTSTART time in Berlin.
	if !strings.Contains(out, "EXDATE;TZID=Europe/Berlin:20260420T140000") {
		t.Errorf("missing timed EXDATE\n%s", out)
	}

	allDay := RenderCalendar("", []CalendarEvent{{
		UID:        "appt-ex-allday@moto",
		Summary:    "Ganztägige Reihe",
		StartDate:  date(2026, 4, 6),
		EndDate:    date(2026, 4, 6),
		AllDay:     true,
		Stamp:      stamp(),
		Recurrence: &CalendarRecurrence{Frequency: "weekly", Interval: 1, Weekdays: []string{"monday"}},
		ExDates:    listOf(date(2026, 4, 20)),
	}})
	if !strings.Contains(allDay, "EXDATE;VALUE=DATE:20260420") {
		t.Errorf("missing all-day EXDATE\n%s", allDay)
	}
}

func TestRenderCancelledAndEscaping(t *testing.T) {
	t.Parallel()

	out := RenderCalendar("", []CalendarEvent{{
		UID:         "appt-4@moto",
		Summary:     "Abgesagt; wichtig, mit Komma",
		Description: "Zeile1\nZeile2",
		StartDate:   date(2026, 4, 2),
		EndDate:     date(2026, 4, 2),
		AllDay:      true,
		Cancelled:   true,
		Sequence:    2,
		Stamp:       stamp(),
	}})
	if !strings.Contains(out, "STATUS:CANCELLED") {
		t.Errorf("cancelled event must carry STATUS:CANCELLED\n%s", out)
	}
	if !strings.Contains(out, "SEQUENCE:2") {
		t.Errorf("missing SEQUENCE\n%s", out)
	}
	if !strings.Contains(out, `SUMMARY:Abgesagt\; wichtig\, mit Komma`) {
		t.Errorf("special chars not escaped\n%s", out)
	}
	if !strings.Contains(out, `DESCRIPTION:Zeile1\nZeile2`) {
		t.Errorf("newline not escaped\n%s", out)
	}
}

func TestRenderAllDayRecurrenceUntilIsDate(t *testing.T) {
	t.Parallel()

	until := date(2026, 6, 30)
	out := RenderCalendar("", []CalendarEvent{{
		UID:        "appt-allday-rrule@moto",
		Summary:    "Ganztägige Reihe",
		StartDate:  date(2026, 4, 6),
		EndDate:    date(2026, 4, 6),
		AllDay:     true,
		Stamp:      stamp(),
		Recurrence: &CalendarRecurrence{Frequency: "weekly", Interval: 1, Weekdays: []string{"monday"}, Until: until},
	}})
	// An all-day (VALUE=DATE) DTSTART requires a date-valued UNTIL, not a
	// date-time; a date-time UNTIL here is invalid iCalendar.
	if !strings.Contains(out, "UNTIL=20260630") {
		t.Errorf("all-day recurrence must include UNTIL=20260630\n%s", out)
	}
	if strings.Contains(out, "UNTIL=20260630T") {
		t.Errorf("all-day recurrence must not emit a date-time UNTIL\n%s", out)
	}
}

func TestRenderRRULEFiltersMatchFrequency(t *testing.T) {
	t.Parallel()

	// A daily rule that happens to carry weekdays must NOT export BYDAY — the app
	// ignores weekdays for daily rules, so exporting BYDAY would diverge.
	daily := RenderCalendar("", []CalendarEvent{{
		UID:        "appt-daily@moto",
		Summary:    "Täglich",
		StartDate:  date(2026, 4, 6),
		EndDate:    date(2026, 4, 6),
		StartClock: testpkg.WallClock(9, 0),
		EndClock:   testpkg.WallClock(10, 0),
		Stamp:      stamp(),
		Recurrence: &CalendarRecurrence{Frequency: "daily", Interval: 1, Weekdays: []string{"monday"}},
	}})
	// Scope the check to the event's RRULE — the VTIMEZONE legitimately carries a
	// BYDAY for its DST rule, which is unrelated.
	if strings.Contains(eventRRULE(daily), "BYDAY") {
		t.Errorf("daily rule must not export BYDAY\n%s", daily)
	}

	// A weekly rule carrying month_days must export BYDAY but NOT BYMONTHDAY.
	weekly := RenderCalendar("", []CalendarEvent{{
		UID:        "appt-weekly@moto",
		Summary:    "Wöchentlich",
		StartDate:  date(2026, 4, 6),
		EndDate:    date(2026, 4, 6),
		StartClock: testpkg.WallClock(9, 0),
		EndClock:   testpkg.WallClock(10, 0),
		Stamp:      stamp(),
		Recurrence: &CalendarRecurrence{Frequency: "weekly", Interval: 1, Weekdays: []string{"monday"}, MonthDays: []int{15}},
	}})
	weeklyRRULE := eventRRULE(weekly)
	if !strings.Contains(weeklyRRULE, "BYDAY=MO") {
		t.Errorf("weekly rule must export BYDAY\n%s", weekly)
	}
	if strings.Contains(weeklyRRULE, "BYMONTHDAY") {
		t.Errorf("weekly rule must not export BYMONTHDAY\n%s", weekly)
	}
}

// eventRRULE returns the RRULE line of the first VEVENT, skipping any VTIMEZONE
// RRULEs that precede it.
func eventRRULE(out string) string {
	_, after, ok := strings.Cut(out, "BEGIN:VEVENT")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(after, "\r\n") {
		if strings.HasPrefix(line, "RRULE:") {
			return line
		}
	}
	return ""
}

func TestRenderSequenceAndLastModified(t *testing.T) {
	t.Parallel()

	out := RenderCalendar("", []CalendarEvent{{
		UID:          "appt-rev@moto",
		Summary:      "Geändert",
		StartDate:    date(2026, 4, 2),
		EndDate:      date(2026, 4, 2),
		AllDay:       true,
		Sequence:     3,
		Stamp:        stamp(),
		LastModified: time.Date(2026, 2, 1, 8, 30, 0, 0, time.UTC),
	}})
	if !strings.Contains(out, "SEQUENCE:3") {
		t.Errorf("missing SEQUENCE\n%s", out)
	}
	if !strings.Contains(out, "LAST-MODIFIED:20260201T083000Z") {
		t.Errorf("missing LAST-MODIFIED\n%s", out)
	}
}

// listOf builds a typed slice from the helper's return type without naming it.
func listOf[T any](items ...T) []T { return items }
