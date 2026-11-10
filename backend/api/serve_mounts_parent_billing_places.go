package api

import (
	"context"
	"errors"
	"fmt"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The places of the holiday weeks as the Elternabrechnung changes them
// (#4699): a rebooking and a new capacity run through Enrollment, which moves
// the waiting lists up; the Elternabrechnung freezes the contribution of
// whoever moved up in the same transaction.

// RebookHolidayChild rebooks the child through the review, which gives the
// places it frees to the waiting lists (#4699).
func (b parentBillingHolidayRegistrations) RebookHolidayChild(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID, childID int64, bookings []parentbilling.HolidayBookingChoice) (parentbilling.HolidayRegistration, error) {
	choices := make([]enrollmentOwner.HolidayBookingInput, len(bookings))
	for i, booking := range bookings {
		choices[i] = enrollmentOwner.HolidayBookingInput{WeekID: booking.WeekID, TimeModelID: booking.TimeModelID, Days: booking.Days}
	}
	detail, err := b.review.RebookHolidayChild(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, registrationID, childID, choices)
	if err != nil {
		return parentbilling.HolidayRegistration{}, parentBillingPlacesError(err)
	}
	registration := parentBillingRegistration(detail)
	registration.View = detail
	return registration, nil
}

// SetHolidayWeekCapacity sets the places of a week through Enrollment
// (#4699) and names the registrations that moved up.
func (b parentBillingHolidayRegistrations) SetHolidayWeekCapacity(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, weekID int64, capacity *int) ([]int64, error) {
	change, err := b.places.SetHolidayWeekCapacity(ctx, enrollmentOwner.HolidayReviewer(reviewer), periodID, weekID, capacity)
	if err != nil {
		return nil, parentBillingPlacesError(err)
	}
	return promotedRegistrations(change.Promotions), nil
}

// parentBillingPlacesError names a refused rebooking or capacity in the
// Elternabrechnung's vocabulary (#4699).
func parentBillingPlacesError(err error) error {
	var fields validation.Errors
	var dayLimit *enrollmentOwner.HolidayChildDayLimitRefusal
	var noCareDay *enrollmentOwner.HolidayWeekNoCareDayRefusal
	switch {
	case errors.As(err, &dayLimit):
		return &parentbilling.HolidayChildDayLimitRefusal{Limit: dayLimit.Limit, Days: dayLimit.Days, Err: err}
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
