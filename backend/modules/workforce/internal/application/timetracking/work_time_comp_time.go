package timetracking

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// GetBalanceReductionCapacity treats adjustments as start-of-day ledger
// entries. It checks the opening balance on effectiveDate, every later debit
// already in the ledger, current balance for backdated entries, and every
// effective comp-time absence commitment through the supported future
// horizon. The caller holds the shared staff-balance lock, so these reads
// cannot race another balance-affecting write.
func (s *workTimeMonthService) GetBalanceReductionCapacity(ctx context.Context, staffID int64, effectiveDate calendar.Date) (int, error) {
	if staffID <= 0 || effectiveDate.IsZero() {
		return 0, errors.New("staff id and effective date are required")
	}

	today := s.today()
	last := monthOf(today).addMonths(maxFutureMonths).lastDay()
	if effectiveDate.After(last) {
		return 0, fmt.Errorf("effective date %s is outside the supported balance horizon", effectiveDate.String())
	}

	checkpointSet := map[calendar.Date]struct{}{effectiveDate: {}}
	if err := s.addAdjustmentReductionCheckpoints(ctx, staffID, effectiveDate, last, checkpointSet); err != nil {
		return 0, err
	}
	compTimeFrom := effectiveDate
	if tomorrow := today.AddDays(1); effectiveDate.After(today) {
		compTimeFrom = tomorrow
	}
	compTimeAbsences, err := s.addCompTimeReductionCheckpoints(ctx, staffID, compTimeFrom, last, checkpointSet)
	if err != nil {
		return 0, err
	}

	checkpoints := make([]calendar.Date, 0, len(checkpointSet))
	for checkpoint := range checkpointSet {
		checkpoints = append(checkpoints, checkpoint)
	}
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Before(checkpoints[j])
	})

	capacity, err := s.minimumOpeningReductionCapacity(ctx, staffID, checkpoints, compTimeAbsences, today)
	if err != nil {
		return 0, err
	}

	if !effectiveDate.After(today) {
		currentBalance, err := s.GetClosingBalanceAsOf(ctx, staffID, today)
		if err != nil {
			return 0, fmt.Errorf("failed to compute current balance for reduction validation: %w", err)
		}
		capacity = min(capacity, currentBalance)
		minimumHistoricalClosing, err := s.getMinimumDailyClosingBalance(ctx, staffID, effectiveDate, today)
		if err != nil {
			return 0, err
		}
		capacity = min(capacity, minimumHistoricalClosing)
	}

	return capacity, nil
}

func (s *workTimeMonthService) addAdjustmentReductionCheckpoints(
	ctx context.Context,
	staffID int64,
	effectiveDate, last calendar.Date,
	checkpoints map[calendar.Date]struct{},
) error {
	if s.adjustmentRepo == nil {
		return nil
	}
	adjustmentValues, err := s.adjustmentRepo.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{StaffID: staffID, EffectiveFrom: effectiveDate.String(), EffectiveTo: last.String()})
	adjustments := adjustmentsToRecords(adjustmentValues)
	err = rowReadError("get balance adjustments by staff+range", err, nil)
	if err != nil {
		return fmt.Errorf("failed to load balance adjustments for reduction validation: %w", err)
	}
	for _, adjustment := range adjustments {
		if adjustment.MinutesDelta < 0 && adjustment.EffectiveDate.After(effectiveDate) {
			checkpoints[adjustment.EffectiveDate] = struct{}{}
		}
	}
	return nil
}

// GetFutureCompTimeCommitmentMinutes sums the unrealized deductions of every
// effective comp-time absence from tomorrow through the balance horizon.
func (s *workTimeMonthService) GetFutureCompTimeCommitmentMinutes(ctx context.Context, staffID int64) (int, error) {
	if staffID <= 0 {
		return 0, errors.New("staff id is required")
	}
	if s.absenceRepo == nil {
		return 0, nil
	}
	today := s.today()
	from := today.AddDays(1)
	last := monthOf(today).addMonths(maxFutureMonths).lastDay()
	checkpoints := map[calendar.Date]struct{}{}
	compTimeAbsences, err := s.addCompTimeReductionCheckpoints(ctx, staffID, from, last, checkpoints)
	if err != nil {
		return 0, err
	}
	return s.getCompTimeDeductionInRange(ctx, staffID, from, last, compTimeAbsences)
}

func (s *workTimeMonthService) addCompTimeReductionCheckpoints(
	ctx context.Context,
	staffID int64,
	effectiveDate, last calendar.Date,
	checkpoints map[calendar.Date]struct{},
) ([]*StaffAbsence, error) {
	if s.absenceRepo == nil {
		return nil, nil
	}
	absenceValues, err := s.absenceRepo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffID: staffID, OverlapFrom: effectiveDate.String(), OverlapTo: last.String(), Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}}})
	absences := absencesToRecord(absenceValues)
	err = rowReadError("get absences by staff and date range", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load comp-time absences for reduction validation: %w", err)
	}
	compTimeAbsences := make([]*StaffAbsence, 0, len(absences))
	for _, absence := range absences {
		if absence.AbsenceType != AbsenceTypeCompTime ||
			(absence.Status != AbsenceStatusReported && absence.Status != AbsenceStatusApproved) {
			continue
		}
		compTimeAbsences = append(compTimeAbsences, absence)
		checkpoint := absence.DateStart
		if checkpoint.Before(effectiveDate) {
			checkpoint = effectiveDate
		}
		checkpoints[checkpoint] = struct{}{}
	}
	return compTimeAbsences, nil
}

func (s *workTimeMonthService) minimumOpeningReductionCapacity(
	ctx context.Context,
	staffID int64,
	checkpoints []calendar.Date,
	compTimeAbsences []*StaffAbsence,
	today calendar.Date,
) (int, error) {
	capacity := int(^uint(0) >> 1)
	for _, checkpoint := range checkpoints {
		available, err := s.getOpeningBalanceOnDate(ctx, staffID, checkpoint, today, compTimeAbsences)
		if err != nil {
			return 0, err
		}
		commitment, err := s.getRemainingCompTimeCommitment(ctx, staffID, checkpoint, compTimeAbsences)
		if err != nil {
			return 0, err
		}
		capacity = min(capacity, available-commitment)
	}
	return capacity, nil
}

func (s *workTimeMonthService) getCompTimeDeductionInRange(
	ctx context.Context,
	staffID int64,
	from, to calendar.Date,
	absences []*StaffAbsence,
) (int, error) {
	if to.Before(from) {
		return 0, nil
	}
	ranges := clippedCompTimeRanges(absences, from, to)
	if len(ranges) == 0 {
		return 0, nil
	}
	loadFrom, loadTo := ranges[0].start, ranges[0].end
	for _, dateRange := range ranges[1:] {
		if dateRange.start.Before(loadFrom) {
			loadFrom = dateRange.start
		}
		if dateRange.end.After(loadTo) {
			loadTo = dateRange.end
		}
	}
	resolver, actualByDate, err := s.loadCompTimeDeductionInputs(ctx, staffID, loadFrom, loadTo)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, dateRange := range ranges {
		total += compTimeDeductionMinutes(resolver, actualByDate, dateRange.start, dateRange.end)
	}
	return total, nil
}

type compTimeRange struct {
	start calendar.Date
	end   calendar.Date
}

func clippedCompTimeRanges(absences []*StaffAbsence, from, to calendar.Date) []compTimeRange {
	ranges := make([]compTimeRange, 0, len(absences))
	for _, absence := range absences {
		start, end := absence.DateStart, absence.DateEnd
		if start.Before(from) {
			start = from
		}
		if end.After(to) {
			end = to
		}
		if !end.Before(start) {
			ranges = append(ranges, compTimeRange{start: start, end: end})
		}
	}
	return ranges
}

// getMinimumDailyClosingBalance returns the lowest end-of-day balance in the
// historical range. A backdated debit lowers every closing balance from its
// effective date onward, so checking only that day's opening and today's
// closing can miss an intervening deficit that later work repaired.
func (s *workTimeMonthService) getMinimumDailyClosingBalance(
	ctx context.Context,
	staffID int64,
	from, to calendar.Date,
) (int, error) {
	anchor, err := s.chainAnchor(ctx, monthOf(to))
	if err != nil {
		return 0, fmt.Errorf("failed to resolve account start for daily reduction validation: %w", err)
	}

	balance := 0
	if cutoff := from.AddDays(-1); !cutoff.Before(anchor) {
		balance, err = s.GetClosingBalanceAsOf(ctx, staffID, cutoff)
		if err != nil {
			return 0, fmt.Errorf("failed to compute balance before %s: %w", from.String(), err)
		}
	}

	deltas, err := s.getDailyBalanceDeltas(ctx, staffID, from, to)
	if err != nil {
		return 0, err
	}
	// A frozen month boundary inside the range RESTARTS the chain at the
	// frozen closing balance (#1417). Accumulating live deltas straight
	// across it would report a minimum that is off by that month's drift, and
	// the payout/comp-time capacity checks in #1420 rely on this number.
	boundaries, err := s.frozenMonthBoundaries(ctx, staffID, from, to)
	if err != nil {
		return 0, err
	}
	minimum := int(^uint(0) >> 1)
	for d := from; !d.After(to); d = d.AddDays(1) {
		if opening, ok := boundaries[d]; ok && d.After(anchor) {
			balance = opening
		}
		balance += deltas[d]
		minimum = min(minimum, balance)
	}
	return minimum, nil
}

// frozenMonthBoundaries maps each month-start day inside [from, to] to the
// frozen closing balance of the preceding month, for the days where the carry
// chain restarts instead of continuing. `from` is a boundary only when it is
// itself the first day of a month.
func (s *workTimeMonthService) frozenMonthBoundaries(
	ctx context.Context,
	staffID int64,
	from, to calendar.Date,
) (map[calendar.Date]int, error) {
	boundaries := make(map[calendar.Date]int)
	if s.snapshotRepo == nil {
		return boundaries, nil
	}
	first := monthOf(from)
	if first.firstDay().Before(from) {
		first = first.next()
	}
	for k := first; !monthOf(to).before(k); k = k.next() {
		prev := k.addMonths(-1)
		snapshot, err := s.snapshotRepo.LatestClosedMonth(ctx, staffID, prev.Year, prev.Month)
		if err != nil {
			return nil, fmt.Errorf("failed to load month balance snapshot for %d-%02d: %w", prev.Year, prev.Month, err)
		}
		if snapshot == nil || snapshot.Year != prev.Year || snapshot.Month != prev.Month {
			continue
		}
		boundaries[k.firstDay()] = snapshot.ClosingBalanceMinutes
	}
	return boundaries, nil
}

// getDailyBalanceDeltas is deliberately snapshot-agnostic (#1417): it reports
// what happened on each day. A frozen month is not a per-day event, so folding
// it in here would double-count against the boundary reset in
// getMinimumDailyClosingBalance.
func (s *workTimeMonthService) getDailyBalanceDeltas(
	ctx context.Context,
	staffID int64,
	from, to calendar.Date,
) (map[calendar.Date]int, error) {
	resolver, err := s.buildTargetResolver(ctx, staffID, from, to)
	if err != nil {
		return nil, err
	}
	deltas := make(map[calendar.Date]int, from.DaysUntil(to)+1)
	for d := from; !d.After(to); d = d.AddDays(1) {
		deltas[d] = -resolver.TargetFor(d)
	}
	if err := s.addDailyActualMinutes(ctx, staffID, from, to, deltas); err != nil {
		return nil, err
	}
	if err := s.addDailyAbsenceCredits(ctx, staffID, from, to, resolver, deltas); err != nil {
		return nil, err
	}
	if err := s.addDailyAdjustments(ctx, staffID, from, to, deltas); err != nil {
		return nil, err
	}
	anchor, err := s.chainAnchor(ctx, monthOf(from))
	if err != nil {
		return nil, err
	}
	for d := from; !d.After(to); d = d.AddDays(1) {
		if excludedByAccountStart(d, anchor) {
			deltas[d] = 0
		}
	}
	return deltas, nil
}

func (s *workTimeMonthService) addDailyActualMinutes(
	ctx context.Context,
	staffID int64,
	from, to calendar.Date,
	deltas map[calendar.Date]int,
) error {
	actualByDate, err := s.getDailyActualMinutes(ctx, staffID, from, to)
	if err != nil {
		return err
	}
	for date, minutes := range actualByDate {
		deltas[date] += minutes
	}
	return nil
}

func (s *workTimeMonthService) getDailyActualMinutes(
	ctx context.Context,
	staffID int64,
	from, to calendar.Date,
) (map[calendar.Date]int, error) {
	end := to.AddDays(1).BerlinMidnight()
	sessionsRecords, err := s.sessionRepo.ListOverlappingWorkSessions(ctx, []int64{staffID}, from.BerlinMidnight(), &end)
	sessions := workSessionsToRecords(sessionsRecords)
	err = rowReadError("list overlapping by staff ID", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load work sessions for daily balance calculation: %w", err)
	}
	actualByDate := make(map[calendar.Date]int)
	now := s.now()
	breaksBySessionID, err := s.breaksBySessionID(ctx, sessions)
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		for day, minutes := range netMinutesByDate(session, breaksBySessionID[session.ID], balanceSessionEnd(session, now, s.today()), from, to) {
			actualByDate[day] += minutes
		}
	}
	return actualByDate, nil
}

func (s *workTimeMonthService) addDailyAbsenceCredits(
	ctx context.Context,
	staffID int64,
	from, to calendar.Date,
	resolver *dailyTargetResolver,
	deltas map[calendar.Date]int,
) error {
	credits, err := s.getDailyAbsenceCredits(ctx, staffID, from, to, to, resolver)
	if err != nil {
		return err
	}
	for d, credit := range credits {
		deltas[d] += credit
	}
	return nil
}

// getDailyAbsenceCredits is the per-day credit map behind both the daily
// deltas and the table's Gutschrift column (#2443). `through` is the last
// creditable day (see walkCreditedAbsenceDays).
func (s *workTimeMonthService) getDailyAbsenceCredits(
	ctx context.Context,
	staffID int64,
	from, to, through calendar.Date,
	resolver *dailyTargetResolver,
) (map[calendar.Date]int, error) {
	absenceValues, err := s.absenceRepo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffID: staffID, OverlapFrom: from.String(), OverlapTo: to.String(), Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}}})
	absences := absencesToRecord(absenceValues)
	err = rowReadError("get absences by staff and date range", err, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load absences for daily credits: %w", err)
	}
	credits := make(map[calendar.Date]int)
	walkCreditedAbsenceDays(absences, from, through, resolver.TargetFor,
		func(d calendar.Date, _ *StaffAbsence, credit int, _ float64) {
			credits[d] += credit
		})
	return credits, nil
}

func (s *workTimeMonthService) addDailyAdjustments(
	ctx context.Context,
	staffID int64,
	from, to calendar.Date,
	deltas map[calendar.Date]int,
) error {
	if s.adjustmentRepo == nil {
		return nil
	}
	adjustmentValues, err := s.adjustmentRepo.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{StaffID: staffID, EffectiveFrom: from.String(), EffectiveTo: to.String()})
	adjustments := adjustmentsToRecords(adjustmentValues)
	err = rowReadError("get balance adjustments by staff+range", err, nil)
	if err != nil {
		return fmt.Errorf("failed to load adjustments for daily reduction validation: %w", err)
	}
	for _, adjustment := range adjustments {
		deltas[adjustment.EffectiveDate] += adjustment.MinutesDelta
	}
	return nil
}

func (s *workTimeMonthService) getRemainingCompTimeCommitment(
	ctx context.Context,
	staffID int64,
	from calendar.Date,
	absences []*StaffAbsence,
) (int, error) {
	remaining := make([]*StaffAbsence, 0, len(absences))
	var through calendar.Date
	for _, absence := range absences {
		if from.Before(absence.DateStart) || from.After(absence.DateEnd) {
			continue
		}
		remaining = append(remaining, absence)
		if through.IsZero() || absence.DateEnd.After(through) {
			through = absence.DateEnd
		}
	}
	if len(remaining) == 0 {
		return 0, nil
	}
	return s.getCompTimeDeductionInRange(ctx, staffID, from, through, remaining)
}

// GetCompTimeDeductionMinutes sums the missing daily target minutes a
// comp_time absence can remove from the Stundenkonto. Neither a full-day nor a
// half-day absence credits time, so recorded net work reduces the deduction by
// the same amount it contributes to the Monatskarte. Future days still reserve
// the full target.
func (s *workTimeMonthService) GetCompTimeDeductionMinutes(ctx context.Context, staffID int64, start, end calendar.Date, _ bool) (int, error) {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return 0, errors.New("start and end must form a valid range")
	}
	resolver, actualByDate, err := s.loadCompTimeDeductionInputs(ctx, staffID, start, end)
	if err != nil {
		return 0, err
	}
	return compTimeDeductionMinutes(resolver, actualByDate, start, end), nil
}

func (s *workTimeMonthService) loadCompTimeDeductionInputs(
	ctx context.Context,
	staffID int64,
	start, end calendar.Date,
) (*dailyTargetResolver, map[calendar.Date]int, error) {
	resolver, err := s.buildTargetResolver(ctx, staffID, start, end)
	if err != nil {
		return nil, nil, err
	}
	actualByDate := make(map[calendar.Date]int)
	actualEnd := end
	if today := s.today(); actualEnd.After(today) {
		actualEnd = today
	}
	if !actualEnd.Before(start) {
		actualByDate, err = s.getDailyActualMinutes(ctx, staffID, start, actualEnd)
		if err != nil {
			return nil, nil, err
		}
	}
	return resolver, actualByDate, nil
}

func compTimeDeductionMinutes(
	resolver *dailyTargetResolver,
	actualByDate map[calendar.Date]int,
	start, end calendar.Date,
) int {
	total := 0
	for d := start; !d.After(end); d = d.AddDays(1) {
		target := resolver.TargetFor(d)
		if target <= 0 {
			continue
		}
		deduction := max(0, target-actualByDate[d])
		total += deduction
	}
	return total
}
