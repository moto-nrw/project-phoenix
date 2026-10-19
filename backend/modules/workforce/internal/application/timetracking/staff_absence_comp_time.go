package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CompTimeBalancePreview is what the create modal shows before the Leitung
// confirms a Freizeitausgleich (#2873). All values are minutes.
type CompTimeBalancePreview struct {
	// CurrentBalanceMinutes is the live Stundenkonto as of today.
	CurrentBalanceMinutes int `json:"current_balance_minutes"`
	// DeductionMinutes is what the requested range additionally deducts
	// (overlapping existing comp-time rows already reserve their share).
	DeductionMinutes int `json:"deduction_minutes"`
	// RealizedDeductionMinutes is the part of DeductionMinutes on days up to
	// today — already contained in CurrentBalanceMinutes, since a day without
	// recorded work shows its missing target as minus either way.
	RealizedDeductionMinutes int `json:"realized_deduction_minutes"`
	// FutureCommitmentMinutes sums the deductions of OTHER comp-time entries
	// on days after today, which the current balance does not yet include.
	FutureCommitmentMinutes int `json:"future_commitment_minutes"`
	// FutureAdjustmentMinutes is the signed sum of Stundenkonto transactions
	// (payout / grant / reset, #1420) effective after today — booked, but like
	// the commitments not yet part of CurrentBalanceMinutes.
	FutureAdjustmentMinutes int `json:"future_adjustment_minutes"`
	// ProjectedBalanceMinutes = current + future adjustments − future
	// commitments − the unrealized part of the new deduction: the account once
	// every named day has passed.
	ProjectedBalanceMinutes int `json:"projected_balance_minutes"`
}

// PreviewCompTimeBalance projects what a comp_time absence over [start, end]
// does to the Stundenkonto (#2873). It replaces the former overdraft
// rejection: the numbers inform the Leitung before the deliberate confirm in
// the frontend, they never block the booking.
func (s *staffAbsenceService) PreviewCompTimeBalance(
	ctx context.Context,
	staffID int64,
	start, end calendar.Date,
	halfDay bool,
) (*CompTimeBalancePreview, error) {
	if s.monthService == nil {
		return nil, fmt.Errorf("comp_time preview requires the month service")
	}
	if end.Before(start) {
		return nil, &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid date range: date_end must not be before date_start")}
	}
	if err := validateSingleDayHalfDayAbsence(AbsenceTypeCompTime, halfDay, start, end); err != nil {
		return nil, err
	}
	if err := s.rejectPreAccountCompTime(ctx, AbsenceTypeCompTime, start, end); err != nil {
		return nil, err
	}

	today := s.today()
	currentBalance := 0
	anchor, err := resolveAccountAnchor(ctx, s.settings, s.getLogger(), monthOf(today))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve account start for comp_time preview: %w", err)
	}
	if !today.Before(anchor) {
		currentBalance, err = s.monthService.GetClosingBalanceAsOf(ctx, staffID, today)
		if err != nil {
			return nil, fmt.Errorf("failed to compute current balance for comp_time preview: %w", err)
		}
	}

	// Existing comp-time rows overlapping the requested range already reserve
	// their minutes; a create over them merges, so only the additional part
	// counts — the same reservation arithmetic the former guard used.
	overlapping, err := s.effectiveCompTimeAbsences(ctx, staffID, start, end)
	if err != nil {
		return nil, err
	}
	deduction, err := s.getAdditionalCompTimeDeduction(ctx, staffID, start, end, halfDay, overlapping)
	if err != nil {
		return nil, err
	}

	unrealizedDeduction := 0
	if end.After(today) {
		clippedStart := start
		if tomorrow := today.AddDays(1); clippedStart.Before(tomorrow) {
			clippedStart = tomorrow
		}
		unrealizedDeduction, err = s.getAdditionalCompTimeDeduction(ctx, staffID, clippedStart, end, halfDay, overlapping)
		if err != nil {
			return nil, err
		}
	}

	futureCommitment, err := s.monthService.GetFutureCompTimeCommitmentMinutes(ctx, staffID)
	if err != nil {
		return nil, fmt.Errorf("failed to compute future comp_time commitments: %w", err)
	}

	// Future-dated Stundenkonto transactions (payout / grant / reset, #1420)
	// move the account exactly like committed comp-time days, so the
	// projection folds them in over the same carry-chain horizon.
	horizon := monthOf(today).addMonths(maxFutureMonths).lastDay()
	futureAdjustments, err := s.monthService.GetBalanceAdjustmentMinutes(ctx, staffID, today.AddDays(1), horizon)
	if err != nil {
		return nil, fmt.Errorf("failed to compute future balance adjustments: %w", err)
	}

	return &CompTimeBalancePreview{
		CurrentBalanceMinutes:    currentBalance,
		DeductionMinutes:         deduction,
		RealizedDeductionMinutes: deduction - unrealizedDeduction,
		FutureCommitmentMinutes:  futureCommitment,
		FutureAdjustmentMinutes:  futureAdjustments,
		ProjectedBalanceMinutes:  currentBalance + futureAdjustments - futureCommitment - unrealizedDeduction,
	}, nil
}

// effectiveCompTimeAbsences returns the comp_time rows overlapping [start,
// end] that actually reserve balance (reported/approved) — the set a create
// over the same range would merge with.
func (s *staffAbsenceService) effectiveCompTimeAbsences(ctx context.Context, staffID int64, start, end calendar.Date) ([]*StaffAbsence, error) {
	rows, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing comp_time absences: %w", err)
	}
	compTime := make([]*StaffAbsence, 0, len(rows))
	for _, row := range rows {
		if row.AbsenceType != AbsenceTypeCompTime {
			continue
		}
		if row.Status != AbsenceStatusReported && row.Status != AbsenceStatusApproved {
			continue
		}
		compTime = append(compTime, row)
	}
	return compTime, nil
}

func (s *staffAbsenceService) getAdditionalCompTimeDeduction(
	ctx context.Context,
	staffID int64,
	start, end calendar.Date,
	halfDay bool,
	existing []*StaffAbsence,
) (int, error) {
	deduction, err := s.monthService.GetCompTimeDeductionMinutes(ctx, staffID, start, end, halfDay)
	if err != nil {
		return 0, fmt.Errorf("failed to compute comp_time deduction: %w", err)
	}

	reservedDeduction := 0
	for _, absence := range existing {
		overlapStart := absence.DateStart
		if overlapStart.Before(start) {
			overlapStart = start
		}
		overlapEnd := absence.DateEnd
		if overlapEnd.After(end) {
			overlapEnd = end
		}
		if overlapEnd.Before(overlapStart) {
			continue
		}
		reserved, err := s.monthService.GetCompTimeDeductionMinutes(
			ctx,
			staffID,
			overlapStart,
			overlapEnd,
			absence.HalfDay,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to compute existing comp_time deduction: %w", err)
		}
		reservedDeduction += reserved
	}
	return deduction - reservedDeduction, nil
}
