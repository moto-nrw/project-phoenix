package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *staffOverviewService) GetDashboardSummary(ctx context.Context, period string) (*DashboardSummary, error) {
	if period == "" {
		period = OverviewPeriodMonth
	}
	if period != OverviewPeriodWeek && period != OverviewPeriodMonth {
		return nil, fmt.Errorf("%w: period must be %s or %s", ErrOverviewInvalid, OverviewPeriodWeek, OverviewPeriodMonth)
	}

	today := s.today()
	key := monthOf(today)
	summary := &DashboardSummary{}

	staffMembers, err := s.activeStaff(ctx)
	if err != nil {
		return nil, err
	}
	summary.ActiveStaffCount = len(staffMembers)
	activeIDs := make(map[int64]bool, len(staffMembers))
	for _, staff := range staffMembers {
		activeIDs[staff.ID] = true
	}

	if err := s.addTodayCounters(ctx, summary, activeIDs, today); err != nil {
		return nil, err
	}
	pending, err := s.countPendingRequests(ctx)
	if err != nil {
		return nil, err
	}
	summary.PendingRequestsCount = pending

	if len(staffMembers) == 0 {
		return summary, nil
	}

	// period=week can reach into the previous month, so the prefetch window
	// has to start no later than that Monday.
	var lower *calendar.Date
	weekStart := today.StartOfISOWeek()
	if period == OverviewPeriodWeek {
		lower = &weekStart
	}
	prefetch, err := s.buildPrefetch(ctx, staffMembers, lower, key)
	if err != nil {
		return nil, err
	}
	totals := dashboardTotals{monthSvc: newPrefetchedMonthService(prefetch, s.logger), period: period, key: key, weekStart: weekStart, weekEnd: dashboardWeekEnd(weekStart, today)}
	for _, staff := range staffMembers {
		if err := totals.add(ctx, summary, staff.ID); err != nil {
			return nil, err
		}
	}
	return summary, nil
}

// dashboardWeekEnd clamps the ISO week at today so Soll and Ist are
// comparable, mirroring the month card's target_to_date.
func dashboardWeekEnd(weekStart, today calendar.Date) calendar.Date {
	weekEnd := weekStart.AddDays(6)
	if weekEnd.After(today) {
		weekEnd = today
	}
	return weekEnd
}

// dashboardTotals adds one staff member after another to the dashboard's
// school saldo and period totals.
type dashboardTotals struct {
	monthSvc           WorkTimeMonthService
	period             string
	key                monthKey
	weekStart, weekEnd calendar.Date
}

func (t dashboardTotals) add(ctx context.Context, summary *DashboardSummary, staffID int64) error {
	monthSummary, err := t.monthSvc.GetMonthSummary(ctx, staffID, t.key.Year, t.key.Month)
	if err != nil {
		return fmt.Errorf("failed to compute month summary for staff %d: %w", staffID, err)
	}
	summary.SaldoSchoolTotalMinutes += monthSummary.ClosingBalanceMinutes

	if t.period == OverviewPeriodMonth {
		summary.Period.SollMinutes += monthSummary.TargetMinutesToDate
		summary.Period.IstMinutes += monthSummary.ActualMinutes
		summary.Period.DeltaMinutes += monthSummary.BalanceMinutes
		return nil
	}
	week, err := t.monthSvc.GetRangeAggregate(ctx, staffID, t.weekStart, t.weekEnd)
	if err != nil {
		return fmt.Errorf("failed to compute week aggregate for staff %d: %w", staffID, err)
	}
	summary.Period.SollMinutes += week.TargetMinutes
	summary.Period.IstMinutes += week.ActualMinutes
	summary.Period.DeltaMinutes += week.BalanceMinutes
	return nil
}

// addTodayCounters fills the presence and absence KPIs.
//
// Every set is intersected with the active-staff IDs: the presence, absence and
// shift lookups do not join users.staff, and a leaver's sessions and absences
// survive offboarding, so counting them raw would overstate each KPI.
func (s *staffOverviewService) addTodayCounters(
	ctx context.Context,
	summary *DashboardSummary,
	activeIDs map[int64]bool,
	today calendar.Date,
) error {
	clockedIn, err := s.clockedInCount(ctx, activeIDs, today)
	if err != nil {
		return err
	}
	summary.CurrentlyClockedIn = clockedIn

	absences, err := s.absenceRepo.StaffAbsenceMapForDate(ctx, today.String())
	err = rowReadError("get effective absences for date", err, nil)
	if err != nil {
		return fmt.Errorf("failed to load absence map: %w", err)
	}
	countAbsencesToday(summary, absences, activeIDs)

	expected, err := s.expectedClockedIn(ctx, activeIDs, today)
	if err != nil {
		return err
	}
	summary.ExpectedClockedIn = expected
	return nil
}

// clockedInCount counts the active staff members at work right now.
func (s *staffOverviewService) clockedInCount(ctx context.Context, activeIDs map[int64]bool, today calendar.Date) (int, error) {
	presence, err := s.sessionRepo.WorkPresenceMap(ctx)
	err = rowReadError("get today presence map", err, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to load presence map: %w", err)
	}
	clockedIn := make(map[int64]bool, len(presence))
	for staffID, status := range presence {
		// The map stores the session status for an open session and the
		// sentinel "checked_out" otherwise; match the open statuses
		// positively so a new status value cannot be miscounted as present.
		if !activeIDs[staffID] {
			continue
		}
		if status == WorkSessionStatusPresent || status == WorkSessionStatusHomeOffice {
			clockedIn[staffID] = true
		}
	}
	// A session opened before midnight and never closed is still someone at
	// work, but it is not in today's presence map.
	openSessionsRecords, err := s.sessionRepo.ListWorkSessions(ctx, workforce.WorkSessionFilter{DateBefore: today.String(), Open: boolPointer(true)})
	openSessions := workSessionsToRecords(openSessionsRecords)
	err = rowReadError("get open sessions", err, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to load open sessions: %w", err)
	}
	for _, session := range openSessions {
		if activeIDs[session.StaffID] {
			clockedIn[session.StaffID] = true
		}
	}
	return len(clockedIn), nil
}

// countAbsencesToday counts the active staff members sick or on vacation
// today. The map keeps ONE type per staff member by priority
// (sick > training > vacation > comp_time > other), so somebody with
// overlapping sick and vacation counts as sick only and the two counters
// never double-count a person.
func countAbsencesToday(summary *DashboardSummary, absences map[int64]string, activeIDs map[int64]bool) {
	for staffID, absenceType := range absences {
		if !activeIDs[staffID] {
			continue
		}
		switch absenceType {
		case AbsenceTypeSick:
			summary.SickToday++
		case AbsenceTypeVacation:
			summary.VacationToday++
		}
	}
}

// expectedClockedIn counts staff members whose planned shift covers this very
// moment. Shift rows carry wall-clock TIME columns, normalized by the repo.
func (s *staffOverviewService) expectedClockedIn(
	ctx context.Context,
	activeIDs map[int64]bool,
	today calendar.Date,
) (int, error) {
	shiftDate := calendar.Date(today)
	shifts, err := s.shiftRepo.FindByDateRange(ctx, shiftDate, shiftDate)
	if err != nil {
		return 0, fmt.Errorf("failed to load today's shifts: %w", err)
	}
	nowWall := calendar.NormalizeWallClock(calendar.Now())
	expected := make(map[int64]bool)
	for _, shift := range shifts {
		if shift.Cancelled || !activeIDs[shift.StaffID] {
			continue
		}
		start, end := calendar.NormalizeWallClock(shift.StartTime), calendar.NormalizeWallClock(shift.EndTime)
		if !nowWall.Before(start) && !nowWall.After(end) {
			expected[shift.StaffID] = true
		}
	}
	return len(expected), nil
}

// countPendingRequests counts absences awaiting a decision — the same set
// ListPendingRequests returns, counted through the generic filter API so no
// rows are loaded.
func (s *staffOverviewService) countPendingRequests(ctx context.Context) (int, error) {
	count, err := s.absenceRepo.CountStaffAbsences(ctx, workforce.StaffAbsenceFilter{
		Statuses: []string{AbsenceStatusRequested, AbsenceStatusQuestion},
	})
	err = rowReadError("count with options", err, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to count pending absence requests: %w", err)
	}
	return count, nil
}
