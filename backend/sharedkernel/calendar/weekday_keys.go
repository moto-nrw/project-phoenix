package calendar

import "strings"

// CanonicalDayToISOWeekday translates a three-letter day key to its ISO
// weekday (Monday=1, Sunday=7). Case and surrounding whitespace are ignored.
func CanonicalDayToISOWeekday(day string) (int, bool) {
	weekday, ok := canonicalDayISOWeekday[strings.ToLower(strings.TrimSpace(day))]
	return weekday, ok
}

var canonicalDayISOWeekday = map[string]int{
	"mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6, "sun": 7,
}
