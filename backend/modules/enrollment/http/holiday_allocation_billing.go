package enrollmenthttp

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// HolidayAllocationBilling is the contribution owner's guarded allocation
// command: it holds invoices before deciding and recomputes sibling groups
// for every registration that gains a place, directly or by promotion.
// It joins the request's Träger transaction; any failure rolls it back.
type HolidayAllocationBilling interface {
	AllocateHolidayPlaces(ctx context.Context, organizationID, accountID, periodID int64, decide func(context.Context) ([]int64, error)) error
}

// NewBilledHolidayAllocation binds the office decision to its contribution
// command. Enrollment owns decisions, promotions, audit and mail; the
// contribution owner keeps its invoice lock and contribution updates.
func NewBilledHolidayAllocation(allocation enrollment.HolidayAllocation, billing HolidayAllocationBilling) enrollment.HolidayAllocation {
	if allocation == nil || billing == nil {
		panic("enrollment: holiday allocation and billing are required")
	}
	return holidayBillingAllocation{HolidayAllocation: allocation, billing: billing}
}

type holidayBillingAllocation struct {
	enrollment.HolidayAllocation
	billing HolidayAllocationBilling
}

func (a holidayBillingAllocation) DecideHolidayAllocation(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, choices []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error) {
	var board enrollment.HolidayAllocationBoard
	err := a.billing.AllocateHolidayPlaces(ctx, reviewer.OrganizationID, reviewer.AccountID, periodID, func(ctx context.Context) ([]int64, error) {
		var err error
		board, err = a.HolidayAllocation.DecideHolidayAllocation(ctx, reviewer, periodID, choices)
		return board.PlacedRegistrations, err
	})
	return board, err
}
