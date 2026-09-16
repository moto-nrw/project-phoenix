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
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

// The retained timetable vocabulary these services still speak. The coverage
// interval math and the calendar-week helpers stay in services/schedule
// because the timetable-side coverage probe and the staff pool share them;
// every entry goes with the retained service that uses it.

type staffDateKey = schedule.StaffDateKey

var (
	sortOverviewStaff       = schedule.SortOverviewStaff
	indexCalendarWeeks      = schedule.IndexCalendarWeeks
	indexShifts             = schedule.IndexShiftsByStaffDate
	uncoveredShiftIntervals = schedule.UncoveredShiftIntervals
	containingCalendarWeek  = schedule.ContainingCalendarWeek
)
