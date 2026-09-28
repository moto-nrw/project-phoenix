package students

// The ISO weekdays of the weekly plans (Monday = 1). The routes accept
// Monday to Friday; the names cover the whole week because a stored row is
// rendered as it is.
const (
	weekdayMonday    = 1
	weekdayTuesday   = 2
	weekdayWednesday = 3
	weekdayThursday  = 4
	weekdayFriday    = 5
	weekdaySaturday  = 6
	weekdaySunday    = 7
)

// weekdayNames maps a weekday number to its German name.
var weekdayNames = map[int]string{
	weekdayMonday:    "Montag",
	weekdayTuesday:   "Dienstag",
	weekdayWednesday: "Mittwoch",
	weekdayThursday:  "Donnerstag",
	weekdayFriday:    "Freitag",
	weekdaySaturday:  "Samstag",
	weekdaySunday:    "Sonntag",
}
