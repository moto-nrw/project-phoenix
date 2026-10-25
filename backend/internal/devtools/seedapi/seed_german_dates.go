package api

import "fmt"

var germanWeekdays = [...]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

var germanMonths = [...]string{
	"Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember",
}

// germanDate names a day the way the letters and polls write it:
// "Dienstag, 27. Oktober".
func germanDate(d seedDate) string {
	return fmt.Sprintf("%s, %d. %s", germanWeekdays[d.Weekday()], d.Day(), germanMonths[d.Month()-1])
}

// seedWeekMonday is the Monday of the week the day belongs to.
func seedWeekMonday(d seedDate) seedDate {
	return d.AddDays(-((int(d.Weekday()) + 6) % 7))
}
