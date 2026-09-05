package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	berlinTZID = "Europe/Berlin"
	prodID     = "-//moto//Kalender//DE"
)

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

type CalendarRecurrence struct {
	Frequency string
	Interval  int
	Weekdays  []string
	MonthDays []int
	Until     string
	Count     *int
}

type CalendarEvent struct {
	UID          string
	Summary      string
	Description  string
	Location     string
	StartDate    string
	EndDate      string
	StartClock   time.Time
	EndClock     time.Time
	AllDay       bool
	Cancelled    bool
	Sequence     int
	Stamp        time.Time
	LastModified time.Time
	Recurrence   *CalendarRecurrence
	ExDates      []string
}

var weekdayToICS = map[string]string{
	"monday": "MO", "tuesday": "TU", "wednesday": "WE", "thursday": "TH",
	"friday": "FR", "saturday": "SA", "sunday": "SU",
}

func RenderCalendar(name string, events []CalendarEvent) string {
	return renderCalendar(name, events, true)
}

// RenderCalendarObject renders one RFC 5545 calendar object for CalDAV. A
// calendar object contains exactly one VEVENT and deliberately omits METHOD,
// which describes message transport rather than stored calendar data.
func RenderCalendarObject(event CalendarEvent) string {
	return renderCalendar("", []CalendarEvent{event}, false)
}

func renderCalendar(name string, events []CalendarEvent, publish bool) string {
	var b strings.Builder
	writeLine(&b, "BEGIN:VCALENDAR")
	writeLine(&b, "VERSION:2.0")
	writeLine(&b, "PRODID:"+prodID)
	writeLine(&b, "CALSCALE:GREGORIAN")
	if publish {
		writeLine(&b, "METHOD:PUBLISH")
	}
	if name != "" {
		writeLine(&b, "X-WR-CALNAME:"+escapeText(name))
		writeLine(&b, "X-WR-TIMEZONE:"+berlinTZID)
	}
	if len(events) == 0 || hasTimedEvent(events) {
		b.WriteString(berlinVTimezone)
	}
	for _, event := range events {
		writeEvent(&b, event)
	}
	writeLine(&b, "END:VCALENDAR")
	return b.String()
}

func hasTimedEvent(events []CalendarEvent) bool {
	for _, event := range events {
		if !event.AllDay {
			return true
		}
	}
	return false
}

func writeEvent(b *strings.Builder, event CalendarEvent) {
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
		writeLine(b, "DTEND;VALUE=DATE:"+formatDate(addDays(event.EndDate, 1)))
	} else {
		writeLine(b, "DTSTART;TZID="+berlinTZID+":"+formatLocal(event.StartDate, event.StartClock))
		writeLine(b, "DTEND;TZID="+berlinTZID+":"+formatLocal(event.EndDate, event.EndClock))
	}
	if rrule := formatRRULE(event.Recurrence, event.AllDay); rrule != "" {
		writeLine(b, "RRULE:"+rrule)
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

func formatRRULE(rule *CalendarRecurrence, allDay bool) string {
	if rule == nil || rule.Frequency == "" {
		return ""
	}
	freq := strings.ToUpper(rule.Frequency)
	parts := []string{"FREQ=" + freq}
	if rule.Interval > 1 {
		parts = append(parts, fmt.Sprintf("INTERVAL=%d", rule.Interval))
	}
	if freq == "WEEKLY" && len(rule.Weekdays) > 0 {
		days := make([]string, 0, len(rule.Weekdays))
		for _, weekday := range rule.Weekdays {
			if value, ok := weekdayToICS[strings.ToLower(strings.TrimSpace(weekday))]; ok {
				days = append(days, value)
			}
		}
		if len(days) > 0 {
			parts = append(parts, "BYDAY="+strings.Join(days, ","))
		}
	}
	if freq == "MONTHLY" && len(rule.MonthDays) > 0 {
		days := make([]string, 0, len(rule.MonthDays))
		for _, day := range rule.MonthDays {
			days = append(days, fmt.Sprintf("%d", day))
		}
		parts = append(parts, "BYMONTHDAY="+strings.Join(days, ","))
	}
	if rule.Until != "" {
		if allDay {
			parts = append(parts, "UNTIL="+formatDate(rule.Until))
		} else {
			until, _ := time.Parse("2006-01-02", rule.Until)
			parts = append(parts, "UNTIL="+formatUTC(time.Date(until.Year(), until.Month(), until.Day(), 23, 59, 59, 0, berlinLocation()).UTC()))
		}
	} else if rule.Count != nil && *rule.Count > 0 {
		parts = append(parts, fmt.Sprintf("COUNT=%d", *rule.Count))
	}
	return strings.Join(parts, ";")
}

func addDays(date string, days int) string {
	parsed, _ := time.Parse("2006-01-02", date)
	return parsed.AddDate(0, 0, days).Format("2006-01-02")
}

func formatDate(date string) string { return strings.ReplaceAll(date, "-", "") }

func formatLocal(date string, clock time.Time) string {
	return fmt.Sprintf("%sT%02d%02d%02d", formatDate(date), clock.Hour(), clock.Minute(), clock.Second())
}

func formatUTC(value time.Time) string { return value.UTC().Format("20060102T150405Z") }

func berlinLocation() *time.Location {
	location, err := time.LoadLocation(berlinTZID)
	if err != nil {
		panic("load Europe/Berlin timezone: " + err.Error())
	}
	return location
}

func escapeText(value string) string {
	return strings.NewReplacer("\\", "\\\\", ";", "\\;", ",", "\\,", "\n", "\\n", "\r", "").Replace(value)
}

func writeLine(b *strings.Builder, line string) {
	const limit = 75
	cut := runeSafeCut(line, limit)
	b.WriteString(line[:cut])
	b.WriteString("\r\n")
	line = line[cut:]
	for len(line) > 0 {
		cut = runeSafeCut(line, limit-1)
		b.WriteString(" ")
		b.WriteString(line[:cut])
		b.WriteString("\r\n")
		line = line[cut:]
	}
}

func runeSafeCut(value string, max int) int {
	if len(value) <= max {
		return len(value)
	}
	cut := max
	for cut > 0 && value[cut]&0xC0 == 0x80 {
		cut--
	}
	if cut == 0 {
		return max
	}
	return cut
}
