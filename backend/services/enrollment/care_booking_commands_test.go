package enrollment_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

type testBookingCommands struct {
	owner *careplan.OfferingBookings
}

func requestTestBookingCommands() testBookingCommands {
	return testBookingCommands{owner: carePlanTest.NewOfferingBookings()}
}

func (a testBookingCommands) RecordCareBookings(ctx context.Context, childID int64, inputs []enrollmentService.CareBookingInput) error {
	return a.owner.RecordCareOfferingBookings(ctx, childID, testCareBookingValues(inputs))
}

func (a testBookingCommands) ReplaceCareBookings(ctx context.Context, childID int64, inputs []enrollmentService.CareBookingInput) error {
	return a.owner.ReplaceCareOfferingBookings(ctx, childID, testCareBookingValues(inputs))
}

func (a testBookingCommands) ScheduleCareBookings(ctx context.Context, childID int64, from timezone.Date, inputs []enrollmentService.CareBookingInput) error {
	return a.owner.ScheduleCareOfferingBookings(ctx, childID, careplan.Date(from), testCareBookingValues(inputs))
}

func testCareBookingValues(inputs []enrollmentService.CareBookingInput) []careplan.CareOfferingBooking {
	bookings := make([]careplan.CareOfferingBooking, 0, len(inputs))
	for _, input := range inputs {
		booking := careplan.CareOfferingBooking{CareOfferingID: input.CareOfferingID, ManualSelectedDays: input.ManualSelectedDays, AutomaticSelectedDays: input.AutomaticSelectedDays}
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
