// Package shiftplanning holds the retained staff-shift, shift-type,
// assignment, Dienstplan overview, staff-notice, shift-plan sync and
// substitution services that services/schedule used to hold (#3219). They
// moved file for file with their tests; the root composition still adapts
// them behind the public modules/workforce contracts, and no HTTP path,
// status code, error string, authorization check, tenant scoping,
// coverage-conflict or substitution semantics changed. The package goes when
// the services dissolve into the Workforce application and domain layers.
package shiftplanning

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
