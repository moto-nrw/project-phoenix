package api

import (
	"context"
	"errors"
	"fmt"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/workflows/holidayallocation"
	holidayAllocationCompose "github.com/moto-nrw/project-phoenix/workflows/holidayallocation/compose"
)

// The places of the holiday weeks as the Elternabrechnung's routes change
// them (#4699): a rebooking and a new capacity run through the holiday
// allocation workflow (ADR 0206), which holds the period's invoices and the
// period, changes the places in Enrollment and computes the contribution of
// whoever moved up in the same Träger transaction. This binding only
// translates the values and the refusals between the two owners.
type parentBillingPlaceChanges struct {
	workflow *holidayallocation.Workflow
}

// newHolidayAllocation builds the holiday allocation workflow of one mount
// over Enrollment's period, places and decision and Parent Billing's
// billing lock and contributions.
func newHolidayAllocation(billing parentbilling.HolidayAllocationBilling, review enrollmentOwner.HolidayRegistrationReview, places enrollmentOwner.HolidayPlaces, allocation enrollmentOwner.HolidayAllocation) *holidayallocation.Workflow {
	return holidayAllocationCompose.New(billing, review, holidayAllocationPlaces{HolidayRegistrationReview: review, HolidayPlaces: places}, allocation)
}

// holidayAllocationPlaces offers the workflow Enrollment's rebooking of the
// review and its capacity change of the places as one port.
type holidayAllocationPlaces struct {
	enrollmentOwner.HolidayRegistrationReview
	enrollmentOwner.HolidayPlaces
}

// RebookHolidayChild rebooks the child (#4699).
func (c parentBillingPlaceChanges) RebookHolidayChild(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID, childID int64, bookings []parentbilling.HolidayBookingChoice) (parentbilling.HolidayPlaceChange, error) {
	choices := make([]enrollmentOwner.HolidayBookingInput, len(bookings))
	for i, booking := range bookings {
		choices[i] = enrollmentOwner.HolidayBookingInput{WeekID: booking.WeekID, TimeModelID: booking.TimeModelID, Days: booking.Days}
	}
	detail, err := c.workflow.RebookHolidayChild(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, childID, choices)
	if err != nil {
		return parentbilling.HolidayPlaceChange{}, parentBillingPlacesError(err)
	}
	return parentbilling.HolidayPlaceChange{Registration: detail, RegistrationID: detail.ID, MovedUp: sortedMovedUp(detail.Promotions)}, nil
}

// SetHolidayWeekCapacity sets the places of a week, nil for unlimited
// (#4699).
func (c parentBillingPlaceChanges) SetHolidayWeekCapacity(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, weekID int64, capacity *int) (parentbilling.HolidayPlaceChange, error) {
	change, err := c.workflow.SetHolidayWeekCapacity(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, weekID, capacity)
	if err != nil {
		return parentbilling.HolidayPlaceChange{}, parentBillingPlacesError(err)
	}
	return parentbilling.HolidayPlaceChange{WeekID: weekID, Capacity: capacity, MovedUp: sortedMovedUp(change.Promotions)}, nil
}

// sortedMovedUp names each registration that moved up once, in ascending
// order, and an empty list rather than none.
func sortedMovedUp(promotions []enrollmentOwner.HolidayPromotion) []int64 {
	ids := promotedRegistrations(promotions)
	slices.Sort(ids)
	return ids
}

// parentBillingPlacesError names a refused rebooking or capacity in the
// Elternabrechnung's vocabulary (#4699).
func parentBillingPlacesError(err error) error {
	var fields validation.Errors
	var dayLimit *enrollmentOwner.HolidayChildDayLimitRefusal
	var noCareDay *enrollmentOwner.HolidayWeekNoCareDayRefusal
	switch {
	case errors.As(err, &dayLimit):
		return &parentbilling.HolidayChildDayLimitRefusal{Limit: dayLimit.Limit, Days: dayLimit.Days, Elsewhere: dayLimit.Elsewhere, Err: err}
	case errors.As(err, &noCareDay):
		return &parentbilling.HolidayWeekNoCareDayRefusal{
			Week: noCareDay.Week, WeekID: noCareDay.WeekID,
			Date: noCareDay.Day.Date, Kind: noCareDay.Day.Kind, Name: noCareDay.Day.Name,
		}
	case errors.As(err, &fields):
		return fmt.Errorf("%w: %w", parentbilling.ErrInvalidHolidayPlaces, err)
	case errors.Is(err, enrollmentOwner.ErrHolidayWeekNotFound):
		return fmt.Errorf("%w: %w", parentbilling.ErrHolidayWeekNotFound, err)
	}
	return parentBillingHolidayError(err)
}

func promotedRegistrations(promotions []enrollmentOwner.HolidayPromotion) []int64 {
	ids := []int64{}
	for _, promotion := range promotions {
		if !slices.Contains(ids, promotion.RegistrationID) {
			ids = append(ids, promotion.RegistrationID)
		}
	}
	return ids
}
