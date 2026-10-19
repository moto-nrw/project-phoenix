package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// enrollmentCareBookings records and replaces an approved child's bookings
// in Care Plan for Enrollment's intake, change requests and rollover.
type EnrollmentCareBookingsBinding struct {
	Owner careplan.OfferingBookingCommands
}

func (a EnrollmentCareBookingsBinding) RecordCareBookings(ctx context.Context, childID int64, inputs []enrollmentOwner.CareBookingInput) error {
	return a.Owner.RecordCareOfferingBookings(ctx, childID, careBookingValues(inputs))
}

func (a EnrollmentCareBookingsBinding) ReplaceCareBookings(ctx context.Context, childID int64, inputs []enrollmentOwner.CareBookingInput) error {
	return a.Owner.ReplaceCareOfferingBookings(ctx, childID, careBookingValues(inputs))
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
