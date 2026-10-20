package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *workTimeMonthService) getOpeningBalanceOnDate(
	ctx context.Context,
	staffID int64,
	date, today calendar.Date,
	compTimeAbsences []*StaffAbsence,
) (int, error) {
	anchor, err := s.chainAnchor(ctx, monthOf(today))
	if err != nil {
		return 0, fmt.Errorf("failed to resolve account start for reduction validation: %w", err)
	}
	if date.After(today) {
		return s.projectedOpeningBalance(ctx, staffID, date, today, anchor, compTimeAbsences)
	}

	available, err := s.closingBalanceBefore(ctx, staffID, date, anchor)
	if err != nil {
		return 0, err
	}
	sameDayAdjustments, err := s.GetBalanceAdjustmentMinutes(ctx, staffID, date, date)
	if err != nil {
		return 0, fmt.Errorf("failed to compute adjustments on %s: %w", date.String(), err)
	}
	return available + sameDayAdjustments, nil
}

// projectedOpeningBalance is the opening balance of a future day: today's
// balance plus the adjustments up to the day, minus the comp time booked
// between them.
func (s *workTimeMonthService) projectedOpeningBalance(
	ctx context.Context,
	staffID int64,
	date, today, anchor calendar.Date,
	compTimeAbsences []*StaffAbsence,
) (int, error) {
	available := 0
	if !today.Before(anchor) {
		var err error
		available, err = s.GetClosingBalanceAsOf(ctx, staffID, today)
		if err != nil {
			return 0, fmt.Errorf("failed to compute current balance for reduction validation: %w", err)
		}
	}
	futureAdjustments, err := s.GetBalanceAdjustmentMinutes(ctx, staffID, today.AddDays(1), date)
	if err != nil {
		return 0, fmt.Errorf("failed to compute adjustments through %s: %w", date.String(), err)
	}
	available += futureAdjustments
	priorCompTime, err := s.getCompTimeDeductionInRange(
		ctx,
		staffID,
		today.AddDays(1),
		date.AddDays(-1),
		compTimeAbsences,
	)
	if err != nil {
		return 0, err
	}
	return available - priorCompTime, nil
}

// closingBalanceBefore is the balance at the end of the day before date: a
// frozen month boundary when there is one, the live chain otherwise, and zero
// before the account start.
func (s *workTimeMonthService) closingBalanceBefore(ctx context.Context, staffID int64, date, anchor calendar.Date) (int, error) {
	cutoff := date.AddDays(-1)
	if cutoff.Before(anchor) {
		return 0, nil
	}
	boundaries, err := s.frozenMonthBoundaries(ctx, staffID, date, date)
	if err != nil {
		return 0, fmt.Errorf("failed to load opening balance on %s: %w", date.String(), err)
	}
	if frozen, ok := boundaries[date]; ok {
		return frozen, nil
	}
	available, err := s.GetClosingBalanceAsOf(ctx, staffID, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to compute opening balance on %s: %w", date.String(), err)
	}
	return available, nil
}

func (s *workTimeMonthService) getMonthSummaryThrough(ctx context.Context, staffID int64, key monthKey, through calendar.Date) (*MonthSummary, error) {
	anchor, err := s.chainAnchor(ctx, key)
	if err != nil {
		return nil, err
	}
	start, err := s.carryChainStart(ctx, staffID, key, anchor)
	if err != nil {
		return nil, err
	}

	aggregates, err := s.computeAggregates(ctx, staffID, start.first, key, through)
	if err != nil {
		return nil, err
	}

	carry := start.carry
	for k := start.key; k.before(key); k = k.next() {
		carry += aggregates[k].balance()
	}

	summary := newMonthSummary(staffID, key, carry, aggregates[key])
	applyChainSplice(summary, start.splice)
	return summary, nil
}

// carryChain is where the carry chain of a requested month starts: the first
// month and day it aggregates, the balance it starts with, and the frozen
// splice that shaped it.
type carryChain struct {
	key    monthKey
	first  calendar.Date
	carry  int
	splice *chainSplice
}

// carryChainStart resolves where the carry chain of key starts.
func (s *workTimeMonthService) carryChainStart(ctx context.Context, staffID int64, key monthKey, anchor calendar.Date) (carryChain, error) {
	// The chain starts on the account start date; a month entirely before the
	// account start is summarized standalone (full month, no carry).
	start := carryChain{key: monthOf(anchor), first: anchor}
	if key.before(start.key) {
		start.key, start.first = key, key.firstDay()
	}

	// A frozen earlier month (#1417) short-circuits the chain: its closing
	// balance IS the opening balance here, so nothing before it is re-summed
	// and a retroactive correction there can no longer move this month.
	splice, err := s.spliceChainStart(ctx, staffID, key, anchor)
	if err != nil {
		return carryChain{}, err
	}
	start.splice = splice
	// Only an EARLIER frozen month shortens the chain. A splice that carries
	// nothing but the requested month's own snapshot (used for drift) must
	// leave the chain running from the account start.
	if splice != nil && splice.carryFrom != nil {
		start.key, start.first = splice.startKey, splice.startKey.firstDay()
		start.carry = splice.frozenCarry
	}

	// Bound how far back the chain reaches so a stale/misconfigured account
	// start can't make each poll aggregate and query decades of days. A
	// realistic account never predates the floor; when it does, the setting is
	// wrong, and we fail loudly instead of clamping — a clamp would drop the
	// balance of every month before the floor and quietly present a wrong
	// opening/closing Stundenkonto (#1842).
	if floorKey := key.addMonths(-maxCarryChainMonths); start.key.before(floorKey) {
		s.getLogger().Error("account start older than carry-chain bound, rejecting",
			"account_start", start.first.String(),
			"floor", floorKey.firstDay().String(),
			"max_carry_chain_months", maxCarryChainMonths,
		)
		return carryChain{}, fmt.Errorf("%w: %s is more than %d months before %d-%02d",
			ErrAccountStartTooOld, start.first.String(), maxCarryChainMonths, key.Year, key.Month)
	}
	return start, nil
}

// newMonthSummary builds the Monatskarte of key from its aggregates and the
// carry into it.
func newMonthSummary(staffID int64, key monthKey, carry int, agg *monthAggregates) *MonthSummary {
	summary := &MonthSummary{
		StaffID:                 staffID,
		Year:                    key.Year,
		Month:                   key.Month,
		CarryInMinutes:          carry,
		TargetMinutes:           agg.target,
		TargetMinutesToDate:     agg.targetToDate,
		ActualMinutes:           agg.actual,
		CreditedSickMinutes:     agg.creditedSick,
		CreditedVacationMinutes: agg.creditedVacation,
		CreditedTrainingMinutes: agg.creditedTraining,
		CreditedOtherMinutes:    agg.creditedOther,
		SickDays:                agg.sickDays,
		VacationDays:            agg.vacationDays,
		TrainingDays:            agg.trainingDays,
		BalanceMinutes:          agg.balance(),
	}
	if agg.hasShifts {
		planned := agg.plannedShift
		summary.PlannedShiftMinutes = &planned
	}
	summary.AdjustmentMinutes = agg.adjustment
	for _, adjustment := range agg.adjustments {
		summary.Adjustments = append(summary.Adjustments, AdjustmentView{
			ID:            adjustment.ID,
			Type:          adjustment.Type,
			MinutesDelta:  adjustment.MinutesDelta,
			EffectiveDate: adjustment.EffectiveDate,
			Note:          adjustment.Note,
			DecidedBy:     adjustment.DecidedBy,
			DecidedAt:     adjustment.DecidedAt,
		})
	}
	summary.ClosingBalanceMinutes = summary.CarryInMinutes + summary.BalanceMinutes
	return summary
}

// applyChainSplice marks a carry taken from a frozen month and the requested
// month's own close with its drift.
func applyChainSplice(summary *MonthSummary, splice *chainSplice) {
	if splice == nil {
		return
	}
	if splice.carryFrom != nil {
		summary.CarryInFrozen = true
		label := fmt.Sprintf("%04d-%02d", splice.carryFrom.Year, splice.carryFrom.Month)
		summary.CarryInFrozenFromMonth = &label
	}
	if own := splice.ownSnapshot; own != nil {
		summary.IsClosed = true
		closedAt, closedBy := own.ClosedAt, own.ClosedBy
		summary.ClosedAt, summary.ClosedBy = &closedAt, &closedBy
		summary.CloseReason = own.CloseReason
		frozen := own.ClosingBalanceMinutes
		summary.FrozenClosingBalanceMinutes = &frozen
		summary.DriftMinutes = summary.ClosingBalanceMinutes - frozen
	}
}

// chainSplice is the outcome of the frozen-month lookup for one requested
// month: where the carry chain starts, what it starts with, and whether the
// requested month is itself frozen.
type chainSplice struct {
	// startKey is the first month the chain must still aggregate.
	startKey monthKey
	// frozenCarry is the opening balance for startKey.
	frozenCarry int
	// carryFrom names the frozen month that produced frozenCarry; nil when no
	// earlier month is frozen and the chain runs from the account start.
	carryFrom *monthKey
	// ownSnapshot is the requested month's own active snapshot, if it is
	// closed. It supplies the drift comparison, never the carry.
	ownSnapshot *MonthSnapshot
}

// spliceChainStart resolves how the carry chain for `key` must start (#1417).
// It returns nil when nothing is frozen and the caller should keep the
// account-anchor chain untouched.
//
// One read covers both questions: the newest active snapshot at or before
// `key` is either `key`'s own (then a second read finds the one before it) or
// already an earlier month (then it seeds the chain directly).
//
// A snapshot before the anchor month is ignored on purpose. The chain never
// starts before the account start, so such a row — left behind by a later
// change to the account-start setting — could only corrupt the card. The
// close path rejects creating one; this is the read-side half of that fence.
func (s *workTimeMonthService) spliceChainStart(
	ctx context.Context,
	staffID int64,
	key monthKey,
	anchor calendar.Date,
) (*chainSplice, error) {
	if s.snapshotRepo == nil {
		return nil, nil
	}
	anchorKey := monthOf(anchor)

	latest, err := s.snapshotRepo.LatestClosedMonth(ctx, staffID, key.Year, key.Month)
	if err != nil {
		return nil, fmt.Errorf("failed to load month balance snapshot: %w", err)
	}
	if latest == nil {
		return nil, nil
	}

	splice := &chainSplice{startKey: key}
	seed := latest
	if latest.Year == key.Year && latest.Month == key.Month {
		splice.ownSnapshot = latest
		prev := key.addMonths(-1)
		seed, err = s.snapshotRepo.LatestClosedMonth(ctx, staffID, prev.Year, prev.Month)
		if err != nil {
			return nil, fmt.Errorf("failed to load preceding month balance snapshot: %w", err)
		}
	}

	if seed != nil {
		seedKey := monthKey{Year: seed.Year, Month: seed.Month}
		if !seedKey.before(anchorKey) && seedKey.before(key) {
			splice.startKey = seedKey.next()
			splice.frozenCarry = seed.ClosingBalanceMinutes
			splice.carryFrom = &seedKey
		}
	}
	if splice.carryFrom == nil && splice.ownSnapshot == nil {
		return nil, nil
	}
	return splice, nil
}
