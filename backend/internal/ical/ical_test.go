package ical

import (
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func stamp() time.Time {
	return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
}

func TestRenderTimedEvent(t *testing.T) {
	out := Render("Familienkalender", []Event{{
		UID:        "appt-1@moto",
		Summary:    "Elternabend",
		Location:   "Aula",
		StartDate:  timezone.NewDate(2026, 4, 2),
		EndDate:    timezone.NewDate(2026, 4, 2),
		StartClock: timezone.WallClock(time.Date(1, 1, 1, 18, 0, 0, 0, time.UTC)),
		EndClock:   timezone.WallClock(time.Date(1, 1, 1, 19, 30, 0, 0, time.UTC)),
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
	out := Render("", []Event{{
		UID:       "appt-2@moto",
		Summary:   "Wandertag",
		StartDate: timezone.NewDate(2026, 5, 4),
		EndDate:   timezone.NewDate(2026, 5, 4),
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
	until := timezone.NewDate(2026, 6, 30)
	out := Render("", []Event{{
		UID:        "appt-3@moto",
		Summary:    "Wöchentliche AG",
		StartDate:  timezone.NewDate(2026, 4, 6),
		EndDate:    timezone.NewDate(2026, 4, 6),
		StartClock: timezone.WallClock(time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)),
		EndClock:   timezone.WallClock(time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)),
		Stamp:      stamp(),
		Recurrence: &Recurrence{
			Freq:     "weekly",
			Interval: 2,
			Weekdays: []string{"monday", "wednesday"},
			Until:    &until,
		},
	}})
	if !strings.Contains(out, "RRULE:FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,WE;UNTIL=20260630T") {
		t.Errorf("missing/incorrect RRULE\n%s", out)
	}
}

func TestRenderFoldsLongLines(t *testing.T) {
	// A long, multibyte summary forces RFC 5545 line folding (continuation
	// lines start with a space) without splitting a UTF-8 rune.
	long := "Sehr langer Terminname mit Umlauten äöü und vielen Wörtern zur Prüfung der Zeilenfaltung über fünfundsiebzig Oktette hinaus"
	out := Render("", []Event{{
		UID:       "appt-fold@moto",
		Summary:   long,
		StartDate: timezone.NewDate(2026, 5, 4),
		EndDate:   timezone.NewDate(2026, 5, 4),
		AllDay:    true,
		Stamp:     stamp(),
	}})

	// Folded continuation lines exist, and no output line exceeds ~75 octets.
	if !strings.Contains(out, "\r\n ") {
		t.Errorf("expected folded continuation line\n%s", out)
	}
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 76 {
			t.Errorf("line exceeds fold limit (%d): %q", len(line), line)
		}
	}
}

func TestRenderRecurrenceWithExDates(t *testing.T) {
	out := Render("", []Event{{
		UID:        "appt-ex@moto",
		Summary:    "Wöchentliche AG",
		StartDate:  timezone.NewDate(2026, 4, 6),
		EndDate:    timezone.NewDate(2026, 4, 6),
		StartClock: timezone.WallClock(time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)),
		EndClock:   timezone.WallClock(time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)),
		Stamp:      stamp(),
		Recurrence: &Recurrence{Freq: "weekly", Interval: 1, Weekdays: []string{"monday"}},
		ExDates:    []timezone.Date{timezone.NewDate(2026, 4, 20)},
	}})
	// The excluded occurrence matches the DTSTART time in Berlin.
	if !strings.Contains(out, "EXDATE;TZID=Europe/Berlin:20260420T140000") {
		t.Errorf("missing timed EXDATE\n%s", out)
	}

	allDay := Render("", []Event{{
		UID:        "appt-ex-allday@moto",
		Summary:    "Ganztägige Reihe",
		StartDate:  timezone.NewDate(2026, 4, 6),
		EndDate:    timezone.NewDate(2026, 4, 6),
		AllDay:     true,
		Stamp:      stamp(),
		Recurrence: &Recurrence{Freq: "weekly", Interval: 1, Weekdays: []string{"monday"}},
		ExDates:    []timezone.Date{timezone.NewDate(2026, 4, 20)},
	}})
	if !strings.Contains(allDay, "EXDATE;VALUE=DATE:20260420") {
		t.Errorf("missing all-day EXDATE\n%s", allDay)
	}
}

func TestRenderCancelledAndEscaping(t *testing.T) {
	out := Render("", []Event{{
		UID:         "appt-4@moto",
		Summary:     "Abgesagt; wichtig, mit Komma",
		Description: "Zeile1\nZeile2",
		StartDate:   timezone.NewDate(2026, 4, 2),
		EndDate:     timezone.NewDate(2026, 4, 2),
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
