// Package holidayallocation changes holiday places and the frozen holiday
// contributions in one Träger transaction (ADR 0206, #6555): a rebooking, a
// new capacity of a week and the Büro's decision by criteria. It owns no
// data. Enrollment keeps its period, weeks, bookings, decisions, audit and
// mail; Parent Billing keeps its billing lock, sibling groups and
// contributions. The workflow owns only the order of their commands:
// billing lock B, period lock P, the Enrollment command, the new
// contributions, all before the one commit of the outermost unit.
package holidayallocation

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// errReviewerRequired refuses a call without the Büro's Träger and account.
var errReviewerRequired = errors.New("holiday allocation: the Träger and the account of the Büro are required")

// Billing is Parent Billing's part. HoldHolidayAllocationInvoices takes only
// the billing lock of the period's holiday invoices (B), with or without an
// invoice. RefreezeHolidayAllocationContributions computes the confirmed
// sibling groups of the given registrations again, under the locks the
// workflow still holds; it takes no further billing or run lock.
type Billing interface {
	HoldHolidayAllocationInvoices(ctx context.Context, organizationID, accountID, periodID int64) error
	RefreezeHolidayAllocationContributions(ctx context.Context, organizationID, accountID, periodID int64, registrationIDs []int64) error
}

// Periods holds Enrollment's period row (P) until the unit ends.
type Periods interface {
	HoldHolidayPeriod(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64) error
}

// Places are Enrollment's rebooking and capacity commands with their guards.
// Both report the bookings that moved up.
type Places interface {
	RebookHolidayChild(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID, childID int64, bookings []enrollment.HolidayBookingInput) (enrollment.HolidayRegistrationDetail, error)
	SetHolidayWeekCapacity(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, weekID int64, capacity *int) (enrollment.HolidayCapacityChange, error)
}

// Allocation is Enrollment's decision by criteria; its board names every
// registration that gained a place, directly or by moving up.
type Allocation interface {
	DecideHolidayAllocation(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, choices []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error)
}

// WithinTraeger opens the Träger transaction or joins the running one of the
// same Träger; it refuses a school, administrative or other Träger unit.
type WithinTraeger func(ctx context.Context, organizationID int64, fn func(context.Context) error) error

// Ports are the owner commands the workflow orders.
type Ports struct {
	Billing    Billing
	Periods    Periods
	Places     Places
	Allocation Allocation
	Within     WithinTraeger
}

// Workflow runs the three changes of holiday places.
type Workflow struct {
	ports Ports
}

// New builds the workflow; every port is required.
func New(ports Ports) *Workflow {
	if ports.Billing == nil || ports.Periods == nil || ports.Places == nil || ports.Allocation == nil || ports.Within == nil {
		panic("holiday allocation: billing, periods, places, allocation and the Träger transaction are required")
	}
	return &Workflow{ports: ports}
}

// RebookHolidayChild replaces the bookings of one child. Enrollment locks the
// registration and the weeks it touches under P; whoever moves up and the
// rebooked registration get the contribution of the sibling group again.
func (w *Workflow) RebookHolidayChild(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID, childID int64, bookings []enrollment.HolidayBookingInput) (enrollment.HolidayRegistrationDetail, error) {
	var detail enrollment.HolidayRegistrationDetail
	err := w.run(ctx, reviewer, periodID, func(ctx context.Context) ([]int64, error) {
		var err error
		detail, err = w.ports.Places.RebookHolidayChild(ctx, reviewer, periodID, registrationID, childID, bookings)
		// The rebooked registration is computed again as well: its lunch
		// is billed as booked (#5631, ADR 0232).
		return append(promoted(detail.Promotions), registrationID), err
	})
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	return detail, nil
}

// SetHolidayWeekCapacity sets the places of a week, nil for unlimited.
// Enrollment locks the week under P; whoever moves up gets the contribution
// of the sibling group again.
func (w *Workflow) SetHolidayWeekCapacity(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, weekID int64, capacity *int) (enrollment.HolidayCapacityChange, error) {
	var change enrollment.HolidayCapacityChange
	err := w.run(ctx, reviewer, periodID, func(ctx context.Context) ([]int64, error) {
		var err error
		change, err = w.ports.Places.SetHolidayWeekCapacity(ctx, reviewer, periodID, weekID, capacity)
		return promoted(change.Promotions), err
	})
	if err != nil {
		return enrollment.HolidayCapacityChange{}, err
	}
	return change, nil
}

// DecideHolidayAllocation decides on pending bookings. Enrollment locks the
// criteria catalog and the chosen weeks under P and refuses a changed
// proposal; every directly placed and moved-up registration gets the
// contribution of its sibling group again.
func (w *Workflow) DecideHolidayAllocation(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, choices []enrollment.HolidayAllocationChoice) (enrollment.HolidayAllocationBoard, error) {
	var board enrollment.HolidayAllocationBoard
	err := w.run(ctx, reviewer, periodID, func(ctx context.Context) ([]int64, error) {
		var err error
		board, err = w.ports.Allocation.DecideHolidayAllocation(ctx, reviewer, periodID, choices)
		return board.PlacedRegistrations, err
	})
	if err != nil {
		return enrollment.HolidayAllocationBoard{}, err
	}
	return board, nil
}

// run is the protocol of ADR 0206: B, then P, then the Enrollment command,
// then the new contributions, in one Träger unit. Each port gets the unit's
// context and the same keys. The first error stops the protocol and rolls
// the unit back; no step is retried.
func (w *Workflow) run(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64, change func(context.Context) ([]int64, error)) error {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return errReviewerRequired
	}
	return w.ports.Within(ctx, reviewer.OrganizationID, func(ctx context.Context) error {
		if err := w.ports.Billing.HoldHolidayAllocationInvoices(ctx, reviewer.OrganizationID, reviewer.AccountID, periodID); err != nil {
			return err
		}
		if err := w.ports.Periods.HoldHolidayPeriod(ctx, reviewer, periodID); err != nil {
			return err
		}
		placed, err := change(ctx)
		if err != nil {
			return err
		}
		return w.ports.Billing.RefreezeHolidayAllocationContributions(ctx, reviewer.OrganizationID, reviewer.AccountID, periodID, placed)
	})
}

// promoted names each registration whose bookings moved up once, in the
// order Enrollment reports them.
func promoted(promotions []enrollment.HolidayPromotion) []int64 {
	ids := make([]int64, 0, len(promotions))
	for _, promotion := range promotions {
		if !slices.Contains(ids, promotion.RegistrationID) {
			ids = append(ids, promotion.RegistrationID)
		}
	}
	return ids
}
