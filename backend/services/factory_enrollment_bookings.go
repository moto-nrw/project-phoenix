package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type enrollmentCareBookingCommands struct {
	owner careplan.OfferingBookingCommands
}

func (a enrollmentCareBookingCommands) RecordCareBookings(ctx context.Context, childID int64, inputs []enrollmentOwner.CareBookingInput) error {
	return a.owner.RecordCareOfferingBookings(ctx, childID, careBookingValues(inputs))
}

func (a enrollmentCareBookingCommands) ReplaceCareBookings(ctx context.Context, childID int64, inputs []enrollmentOwner.CareBookingInput) error {
	return a.owner.ReplaceCareOfferingBookings(ctx, childID, careBookingValues(inputs))
}

func careBookingValues(inputs []enrollmentOwner.CareBookingInput) []careplan.CareOfferingBooking {
	bookings := make([]careplan.CareOfferingBooking, 0, len(inputs))
	for _, input := range inputs {
		booking := careplan.CareOfferingBooking{
			CareOfferingID: input.CareOfferingID, ManualSelectedDays: input.ManualSelectedDays, AutomaticSelectedDays: input.AutomaticSelectedDays,
		}
		if input.ValidFrom != nil {
			booking.ValidFrom = new(careplan.Date(*input.ValidFrom))
		}
		if input.ValidUntil != nil {
			booking.ValidUntil = new(careplan.Date(*input.ValidUntil))
		}
		bookings = append(bookings, booking)
	}
	return bookings
}
