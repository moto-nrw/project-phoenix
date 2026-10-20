package timetracking

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewEmploymentTimeReset uses the ordinary carry-chain math, including frozen
// month snapshots. It books the old balance on the return day, so no closed
// historical month is rewritten. Today's activity is outside the cutoff.
func NewEmploymentTimeReset(deps Dependencies, clearSchedule func(context.Context, int64, string) error) func(context.Context, workforce.EmploymentReset, string) error {
	months := NewWorkTimeMonthService(deps.Sessions, deps.Breaks, deps.Absences, deps.StaffSchedules, deps.ScheduleTargets, deps.TargetModels, deps.Shifts, deps.Settings, deps.Logger,
		WithMonthHolidays(deps.Holidays), WithMonthTargetOverrides(deps.TargetOverrides), WithMonthAdjustments(deps.Adjustments), WithMonthSnapshots(deps.MonthSnapshots))
	return func(ctx context.Context, in workforce.EmploymentReset, note string) error {
		today := calendar.TodayDate()
		if deps.Today != nil {
			today = deps.Today()
		}
		if err := deps.Adjustments.LockStaffBalanceWrites(ctx, in.StaffID); err != nil {
			return err
		}
		if err := deps.Absences.LockStaffAbsenceWrites(ctx, in.StaffID); err != nil {
			return err
		}
		balance, err := resumptionBalance(ctx, deps, months, in.StaffID, today)
		if err != nil {
			return fmt.Errorf("read balance before resumption: %w", err)
		}
		if _, err := deps.Adjustments.CreateStaffBalanceAdjustment(ctx, workforce.StaffBalanceAdjustment{StaffID: in.StaffID, Type: BalanceAdjustmentTypeReset, MinutesDelta: -balance, EffectiveDate: today.String(), Note: note, DecidedBy: in.StaffID, DecidedAt: time.Now()}); err != nil {
			return err
		}
		if err := resetEmploymentVacation(ctx, deps, in, today, note); err != nil {
			return err
		}
		return clearSchedule(ctx, in.StaffID, today.String())
	}
}

func resetEmploymentVacation(ctx context.Context, deps Dependencies, in workforce.EmploymentReset, today calendar.Date, note string) error {
	s := &staffAbsenceService{absenceRepo: deps.Absences, quotaRepo: deps.VacationQuotas, openingRepo: deps.VacationOpenings, deletionRepo: deps.DeletionAudit}
	inputs, err := s.loadVacationQuotaInputs(ctx, in.StaffID, today.Year())
	if err != nil {
		return err
	}
	// An old opening is preserved as an audited tombstone before replacement.
	if inputs.opening != nil {
		if err := s.DeleteVacationOpening(ctx, in.StaffID, in.StaffID, today.Year()); err != nil {
			return err
		}
	}
	// The full existing annual entitlement is retained; only the carryover is
	// cleared. A missing annual row starts with the ordinary annual default.
	if err := deps.VacationQuotas.UpsertStaffVacationQuota(ctx, workforce.StaffVacationQuota{StaffID: in.StaffID, Year: today.Year(), EntitledDays: inputs.entitled, CarryoverDays: 0, ChangeReason: note, ChangedBy: in.StaffID}); err != nil {
		return err
	}
	summary := computeVacationQuotaSummary(in.StaffID, today.Year(), inputs.entitled, 0, inputs.absences, nil)
	// Credit the days from the previous employment instead of deleting its
	// absences. Later corrections stay visible, just like balance resets.
	opening := &StaffVacationOpening{StaffID: in.StaffID, Year: today.Year(), EffectiveDate: today, TakenBeforeDays: -summary.TakenDays - summary.ReservedDays, EnteredRemainingDays: inputs.entitled, Note: note, DecidedBy: in.StaffID, DecidedAt: time.Now()}
	if err := opening.Validate(); err != nil {
		return err
	}
	_, err = deps.VacationOpenings.CreateStaffVacationOpening(ctx, openingToCapability(opening))
	return err
}

// A frozen month's stored balance feeds the next month. Reading its live
// closing balance instead would reset the wrong amount after a month close.
func resumptionBalance(ctx context.Context, deps Dependencies, months WorkTimeMonthService, staffID int64, today calendar.Date) (int, error) {
	cutoff := today.AddDays(-1)
	anchor, err := resolveAccountAnchor(ctx, deps.Settings, deps.Logger, monthOf(today))
	if err != nil {
		return 0, err
	}
	if cutoff.Before(anchor) {
		return 0, nil
	}
	if today.Day() == 1 {
		snapshot, err := deps.MonthSnapshots.LatestClosedMonth(ctx, staffID, cutoff.Year(), int(cutoff.Month()))
		if err != nil {
			return 0, err
		}
		if snapshot != nil && snapshot.Year == cutoff.Year() && snapshot.Month == int(cutoff.Month()) {
			return snapshot.ClosingBalanceMinutes, nil
		}
	}
	return months.GetClosingBalanceAsOf(ctx, staffID, cutoff)
}
