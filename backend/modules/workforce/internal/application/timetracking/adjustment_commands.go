package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *adjustmentCommands) ListAdjustments(ctx context.Context, staffID int64, from, to calendar.Date) (value0 []*StaffBalanceAdjustment, err error) {
	err = s.staffBalanceAdjustmentService.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.staffBalanceAdjustmentService.ListAdjustments(txCtx, staffID, from, to)
		return err
	})
	return
}

func (s *adjustmentCommands) CreateAdjustment(ctx context.Context, staffID, decidedBy int64, req CreateBalanceAdjustmentRequest) (value0 *StaffBalanceAdjustment, err error) {
	err = s.staffBalanceAdjustmentService.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.staffBalanceAdjustmentService.CreateAdjustment(txCtx, staffID, decidedBy, req)
		return err
	})
	return
}

func (s *adjustmentCommands) DeleteAdjustment(ctx context.Context, staffID, adjustmentID, deletedBy int64) (err error) {
	err = s.staffBalanceAdjustmentService.runCommand(ctx, func(txCtx context.Context) error {
		err = s.staffBalanceAdjustmentService.DeleteAdjustment(txCtx, staffID, adjustmentID, deletedBy)
		return err
	})
	return
}

func (s *adjustmentCommands) ResetBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate calendar.Date, carryoverMinutes int, note string) (value0 *StaffBalanceAdjustment, err error) {
	err = s.staffBalanceAdjustmentService.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.staffBalanceAdjustmentService.ResetBalance(txCtx, staffID, decidedBy, effectiveDate, carryoverMinutes, note)
		return err
	})
	return
}

// CreateOpeningBalance writes the go-live opening booking (#2132): a signed
// rebaseline that turns the closing balance as of effectiveDate into
// balanceMinutes. Unlike ResetBalance there is no lower bound and no
// reduction-capacity check — a migrated account legitimately starts negative
// when the staff member owed hours in the previous system.
func (s *adjustmentCommands) CreateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate calendar.Date, balanceMinutes int, note string) (value0 *StaffBalanceAdjustment, err error) {
	err = s.staffBalanceAdjustmentService.runCommand(ctx, func(txCtx context.Context) error {
		value0, err = s.staffBalanceAdjustmentService.CreateOpeningBalance(txCtx, staffID, decidedBy, effectiveDate, balanceMinutes, note)
		return err
	})
	return
}

// ValidateOpeningBalance runs the read-only booking guards used by
// CreateOpeningBalance. It deliberately does not take the transaction-scoped
// staff lock: bulk imports validate every uploaded row before their sorted
// creation pass, and file-order locks here could deadlock concurrent imports.
func (s *adjustmentCommands) ValidateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate calendar.Date, balanceMinutes int, note string) (err error) {
	err = s.staffBalanceAdjustmentService.runCommand(ctx, func(txCtx context.Context) error {
		err = s.staffBalanceAdjustmentService.ValidateOpeningBalance(txCtx, staffID, decidedBy, effectiveDate, balanceMinutes, note)
		return err
	})
	return
}
