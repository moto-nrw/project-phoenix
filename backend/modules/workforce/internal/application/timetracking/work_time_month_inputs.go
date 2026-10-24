package timetracking

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// addAdjustments folds the Stundenkonto transactions (#1420) into their
// effective month. Future-dated adjustments are skipped like future targets
// and credits — a grant scheduled for next week must not pre-empt today's
// balance; the carry chain picks it up once the date arrives.
func (s *workTimeMonthService) addAdjustments(ctx context.Context, staffID int64, first, last, today calendar.Date, aggregates map[monthKey]*monthAggregates) error {
	if s.adjustmentRepo == nil {
		return nil
	}
	adjustmentValues, err := s.adjustmentRepo.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{StaffID: staffID, EffectiveFrom: first.String(), EffectiveTo: last.String()})
	adjustments := adjustmentsToRecords(adjustmentValues)
	err = rowReadError("get balance adjustments by staff+range", err, nil)
	if err != nil {
		return fmt.Errorf("failed to load balance adjustments: %w", err)
	}
	for _, adjustment := range adjustments {
		if adjustment.EffectiveDate.After(today) {
			continue
		}
		agg, ok := aggregates[monthOf(adjustment.EffectiveDate)]
		if !ok {
			continue
		}
		agg.adjustment += adjustment.MinutesDelta
		agg.adjustments = append(agg.adjustments, adjustment)
	}
	return nil
}

// addActualMinutes assigns each session's time to every Berlin calendar day it
// intersects. A work session is filed under its check-in date, but may validly
// run past midnight; counting it only in that filing month would lose the
// after-midnight part from the following day's balance.
func (s *workTimeMonthService) addActualMinutes(ctx context.Context, staffID int64, first, last, today calendar.Date, aggregates map[monthKey]*monthAggregates) error {
	end := last.AddDays(1).BerlinMidnight()
	sessionsRecords, err := s.sessionRepo.ListOverlappingWorkSessions(ctx, []int64{staffID}, first.BerlinMidnight(), &end)
	sessions := workSessionsToRecords(sessionsRecords)
	err = rowReadError("list overlapping by staff ID", err, nil)
	if err != nil {
		return fmt.Errorf("failed to load work sessions: %w", err)
	}
	now := s.now()
	breaksBySessionID, err := s.breaksBySessionID(ctx, sessions)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		end := balanceSessionEnd(session, now, today)
		if !end.After(session.CheckInTime) {
			continue
		}

		for day, minutes := range netMinutesByDate(session, breaksBySessionID[session.ID], end, first, today) {
			if agg, ok := aggregates[monthOf(day)]; ok {
				agg.actual += minutes
			}
		}
	}
	return nil
}

func (s *workTimeMonthService) breaksBySessionID(ctx context.Context, sessions []*WorkSession) (map[int64][]*WorkSessionBreak, error) {
	ids := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		ids = append(ids, session.ID)
	}
	if len(ids) == 0 {
		return map[int64][]*WorkSessionBreak{}, nil
	}
	var breaksRecords []workforce.WorkSessionBreak
	var breakErr error
	if len(ids) > 0 {
		breaksRecords, breakErr = s.breakRepo.ListWorkSessionBreaks(ctx, workforce.WorkSessionBreakFilter{SessionIDs: ids})
	}
	err := breakErr
	breaks := workBreaksBySession(breaksRecords)
	err = rowReadError("get breaks by session IDs", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load work session breaks: %w", err)
	}
	return breaks, nil
}

// sessionEndUpTo clamps a session end at now. This prevents a future
// check-out from being credited before it happens while preserving valid work
// across Berlin midnight.
func sessionEndUpTo(session *WorkSession, now time.Time) time.Time {
	end := now
	if session.CheckOutTime != nil && session.CheckOutTime.Before(end) {
		end = *session.CheckOutTime
	}
	return end
}

// addAbsenceCredits credits sick/vacation/training days with the day's
// contractual target (Ansatz B, mirroring the frontend engine): half-day
// boundaries credit floor(target/2), overlapping absences credit a day only
// once (lowest absence ID wins), and only reported/approved absences count.
// Day counters are schedule-aware: a vacation day only counts when the day
// actually had a target (> 0), so part-timers aren't charged for days off.
func (s *workTimeMonthService) addAbsenceCredits(ctx context.Context, staffID int64, first, last, today calendar.Date, resolver *dailyTargetResolver, aggregates map[monthKey]*monthAggregates) error {
	absenceValues, err := s.absenceRepo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffID: staffID, OverlapFrom: first.String(), OverlapTo: last.String(), Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}}})
	absences := absencesToRecord(absenceValues)
	err = rowReadError("get absences by staff and date range", err, nil)
	if err != nil {
		return fmt.Errorf("failed to load absences: %w", err)
	}
	through := last
	if today.Before(through) {
		through = today
	}
	walkCreditedAbsenceDays(absences, first, through, resolver.TargetFor,
		func(d calendar.Date, absence *StaffAbsence, credit int, fraction float64) {
			creditAbsenceDay(absence, credit, fraction, aggregates[monthOf(d)])
		})
	return nil
}

// absenceDayVisitor receives one effectively covered absence day together with
// the minutes it credits (already 0 for Freizeitausgleich, already halved on a
// half-day boundary) and the day fraction that credit represents.
func (s *workTimeMonthService) addPlannedShifts(ctx context.Context, staffID int64, first, last calendar.Date, aggregates map[monthKey]*monthAggregates) error {
	shifts, err := s.shiftRepo.FindByStaffAndDateRange(ctx, staffID, calendar.Date(first), calendar.Date(last))
	if err != nil {
		return fmt.Errorf("failed to load staff shifts: %w", err)
	}
	for _, shift := range shifts {
		agg, ok := aggregates[monthOf(calendar.Date(shift.Date))]
		if !ok {
			continue
		}
		agg.hasShifts = true
		if !shift.Cancelled {
			agg.plannedShift += shiftNetMinutes(shift)
		}
	}
	return nil
}

// shiftNetMinutes is the planned working time of one shift: wall-clock span
// minus break, floored at 0. Local copy of the (unexported) helper in
// services/schedule — that package imports this one, so it cannot be shared
// without an import cycle.
func shiftNetMinutes(shift *TimeTrackingShift) int {
	start := calendar.NormalizeWallClock(shift.StartTime)
	end := calendar.NormalizeWallClock(shift.EndTime)
	minutes := int(end.Sub(start)/time.Minute) - shift.BreakMinutes
	if minutes < 0 {
		return 0
	}
	return minutes
}

// ---- carry chain + summary ---------------------------------------------------
