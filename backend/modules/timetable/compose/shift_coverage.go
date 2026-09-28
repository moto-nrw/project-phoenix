package compose

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// Shift-coverage probe (#1873): does the Dienstplan cover every probed staff
// member for every effective occurrence of a single block or a recurring
// series? Advisory only — the subsequent write is never blocked.

const maxReturnedShiftCoverageWarnings = 100

// DetectShiftCoverage checks every effective occurrence with a fixed number
// of batched reads. A ReplanWeek projection uses at most eight reads: period,
// the group's recurrence envelope, group-scoped instances, group-scoped
// exceptions, all concrete staff rows, all shifts, all tenant week usage, and
// the warning names.
func (d *conflictDetection) DetectShiftCoverage(ctx context.Context, probe timetable.ShiftCoverageProbe) (timetable.ShiftCoverageResult, error) {
	dates, staffIDs, err := normalizeShiftCoverageProbe(probe)
	if err != nil {
		return timetable.ShiftCoverageResult{}, err
	}
	dates, err = d.resolveCoverageDates(ctx, probe, dates)
	if err != nil {
		return timetable.ShiftCoverageResult{}, err
	}
	if len(dates) == 0 {
		return emptyShiftCoverageResult(), nil
	}
	coverageDays, effectiveStaffIDs, err := d.resolveEffectiveCoverageDays(ctx, probe, dates, staffIDs)
	if err != nil {
		return timetable.ShiftCoverageResult{}, err
	}
	if len(effectiveStaffIDs) == 0 {
		return emptyShiftCoverageResult(), nil
	}
	shifts, usedWeeks, err := d.loadShiftCoverageData(ctx, dates, effectiveStaffIDs)
	if err != nil {
		return timetable.ShiftCoverageResult{}, err
	}
	if len(usedWeeks) == 0 {
		return emptyShiftCoverageResult(), nil
	}
	pending, total := collectPendingCoverageWarnings(dates, coverageDays, shifts, usedWeeks)
	if len(pending) == 0 {
		return emptyShiftCoverageResult(), nil
	}
	staff, err := d.deps.Staff.FindWithPersonByIDs(ctx, effectiveStaffIDs)
	if err != nil {
		return timetable.ShiftCoverageResult{}, fmt.Errorf("load staff names for coverage warnings: %w", err)
	}
	return timetable.ShiftCoverageResult{Warnings: buildShiftCoverageWarnings(pending, staff), TotalWarningCount: total}, nil
}

func emptyShiftCoverageResult() timetable.ShiftCoverageResult {
	return timetable.ShiftCoverageResult{Warnings: make([]timetable.ShiftCoverageWarning, 0)}
}

// resolveCoverageDates narrows the candidate dates to the probed period, A/B
// week and series envelope. An empty result without an error means there is
// nothing to check.
func (d *conflictDetection) resolveCoverageDates(ctx context.Context, probe timetable.ShiftCoverageProbe, dates []timezone.Date) ([]timezone.Date, error) {
	dates, err := d.resolvePeriodCoverageDates(ctx, probe, dates)
	if err != nil || len(dates) == 0 || probe.ReplanActivityGroupID == nil {
		return dates, err
	}
	return d.resolveSeriesCoverageDates(ctx, *probe.ReplanActivityGroupID, dates)
}

func (d *conflictDetection) resolvePeriodCoverageDates(ctx context.Context, probe timetable.ShiftCoverageProbe, dates []timezone.Date) ([]timezone.Date, error) {
	if probe.CalendarPeriodID == nil {
		return dates, nil
	}
	period, err := d.deps.CalendarPeriods.FindByID(ctx, *probe.CalendarPeriodID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, invalidShiftCoverageProbe("calendar period does not exist for this tenant")
		}
		return nil, fmt.Errorf("load calendar period for shift coverage: %w", err)
	}
	if period == nil {
		return nil, invalidShiftCoverageProbe("calendar period does not exist for this tenant")
	}
	if !period.IsActive {
		return []timezone.Date{}, nil
	}
	filtered := filterShiftCoverageDates(dates, period, *probe.WeekPattern)
	if probe.ConcreteInstanceDate != nil && containsCoverageDate(dates, *probe.ConcreteInstanceDate) &&
		!containsCoverageDate(filtered, *probe.ConcreteInstanceDate) {
		filtered = append(filtered, *probe.ConcreteInstanceDate)
		sort.Slice(filtered, func(i, j int) bool { return filtered[i].Before(filtered[j]) })
	}
	return filtered, nil
}

// filterShiftCoverageDates keeps the dates inside the period on which the
// week pattern applies. The A/B decision is the School Calendar's single
// week-cycle engine (ADR 0038).
func filterShiftCoverageDates(dates []timezone.Date, period *scheduleModels.CalendarPeriod, weekPattern int) []timezone.Date {
	cycle := schoolcalendar.WeekCycleOf(period.WeekCycleLength, period.WeekCycleAnchor)
	filtered := make([]timezone.Date, 0, len(dates))
	for _, date := range dates {
		if period.ContainsDay(scheduleModels.Date(date)) && schoolcalendar.WeekPatternApplies(weekPattern, date.String(), cycle) {
			filtered = append(filtered, date)
		}
	}
	return filtered
}

func containsCoverageDate(dates []timezone.Date, target timezone.Date) bool {
	return slices.Contains(dates, target)
}

func (d *conflictDetection) loadShiftCoverageData(ctx context.Context, dates []timezone.Date, staffIDs []int64) ([]*scheduleModels.StaffShift, map[timezone.Date]bool, error) {
	firstWeekFrom, _ := timetable.ContainingCalendarWeek(dates[0])
	_, lastWeekTo := timetable.ContainingCalendarWeek(dates[len(dates)-1])
	scheduleDates := make([]scheduleModels.Date, len(dates))
	for index, date := range dates {
		scheduleDates[index] = scheduleModels.Date(date)
	}
	rows, err := d.deps.Shifts.FindByStaffIDsAndDates(ctx, staffIDs, scheduleDates)
	if err != nil {
		return nil, nil, fmt.Errorf("load staff shifts for coverage: %w", err)
	}
	weeks, err := d.deps.Shifts.FindUsedCalendarWeeks(ctx, scheduleModels.Date(firstWeekFrom), scheduleModels.Date(lastWeekTo))
	if err != nil {
		return nil, nil, fmt.Errorf("load used staff-shift weeks for coverage: %w", err)
	}
	weekDates := make([]timezone.Date, len(weeks))
	for index, week := range weeks {
		weekDates[index] = timezone.Date(week)
	}
	return rows, timetable.IndexCalendarWeeks(weekDates), nil
}

type staffDateKey struct {
	staffID int64
	date    timezone.Date
}

type pendingShiftCoverageWarning struct {
	staffID int64
	date    timezone.Date
	start   time.Time
	end     time.Time
	gap     timetable.ShiftCoverageInterval
}

func collectPendingCoverageWarnings(
	dates []timezone.Date,
	coverageDays map[timezone.Date]effectiveShiftCoverageDay,
	shifts []*scheduleModels.StaffShift,
	usedWeeks map[timezone.Date]bool,
) ([]pendingShiftCoverageWarning, int) {
	shiftIndex := indexShiftsByStaffDate(shifts)
	pending := make([]pendingShiftCoverageWarning, 0, maxReturnedShiftCoverageWarnings)
	total := 0
	for _, date := range dates {
		day := coverageDays[date]
		weekFrom, _ := timetable.ContainingCalendarWeek(date)
		if !day.active || !usedWeeks[weekFrom] {
			continue
		}
		for _, staffID := range day.staffIDs {
			gaps := timetable.UncoveredShiftIntervals(day.start, day.end, shiftWindowsOf(shiftIndex[staffDateKey{staffID: staffID, date: date}]))
			for _, gap := range gaps {
				total++
				if len(pending) < maxReturnedShiftCoverageWarnings {
					pending = append(pending, pendingShiftCoverageWarning{staffID: staffID, date: date, start: day.start, end: day.end, gap: gap})
				}
			}
		}
	}
	return pending, total
}

// indexShiftsByStaffDate groups shifts by staff member and calendar day.
func indexShiftsByStaffDate(shifts []*scheduleModels.StaffShift) map[staffDateKey][]*scheduleModels.StaffShift {
	index := make(map[staffDateKey][]*scheduleModels.StaffShift)
	for _, shift := range shifts {
		if shift != nil {
			key := staffDateKey{staffID: shift.StaffID, date: timezone.Date(shift.Date)}
			index[key] = append(index[key], shift)
		}
	}
	return index
}

func buildShiftCoverageWarnings(pending []pendingShiftCoverageWarning, staff map[int64]*usersModels.Staff) []timetable.ShiftCoverageWarning {
	warnings := make([]timetable.ShiftCoverageWarning, 0, len(pending))
	for _, item := range pending {
		name := coverageStaffName(staff[item.staffID], item.staffID)
		start, end := clockLabel(item.start), clockLabel(item.end)
		gapStart, gapEnd := clockLabel(item.gap.StartTime), clockLabel(item.gap.EndTime)
		warnings = append(warnings, timetable.ShiftCoverageWarning{
			StaffID:            item.staffID,
			StaffName:          name,
			Date:               item.date.String(),
			StartTime:          start,
			EndTime:            end,
			UncoveredStartTime: gapStart,
			UncoveredEndTime:   gapEnd,
			Message: fmt.Sprintf(
				"%s ist am %s für den Termin %s–%s nicht durchgehend im Dienstplan abgedeckt (offen: %s–%s).",
				name, item.date.Format(germanDateLayout), start, end, gapStart, gapEnd,
			),
		})
	}
	return warnings
}

func coverageStaffName(staff *usersModels.Staff, id int64) string {
	if staff != nil && staff.Person != nil {
		if name := strings.TrimSpace(staff.Person.FirstName + " " + staff.Person.LastName); name != "" {
			return name
		}
	}
	return fmt.Sprintf("Mitarbeiter #%d", id)
}
