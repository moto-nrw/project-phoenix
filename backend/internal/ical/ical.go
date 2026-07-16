// Package ical renders calendar appointments into RFC 5545 iCalendar
// (text/calendar) documents. It is deliberately minimal: enough to produce
// VEVENTs that Google Calendar (Android), Apple Calendar (iOS) and Outlook
// import correctly, including recurrence, all-day events and cancellations.
//
// Wall-clock times are emitted with TZID=Europe/Berlin (the app's single
// timezone); all-day events use VALUE=DATE with a DTEND on the day AFTER the
// last day, per the RFC's non-inclusive end convention.
package ical

import (
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

const (
	// berlinTZID is the single timezone the product operates in. Wall-clock
	// TIME columns are emitted against it so clients localise correctly.
	berlinTZID = "Europe/Berlin"
	prodID     = "-//moto//Kalender//DE"
)

// Recurrence mirrors the appointment recurrence rule in the terms an RRULE
// needs. Freq is DAILY/WEEKLY/MONTHLY/YEARLY.
type Recurrence struct {
	Freq      string
	Interval  int
	Weekdays  []string // lowercase go weekday names (monday…sunday)
	MonthDays []int
	Until     *timezone.Date
	Count     *int
}

// Event is a single VEVENT to render.
type Event struct {
	UID         string
	Summary     string
	Description string
	Location    string
	StartDate   timezone.Date
	EndDate     timezone.Date
	StartClock  time.Time // wall-clock; ignored when AllDay
	EndClock    time.Time // wall-clock; ignored when AllDay
	AllDay      bool
	Cancelled   bool
	Sequence    int
	Stamp       time.Time // DTSTAMP — pass a fixed value so output is stable
	Recurrence  *Recurrence
}

var weekdayToICS = map[string]string{
	"monday":    "MO",
	"tuesday":   "TU",
	"wednesday": "WE",
	"thursday":  "TH",
	"friday":    "FR",
	"saturday":  "SA",
	"sunday":    "SU",
}

// Render produces a full VCALENDAR document for the given events.
func Render(calendarName string, events []Event) string {
	var b strings.Builder
	writeLine(&b, "BEGIN:VCALENDAR")
	writeLine(&b, "VERSION:2.0")
	writeLine(&b, "PRODID:"+prodID)
	writeLine(&b, "CALSCALE:GREGORIAN")
	writeLine(&b, "METHOD:PUBLISH")
	if calendarName != "" {
		writeLine(&b, "X-WR-CALNAME:"+escapeText(calendarName))
		writeLine(&b, "X-WR-TIMEZONE:"+berlinTZID)
	}
	for _, event := range events {
		writeEvent(&b, event)
	}
	writeLine(&b, "END:VCALENDAR")
	return b.String()
}

func writeEvent(b *strings.Builder, event Event) {
	writeLine(b, "BEGIN:VEVENT")
	writeLine(b, "UID:"+event.UID)
	writeLine(b, "DTSTAMP:"+formatUTC(event.Stamp))
	if event.Sequence > 0 {
		writeLine(b, fmt.Sprintf("SEQUENCE:%d", event.Sequence))
	}

	if event.AllDay {
		writeLine(b, "DTSTART;VALUE=DATE:"+formatDate(event.StartDate))
		// DTEND is non-inclusive: the day after the last day.
		writeLine(b, "DTEND;VALUE=DATE:"+formatDate(event.EndDate.AddDays(1)))
	} else {
		writeLine(b, "DTSTART;TZID="+berlinTZID+":"+formatLocal(event.StartDate, event.StartClock))
		writeLine(b, "DTEND;TZID="+berlinTZID+":"+formatLocal(event.EndDate, event.EndClock))
	}

	if rrule := formatRRULE(event.Recurrence); rrule != "" {
		writeLine(b, "RRULE:"+rrule)
	}

	writeLine(b, "SUMMARY:"+escapeText(event.Summary))
	if event.Description != "" {
		writeLine(b, "DESCRIPTION:"+escapeText(event.Description))
	}
	if event.Location != "" {
		writeLine(b, "LOCATION:"+escapeText(event.Location))
	}
	if event.Cancelled {
		writeLine(b, "STATUS:CANCELLED")
	} else {
		writeLine(b, "STATUS:CONFIRMED")
	}
	writeLine(b, "END:VEVENT")
}

func formatRRULE(r *Recurrence) string {
	if r == nil {
		return ""
	}
	freq := strings.ToUpper(r.Freq)
	if freq == "" {
		return ""
	}
	parts := []string{"FREQ=" + freq}
	if r.Interval > 1 {
		parts = append(parts, fmt.Sprintf("INTERVAL=%d", r.Interval))
	}
	if len(r.Weekdays) > 0 {
		days := make([]string, 0, len(r.Weekdays))
		for _, wd := range r.Weekdays {
			if ics, ok := weekdayToICS[strings.ToLower(strings.TrimSpace(wd))]; ok {
				days = append(days, ics)
			}
		}
		if len(days) > 0 {
			parts = append(parts, "BYDAY="+strings.Join(days, ","))
		}
	}
	if len(r.MonthDays) > 0 {
		days := make([]string, 0, len(r.MonthDays))
		for _, d := range r.MonthDays {
			days = append(days, fmt.Sprintf("%d", d))
		}
		parts = append(parts, "BYMONTHDAY="+strings.Join(days, ","))
	}
	if r.Until != nil {
		// UNTIL is inclusive; use end-of-day in UTC so the last occurrence counts.
		parts = append(parts, "UNTIL="+formatUTC(r.Until.EndOfDay().UTC()))
	} else if r.Count != nil && *r.Count > 0 {
		parts = append(parts, fmt.Sprintf("COUNT=%d", *r.Count))
	}
	return strings.Join(parts, ";")
}

func formatDate(d timezone.Date) string {
	return d.Format("20060102")
}

func formatLocal(d timezone.Date, clock time.Time) string {
	wc := timezone.WallClock(clock)
	return fmt.Sprintf("%sT%02d%02d%02d", formatDate(d), wc.Hour(), wc.Minute(), wc.Second())
}

func formatUTC(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

// escapeText escapes the RFC 5545 special characters in text values.
func escapeText(s string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\n", "\\n",
		"\r", "",
	)
	return replacer.Replace(s)
}

// writeLine appends a content line, folding it at 75 octets with CRLF per RFC
// 5545 (continuation lines start with a single space).
func writeLine(b *strings.Builder, line string) {
	const limit = 75
	if len(line) <= limit {
		b.WriteString(line)
		b.WriteString("\r\n")
		return
	}
	// Fold on byte boundaries that don't split a UTF-8 rune.
	for len(line) > limit {
		cut := limit
		for cut > 0 && !utf8RuneStart(line[cut]) {
			cut--
		}
		if cut == 0 {
			cut = limit
		}
		b.WriteString(line[:cut])
		b.WriteString("\r\n ")
		line = line[cut:]
	}
	b.WriteString(line)
	b.WriteString("\r\n")
}

func utf8RuneStart(b byte) bool {
	return b&0xC0 != 0x80
}
