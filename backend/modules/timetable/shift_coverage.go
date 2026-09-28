package timetable

import (
	"errors"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrInvalidShiftCoverageQuery identifies caller-controlled validation
// failures of the shift-coverage probe; the API maps every wrapped instance
// to one stable 400 response.
var ErrInvalidShiftCoverageQuery = errors.New("invalid shift coverage query")

// ShiftCoverageWarning is one advisory uncovered interval returned by the
// shift-coverage probe. It never blocks the subsequent write.
type ShiftCoverageWarning struct {
	StaffID            int64  `json:"staff_id"`
	StaffName          string `json:"staff_name"`
	Date               string `json:"date"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	UncoveredStartTime string `json:"uncovered_start_time"`
	UncoveredEndTime   string `json:"uncovered_end_time"`
	Message            string `json:"message"`
}

// ShiftCoverageResult carries at most a bounded number of warnings plus the
// total number of uncovered intervals found.
type ShiftCoverageResult struct {
	Warnings          []ShiftCoverageWarning
	TotalWarningCount int
}

// ShiftCoverageProbe describes one single-occurrence or recurring-series
// availability probe. Dates are explicit candidate weekdays supplied by the
// caller; optional period and A/B filtering selects the actual occurrences.
// StartTime and EndTime are wall-clock values with any date anchor.
type ShiftCoverageProbe struct {
	Dates                 []calendar.Date
	StartTime             time.Time
	EndTime               time.Time
	StaffIDs              []int64
	ExcludeInstanceID     *int64
	ConcreteInstanceDate  *calendar.Date
	ReplanActivityGroupID *int64
	CalendarPeriodID      *int64
	WeekPattern           *int
}

// ShiftCoverageInterval is one uncovered part of an assignment's wall-clock
// window. StartTime and EndTime are normalized through
// calendar.NormalizeWallClock.
type ShiftCoverageInterval struct {
	StartTime time.Time
	EndTime   time.Time
}

// ShiftWindow is the wall-clock window of one planned shift. A cancelled
// shift does not take place (#1841) and covers nothing.
type ShiftWindow struct {
	StartTime time.Time
	EndTime   time.Time
	Cancelled bool
}

// UncoveredShiftIntervals returns the exact gaps in [start, end) after
// clipping, sorting, and unioning all shift windows. Overlapping and directly
// touching shifts form one continuous interval. Break minutes intentionally do
// not create gaps because a shift does not locate its break.
func UncoveredShiftIntervals(start, end time.Time, shifts []ShiftWindow) []ShiftCoverageInterval {
	start = calendar.NormalizeWallClock(start)
	end = calendar.NormalizeWallClock(end)
	if !end.After(start) {
		return []ShiftCoverageInterval{}
	}
	return gapsBetweenShiftWindows(start, end, coveredShiftWindows(start, end, shifts))
}

type shiftCoverageWindow struct {
	start time.Time
	end   time.Time
}

func coveredShiftWindows(start, end time.Time, shifts []ShiftWindow) []shiftCoverageWindow {
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

func clipShiftCoverageWindow(start, end time.Time, shift ShiftWindow) (shiftCoverageWindow, bool) {
	if shift.Cancelled {
		// A cancelled shift does not take place (#1841), so it covers nothing;
		// the assignment reads as uncovered until a replacement is entered.
		return shiftCoverageWindow{}, false
	}
	shiftStart := calendar.NormalizeWallClock(shift.StartTime)
	shiftEnd := calendar.NormalizeWallClock(shift.EndTime)
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

// ContainingCalendarWeek returns the Monday and Sunday of the ISO week that
// contains date. Dienstplan comparison accepts all seven weekdays even though
// the weekly grid renders Monday-Friday.
func ContainingCalendarWeek(date calendar.Date) (calendar.Date, calendar.Date) {
	monday := date.StartOfISOWeek()
	return monday, monday.AddDays(6)
}

// IndexCalendarWeeks marks every non-zero week start as a used Dienstplan
// week.
func IndexCalendarWeeks(weeks []calendar.Date) map[calendar.Date]bool {
	used := make(map[calendar.Date]bool, len(weeks))
	for _, week := range weeks {
		if !week.IsZero() {
			used[week] = true
		}
	}
	return used
}
