package timetracking

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// buildPrefetch loads every month-math input for the given staff members with
// one query per source. `lower` lets the caller widen the window below the
// account anchor (the ISO week can start in the previous month).
func (s *staffOverviewService) buildPrefetch(
	ctx context.Context,
	staffMembers []OverviewStaff,
	lower *calendar.Date,
	through monthKey,
) (*monthPrefetch, error) {
	memo := newMemoSettingsResolver(s.settings)
	from, err := s.prefetchStart(ctx, memo, lower, through)
	if err != nil {
		return nil, err
	}

	staffIDs, staffByID, modelIDs := indexOverviewStaff(staffMembers)
	prefetch := &monthPrefetch{
		from: from, to: through.lastDay(),
		staff:    staffByID,
		settings: memo,
	}
	if err := s.prefetchSessionRows(ctx, prefetch, staffIDs); err != nil {
		return nil, err
	}
	if err := s.prefetchCreditRows(ctx, prefetch, staffIDs); err != nil {
		return nil, err
	}
	if prefetch.schedules, prefetch.scheduleHist, err = s.prefetchSchedules(ctx, staffIDs, prefetch.from, prefetch.to); err != nil {
		return nil, err
	}
	if prefetch.workTimeModels, err = s.prefetchWorkTimeModels(ctx, modelIDs); err != nil {
		return nil, err
	}
	if err := s.prefetchDayTargets(ctx, prefetch, staffIDs); err != nil {
		return nil, err
	}
	if prefetch.snapshots, err = s.prefetchSnapshots(ctx, staffIDs); err != nil {
		return nil, err
	}
	return prefetch, nil
}

// prefetchStart is the first day of the prefetch window: the account anchor,
// widened down to lower.
func (s *staffOverviewService) prefetchStart(ctx context.Context, memo *memoSettingsResolver, lower *calendar.Date, through monthKey) (calendar.Date, error) {
	anchor, err := resolveAccountAnchor(ctx, memo, s.getLogger(), through)
	if err != nil {
		return calendar.Date(""), err
	}
	from := anchor
	// getMonthSummaryThrough summarizes a month before the configured account
	// start as a standalone month. Prefetch the same month instead of querying
	// the future anchor through an earlier month end.
	if through.before(monthOf(anchor)) {
		from = through.firstDay()
	}
	if lower != nil && lower.Before(from) {
		from = *lower
	}
	// Same bound the detail view enforces, checked BEFORE loading anything:
	// a misconfigured account start must fail like it does per staff member
	// instead of pulling decades of rows for everybody.
	if floorKey := through.addMonths(-maxCarryChainMonths); monthOf(from).before(floorKey) {
		return calendar.Date(""), fmt.Errorf("%w: %s is more than %d months before %d-%02d",
			ErrAccountStartTooOld, from.String(), maxCarryChainMonths, through.Year, through.Month)
	}
	return from, nil
}

// indexOverviewStaff returns the staff IDs, their schedule assignments and
// the distinct work-time models they are bound to.
func indexOverviewStaff(staffMembers []OverviewStaff) ([]int64, map[int64]*StaffScheduleAssignment, []int64) {
	staffIDs := make([]int64, 0, len(staffMembers))
	staffByID := make(map[int64]*StaffScheduleAssignment, len(staffMembers))
	modelIDs := make([]int64, 0, len(staffMembers))
	for _, staff := range staffMembers {
		staffIDs = append(staffIDs, staff.ID)
		staffByID[staff.ID] = &StaffScheduleAssignment{WorkTimeModelID: staff.WorkTimeModelID, RotationAnchorDate: staff.RotationAnchorDate}
		if staff.WorkTimeModelID != nil && !slices.Contains(modelIDs, *staff.WorkTimeModelID) {
			modelIDs = append(modelIDs, *staff.WorkTimeModelID)
		}
	}
	return staffIDs, staffByID, modelIDs
}

// prefetchSessionRows loads the work sessions intersecting the window and
// their breaks.
func (s *staffOverviewService) prefetchSessionRows(ctx context.Context, prefetch *monthPrefetch, staffIDs []int64) (err error) {
	end := prefetch.to.AddDays(1).BerlinMidnight()
	sessionRecords, err := s.sessionRepo.ListOverlappingWorkSessions(ctx, staffIDs, prefetch.from.BerlinMidnight(), &end)
	prefetch.sessions = workSessionsByStaff(sessionRecords)
	if err = rowReadError("list overlapping by staff IDs", err, nil); err != nil {
		return fmt.Errorf("failed to prefetch work sessions: %w", err)
	}
	// Breaks are keyed by session ID, so they can only be loaded once the
	// sessions are known. The interval math needs both completed and running
	// breaks to assign a night block's pause to the correct calendar day.
	sessionIDs := make([]int64, 0)
	for _, sessions := range prefetch.sessions {
		for _, session := range sessions {
			sessionIDs = append(sessionIDs, session.ID)
		}
	}
	var breakRecords []workforce.WorkSessionBreak
	var breakErr error
	if len(sessionIDs) > 0 {
		breakRecords, breakErr = s.breakRepo.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionIDs: sessionIDs})
	}
	err = breakErr
	prefetch.breaks = workBreaksBySession(breakRecords)
	if err = rowReadError("get breaks by session IDs", err, nil); err != nil {
		return fmt.Errorf("failed to prefetch work-session breaks: %w", err)
	}
	return nil
}

// prefetchCreditRows loads the absences, balance adjustments and shifts of
// the window.
func (s *staffOverviewService) prefetchCreditRows(ctx context.Context, prefetch *monthPrefetch, staffIDs []int64) (err error) {
	from, to := prefetch.from, prefetch.to
	absenceValues, err := s.absenceRepo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffIDs: staffIDs, OverlapFrom: from.String(), OverlapTo: to.String(), Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderStaffID}, {Field: workforce.StaffAbsenceOrderDateStart}}})
	prefetch.absences = staffAbsencesByID(absenceValues)
	err = rowReadError("get absences by staff IDs and date range", err, nil)
	if err != nil {
		return fmt.Errorf("failed to prefetch absences: %w", err)
	}
	adjustmentValues, err := s.adjustmentRepo.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{StaffIDs: staffIDs, EffectiveFrom: from.String(), EffectiveTo: to.String()})
	prefetch.adjustments = staffAdjustmentsByID(adjustmentValues)
	err = rowReadError("get balance adjustments by staff IDs+range", err, nil)
	if err != nil {
		return fmt.Errorf("failed to prefetch balance adjustments: %w", err)
	}
	if prefetch.shifts, err = s.shiftRepo.FindByStaffIDsAndDateRange(ctx, staffIDs, calendar.Date(from), calendar.Date(to)); err != nil {
		return fmt.Errorf("failed to prefetch staff shifts: %w", err)
	}
	return nil
}

// prefetchSchedules loads the schedule targets of the window and the
// schedule history.
func (s *staffOverviewService) prefetchSchedules(ctx context.Context, staffIDs []int64, from, to calendar.Date) (map[int64]WorkScheduleTargets, map[int64]bool, error) {
	scheduleTargets, err := s.scheduleRepo.TargetsByStaff(ctx, staffIDs, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prefetch work schedules: %w", err)
	}
	// Not derivable from the range-filtered schedules above: the question is
	// whether a staff member EVER had a schedule version, which is what
	// decides if the assigned work-time model may stand in.
	scheduleHist, err := s.scheduleRepo.FindStaffIDsWithScheduleHistory(ctx, staffIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prefetch schedule history: %w", err)
	}
	return scheduleTargets, scheduleHist, nil
}

// prefetchWorkTimeModels loads the assigned work-time models by ID.
func (s *staffOverviewService) prefetchWorkTimeModels(ctx context.Context, modelIDs []int64) (map[int64]*WorkTimeTargetModel, error) {
	workTimeModels := make(map[int64]*WorkTimeTargetModel, len(modelIDs))
	if len(modelIDs) == 0 {
		return workTimeModels, nil
	}
	models, err := s.workModelRepo.FindByIDs(ctx, modelIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to prefetch work time models: %w", err)
	}
	for _, model := range models {
		workTimeModels[model.ID] = model
	}
	return workTimeModels, nil
}

// prefetchDayTargets loads the day-level Soll inputs over the prefetch window:
// the non-working days and the Sonderarbeitszeiten (#3259).
func (s *staffOverviewService) prefetchDayTargets(ctx context.Context, prefetch *monthPrefetch, staffIDs []int64) (err error) {
	if s.holidayReader != nil {
		if prefetch.holidays, err = s.holidayReader.HolidayDates(ctx, prefetch.from, prefetch.to); err != nil {
			return fmt.Errorf("failed to prefetch non-working days: %w", err)
		}
	}
	if s.overrideReader != nil {
		if prefetch.overrides, err = s.overrideReader.StaffTargetOverrideDays(ctx, staffIDs, prefetch.from.String(), prefetch.to.String()); err != nil {
			return fmt.Errorf("failed to prefetch target overrides: %w", err)
		}
	}
	return nil
}

// prefetchSnapshots loads the frozen months of the given staff members.
//
// Deliberately unbounded in time: the splice asks for "the newest snapshot at
// or before month M" for several different M, so a month-bounded prefetch could
// not answer them all. At most one active row exists per staff member and
// month, so the set stays small. Workforce supplies the tenant-scoped batch.
func (s *staffOverviewService) prefetchSnapshots(
	ctx context.Context,
	staffIDs []int64,
) (map[int64][]*MonthSnapshot, error) {
	result := make(map[int64][]*MonthSnapshot, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	snapshots, err := s.snapshotRepo.ClosedMonthSnapshotsForStaff(ctx, staffIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to prefetch month balance snapshots: %w", err)
	}
	for _, snapshot := range snapshots {
		result[snapshot.StaffID] = append(result[snapshot.StaffID], snapshot)
	}
	return result, nil
}
