// Package planning is the Dienstplan application layer of Workforce (#3418):
// the staff-shift, shift-type, assignment and schedule-overview services plus
// the mappers that serve the public modules/workforce planning contracts. It
// reaches persistence only through the ports declared in ports.go, which
// modules/workforce/compose binds to the Workforce capability. No HTTP path,
// status code, error string, authorization check, tenant scoping or
// coverage-conflict semantics changed when the services moved here.
package planning

import (
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The Dienstplan overview shares the shift-coverage interval math and the
// calendar-week helpers with the Timetable owner's coverage probe and staff
// pool; it takes them from the public Timetable contract (#3550). The
// indexing and ordering helpers below are the overview's own.

var (
	indexCalendarWeeks     = timetable.IndexCalendarWeeks
	containingCalendarWeek = timetable.ContainingCalendarWeek
)

// staffDateKey addresses one staff member's shifts or minutes on one
// calendar day (or calendar-week start).
type staffDateKey struct {
	StaffID int64
	Date    timezone.Date
}

// uncoveredShiftIntervals returns the gaps of the assignment window the
// shifts leave uncovered (timetable.UncoveredShiftIntervals).
func uncoveredShiftIntervals(start, end time.Time, shifts []*StaffShift) []timetable.ShiftCoverageInterval {
	windows := make([]timetable.ShiftWindow, 0, len(shifts))
	for _, shift := range shifts {
		if shift != nil {
			windows = append(windows, timetable.ShiftWindow{StartTime: shift.StartTime, EndTime: shift.EndTime, Cancelled: shift.Cancelled})
		}
	}
	return timetable.UncoveredShiftIntervals(start, end, windows)
}

// indexShifts groups shifts by staff member and calendar day.
func indexShifts(shifts []*StaffShift) map[staffDateKey][]*StaffShift {
	index := make(map[staffDateKey][]*StaffShift)
	for _, shift := range shifts {
		if shift != nil {
			key := staffDateKey{shift.StaffID, shift.Date}
			index[key] = append(index[key], shift)
		}
	}
	return index
}

// sortOverviewStaff orders staff by last name, first name and id, the order
// the Dienstplan overview presents them in.
func sortOverviewStaff(staff []*usersModel.Staff) {
	sort.Slice(staff, func(i, j int) bool {
		left := staffSortName(staff[i])
		right := staffSortName(staff[j])
		if left == right {
			return staffSortID(staff[i]) < staffSortID(staff[j])
		}
		return left < right
	})
}

func staffSortID(staff *usersModel.Staff) int64 {
	if staff == nil {
		return 0
	}
	return staff.ID
}

func staffSortName(staff *usersModel.Staff) string {
	if staff == nil || staff.Person == nil {
		return ""
	}
	return strings.ToLower(staff.Person.LastName + "\x00" + staff.Person.FirstName)
}
