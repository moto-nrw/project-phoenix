// Package holidayallocation coordinates holiday places and contributions
// in one Träger transaction. Enrollment and Parent Billing retain their data.
package holidayallocation

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Billing is the contribution owner's participation in the decision.
type Billing interface {
	HoldHolidayAllocationInvoices(context.Context, int64, int64, int64) error
	RefreezeHolidayAllocationContributions(context.Context, int64, int64, int64, []int64) error
}

// Periods holds Enrollment's period before the decision is checked.
type Periods interface {
	HoldHolidayPeriod(context.Context, enrollment.HolidayReviewer, int64) error
}

// WithinTraeger opens or joins the organization's transaction. An error
// rolls back every decision, contribution and outbox entry in the unit.
type WithinTraeger func(context.Context, int64, func(context.Context) error) error

// New builds the allocation workflow over the owner commands and runtime.
func New(allocation enrollment.HolidayAllocation, periods Periods, billing Billing, within WithinTraeger) *Workflow {
	if allocation == nil || periods == nil || billing == nil || within == nil {
		panic("holiday allocation: allocation, periods, billing and transaction are required")
	}
	return &Workflow{HolidayAllocation: allocation, periods: periods, billing: billing, within: within}
}

// Workflow keeps the office decision and contribution updates atomic.
type Workflow struct {
	enrollment.HolidayAllocation
	periods Periods
	billing Billing
	within  WithinTraeger
}

// DecideHolidayAllocation holds invoices, then the period, before Enrollment
// checks and applies the choices under its criteria and sorted week locks.
// Parent Billing then refreezes every affected confirmed sibling group.
// All steps use the same Träger unit, including audit and parents' mail.
func (w *Workflow) DecideHolidayAllocation(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, choices []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error) {
	var board enrollment.HolidayAllocationBoard
	err := w.within(ctx, reviewer.OrganizationID, func(ctx context.Context) error {
		if err := w.billing.HoldHolidayAllocationInvoices(ctx, reviewer.OrganizationID, reviewer.AccountID, periodID); err != nil {
			return err
		}
		if err := w.periods.HoldHolidayPeriod(ctx, reviewer, periodID); err != nil {
			return err
		}
		var err error
		board, err = w.HolidayAllocation.DecideHolidayAllocation(ctx, reviewer, periodID, choices)
		if err != nil {
			return err
		}
		return w.billing.RefreezeHolidayAllocationContributions(ctx, reviewer.OrganizationID, reviewer.AccountID, periodID, board.PlacedRegistrations)
	})
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	return board, nil
}
