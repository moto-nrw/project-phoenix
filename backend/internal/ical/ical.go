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

// berlinVTimezone is the RFC 5545 VTIMEZONE definition for Europe/Berlin
// (CET/CEST with the EU last-Sunday DST rules). Any TZID a timed event
// references MUST be defined by a matching VTIMEZONE, or strict clients reject
// the calendar or resolve DST incorrectly. The RRULEs make it valid for every
// year, so a single static block is emitted once per document.
const berlinVTimezone = "BEGIN:VTIMEZONE\r\n" +
	"TZID:Europe/Berlin\r\n" +
	"BEGIN:DAYLIGHT\r\n" +
	"TZOFFSETFROM:+0100\r\n" +
	"TZOFFSETTO:+0200\r\n" +
	"TZNAME:CEST\r\n" +
	"DTSTART:19700329T020000\r\n" +
	"RRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU\r\n" +
	"END:DAYLIGHT\r\n" +
	"BEGIN:STANDARD\r\n" +
	"TZOFFSETFROM:+0200\r\n" +
	"TZOFFSETTO:+0100\r\n" +
	"TZNAME:CET\r\n" +
	"DTSTART:19701025T030000\r\n" +
	"RRULE:FREQ=YEARLY;BYMONTH=10;BYDAY=-1SU\r\n" +
	"END:STANDARD\r\n" +
	"END:VTIMEZONE\r\n"

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
	// LastModified drives LAST-MODIFIED; together with Sequence it tells clients
	// this VEVENT is a newer revision so edits/cancellations are not ignored.
	LastModified time.Time
	Recurrence   *Recurrence
	// ExDates are recurrence exceptions (single occurrences cancelled via an
	// occurrence override). Emitted as EXDATE lines so subscribed calendars drop
	// those dates from the RRULE expansion. Only meaningful when Recurrence is set.
	ExDates []timezone.Date
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
	// Timed events reference TZID=Europe/Berlin. An empty calendar also needs a
	// component because RFC 5545 requires at least one inside VCALENDAR; using
	// VTIMEZONE keeps the feed valid without inventing a visible appointment.
	if len(events) == 0 || hasTimedEvent(events) {
		b.WriteString(berlinVTimezone)
	}
	for _, event := range events {
		writeEvent(&b, event)
	}
	writeLine(&b, "END:VCALENDAR")
	return b.String()
}

// hasTimedEvent reports whether any event carries a wall-clock time (and thus
// references the Berlin TZID).
func hasTimedEvent(events []Event) bool {
	for _, event := range events {
		if !event.AllDay {
			return true
		}
	}
	return false
}

func writeEvent(b *strings.Builder, event Event) {
	writeLine(b, "BEGIN:VEVENT")
	writeLine(b, "UID:"+event.UID)
	writeLine(b, "DTSTAMP:"+formatUTC(event.Stamp))
	if event.Sequence > 0 {
		writeLine(b, fmt.Sprintf("SEQUENCE:%d", event.Sequence))
	}
	if !event.LastModified.IsZero() {
		writeLine(b, "LAST-MODIFIED:"+formatUTC(event.LastModified))
	}

	if event.AllDay {
		writeLine(b, "DTSTART;VALUE=DATE:"+formatDate(event.StartDate))
		// DTEND is non-inclusive: the day after the last day.
		writeLine(b, "DTEND;VALUE=DATE:"+formatDate(event.EndDate.AddDays(1)))
	} else {
		writeLine(b, "DTSTART;TZID="+berlinTZID+":"+formatLocal(event.StartDate, event.StartClock))
		writeLine(b, "DTEND;TZID="+berlinTZID+":"+formatLocal(event.EndDate, event.EndClock))
	}

	if rrule := formatRRULE(event.Recurrence, event.AllDay); rrule != "" {
		writeLine(b, "RRULE:"+rrule)
		// EXDATEs only make sense alongside an RRULE; emit one line per excluded
		// occurrence so clients that don't merge comma-lists still drop each date.
		for _, ex := range event.ExDates {
			if event.AllDay {
				writeLine(b, "EXDATE;VALUE=DATE:"+formatDate(ex))
			} else {
				writeLine(b, "EXDATE;TZID="+berlinTZID+":"+formatLocal(ex, event.StartClock))
			}
		}
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

func formatRRULE(r *Recurrence, allDay bool) string {
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
	// Only emit the BY* filters the application actually evaluates for this
	// frequency: weekly reads BYDAY, monthly reads BYMONTHDAY, daily/yearly read
	// neither. Emitting BYDAY on a DAILY rule (which the app ignores) would make
	// the exported series diverge from the in-app expansion.
	if freq == "WEEKLY" && len(r.Weekdays) > 0 {
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
	if freq == "MONTHLY" && len(r.MonthDays) > 0 {
		days := make([]string, 0, len(r.MonthDays))
		for _, d := range r.MonthDays {
			days = append(days, fmt.Sprintf("%d", d))
		}
		parts = append(parts, "BYMONTHDAY="+strings.Join(days, ","))
	}
	if r.Until != nil {
		// UNTIL's value type must match DTSTART: a date for all-day events, a UTC
		// date-time otherwise. It is inclusive either way.
		if allDay {
			parts = append(parts, "UNTIL="+formatDate(*r.Until))
		} else {
			parts = append(parts, "UNTIL="+formatUTC(r.Until.EndOfDay().UTC()))
		}
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
// 5545. The single space that begins each continuation line counts toward the
// 75-octet limit, so continuation lines carry at most 74 octets of content.
func writeLine(b *strings.Builder, line string) {
	const limit = 75
	// First physical line: up to 75 octets, no continuation prefix.
	cut := runeSafeCut(line, limit)
	b.WriteString(line[:cut])
	b.WriteString("\r\n")
	line = line[cut:]
	// Continuation lines: the leading space consumes one of the 75 octets.
	for len(line) > 0 {
		cut = runeSafeCut(line, limit-1)
		b.WriteString(" ")
		b.WriteString(line[:cut])
		b.WriteString("\r\n")
		line = line[cut:]
	}
}

// runeSafeCut returns the largest length <= max that does not split a multi-byte
// UTF-8 rune (falling back to max when no earlier boundary exists).
func runeSafeCut(s string, max int) int {
	if len(s) <= max {
		return len(s)
	}
	cut := max
	for cut > 0 && !utf8RuneStart(s[cut]) {
		cut--
	}
	if cut == 0 {
		cut = max
	}
	return cut
}

func utf8RuneStart(b byte) bool {
	return b&0xC0 != 0x80
}
