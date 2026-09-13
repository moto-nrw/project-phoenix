package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
)

type enrollmentCareBookingCommands struct {
	owner careplan.OfferingBookingCommands
}

func (a enrollmentCareBookingCommands) RecordCareBookings(ctx context.Context, childID int64, inputs []enrollment.CareBookingInput) error {
	return a.owner.RecordCareOfferingBookings(ctx, childID, careBookingValues(inputs))
}

func (a enrollmentCareBookingCommands) ReplaceCareBookings(ctx context.Context, childID int64, inputs []enrollment.CareBookingInput) error {
	return a.owner.ReplaceCareOfferingBookings(ctx, childID, careBookingValues(inputs))
}

func (a enrollmentCareBookingCommands) ScheduleCareBookings(ctx context.Context, childID int64, from timezone.Date, inputs []enrollment.CareBookingInput) error {
	return a.owner.ScheduleCareOfferingBookings(ctx, childID, careplan.Date(from), careBookingValues(inputs))
}

func careBookingValues(inputs []enrollment.CareBookingInput) []careplan.CareOfferingBooking {
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
