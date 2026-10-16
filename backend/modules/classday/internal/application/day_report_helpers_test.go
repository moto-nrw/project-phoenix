package application

import (
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// classDayWeekdayKey maps a calendar date onto the report day keys
// ("mon".."fri") without a weekend plan. Weekend dates return "".
func classDayWeekdayKey(date calendar.Date) string {
	return classDayKeyOf(calendar.ISOWeekday(date))
}

// buildClassDayReport builds the report for the date's own weekday, as a
// school without a weekend plan sees it.
func buildClassDayReport(schoolClass string, date calendar.Date, phaseName string, rosterRows []DayRosterRow, facts classDayFacts) *classday.DayReport {
	return buildClassDayReportFor(schoolClass, date, classDayWeekdayKey(date), phaseName, rosterRows, facts)
}
