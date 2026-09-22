// Package planning is the Dienstplan application layer of Workforce (#3418):
// the staff-shift, shift-type, assignment and schedule-overview services plus
// the mappers that serve the public modules/workforce planning contracts. It
// reaches persistence only through the ports declared in ports.go, which
// modules/workforce/compose binds to the Workforce capability. No HTTP path,
// status code, error string, authorization check, tenant scoping or
// coverage-conflict semantics changed when the services moved here.
package planning

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
)

// The retained timetable vocabulary these services still speak. The coverage
// interval math and the calendar-week helpers live in the retained timetable
// package (modules/timetable/legacy/timetableplanning, #3218) because the
// timetable-side coverage probe and the staff pool share them; every entry
// goes with the retained service that uses it.

type staffDateKey = timetableplanning.StaffDateKey

var (
	sortOverviewStaff       = timetableplanning.SortOverviewStaff
	indexCalendarWeeks      = timetableplanning.IndexCalendarWeeks
	indexShifts             = timetableplanning.IndexShiftsByStaffDate
	uncoveredShiftIntervals = timetableplanning.UncoveredShiftIntervals
	containingCalendarWeek  = timetableplanning.ContainingCalendarWeek
)
