package schedule

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// The shift-coverage interval vocabulary shared by the timetable-side
// coverage probe (DetectShiftCoverage, the staff pool) and the retained
// Dienstplan overview in modules/workforce/legacy/shiftplanning (#3219). The
// overview moved with the staff-shift services; the interval math stays here
// because the coverage probe is served through TimetableDataService.

// ShiftCoverageInterval is one uncovered part of an assignment's wall-clock
// window. StartTime and EndTime are normalized through timezone.NormalizeWallClock.
type ShiftCoverageInterval struct {
	StartTime time.Time
	EndTime   time.Time
}

type InstanceStaffBatchReader interface {
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModel.InstanceStaff, error)
}

// StaffDateKey addresses one staff member's shifts or minutes on one
// calendar day (or calendar-week start).
type StaffDateKey struct {
	StaffID int64
	Date    timezone.Date
}

// SortOverviewStaff orders staff by last name, first name and id, the order
// the Dienstplan overview and the staff pool present them in.
func SortOverviewStaff(staff []*usersModel.Staff) {
	sort.Slice(staff, func(i, j int) bool {
		left := staffSortName(staff[i])
		right := staffSortName(staff[j])
		if left == right {
			return staffID(staff[i]) < staffID(staff[j])
		}
		return left < right
	})
}

func staffID(staff *usersModel.Staff) int64 {
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

// IndexCalendarWeeks marks every non-zero week start as a used Dienstplan
// week.
func IndexCalendarWeeks(weeks []timezone.Date) map[timezone.Date]bool {
	used := make(map[timezone.Date]bool, len(weeks))
	for _, week := range weeks {
		if !week.IsZero() {
			used[week] = true
		}
	}
	return used
}

// IndexShiftsByStaffDate groups shifts by staff member and calendar day.
func IndexShiftsByStaffDate(shifts []*scheduleModel.StaffShift) map[StaffDateKey][]*scheduleModel.StaffShift {
	index := make(map[StaffDateKey][]*scheduleModel.StaffShift)
	for _, shift := range shifts {
		if shift != nil {
			key := StaffDateKey{shift.StaffID, timezone.Date(shift.Date)}
			index[key] = append(index[key], shift)
		}
	}
	return index
}

type shiftCoverageWindow struct {
	start time.Time
	end   time.Time
}

// UncoveredShiftIntervals returns the exact gaps in [start, end) after
// clipping, sorting, and unioning all shift windows. Overlapping and directly
// touching shifts form one continuous interval. BreakMinutes intentionally do
// not create gaps because the model does not locate a break within the shift.
func UncoveredShiftIntervals(start, end time.Time, shifts []*scheduleModel.StaffShift) []ShiftCoverageInterval {
	start = timezone.NormalizeWallClock(start)
	end = timezone.NormalizeWallClock(end)
	if !end.After(start) {
		return []ShiftCoverageInterval{}
	}

	covered := coveredShiftWindows(start, end, shifts)
	return gapsBetweenShiftWindows(start, end, covered)
}

func coveredShiftWindows(start, end time.Time, shifts []*scheduleModel.StaffShift) []shiftCoverageWindow {
	covered := make([]shiftCoverageWindow, 0, len(shifts))
	for _, shift := range shifts {
		if window, ok := clipShiftCoverageWindow(start, end, shift); ok {
			covered = append(covered, window)
		}
	}
	sort.Slice(covered, func(i, j int) bool {
		if covered[i].start.Equal(covered[j].start) {
			return covered[i].end.Before(covered[j].end)
		}
		return covered[i].start.Before(covered[j].start)
	})
	return covered
}

func clipShiftCoverageWindow(start, end time.Time, shift *scheduleModel.StaffShift) (shiftCoverageWindow, bool) {
	if shift == nil || shift.Cancelled {
		// A cancelled shift does not take place (#1841), so it covers nothing;
		// the assignment reads as uncovered until a replacement is entered.
		return shiftCoverageWindow{}, false
	}
	shiftStart := timezone.NormalizeWallClock(shift.StartTime)
	shiftEnd := timezone.NormalizeWallClock(shift.EndTime)
	if !shiftEnd.After(start) || !end.After(shiftStart) {
		return shiftCoverageWindow{}, false
	}
	if shiftStart.Before(start) {
		shiftStart = start
	}
	if shiftEnd.After(end) {
		shiftEnd = end
	}
	return shiftCoverageWindow{start: shiftStart, end: shiftEnd}, true
}

func gapsBetweenShiftWindows(start, end time.Time, covered []shiftCoverageWindow) []ShiftCoverageInterval {
	gaps := make([]ShiftCoverageInterval, 0)
	cursor := start
	for _, current := range covered {
		if current.end.Before(cursor) || current.end.Equal(cursor) {
			continue
		}
		if current.start.After(cursor) {
			gaps = append(gaps, ShiftCoverageInterval{StartTime: cursor, EndTime: current.start})
		}
		if current.end.After(cursor) {
			cursor = current.end
		}
		if !cursor.Before(end) {
			return gaps
		}
	}
	if cursor.Before(end) {
		gaps = append(gaps, ShiftCoverageInterval{StartTime: cursor, EndTime: end})
	}
	return gaps
}
