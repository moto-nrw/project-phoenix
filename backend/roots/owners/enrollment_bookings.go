package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
)

// NewEnrollmentBookings is Enrollment's booking projection over Care Plan's
// bookings in the ambient tenant transaction (#2743, #4012). The enrollment
// and settings routes, the service graph and the booking audit read it.
func NewEnrollmentBookings() *enrollment.BookingProjection {
	return ComposeEnrollmentBookings(enrollmentCompose.New())
}

func ComposeEnrollmentBookings(owner enrollment.Owner) *enrollment.BookingProjection {
	return enrollment.NewBookingProjection(owner, enrollmentBookingRows{bookings: carePlanCompose.NewOfferingBookings()})
}

// enrollmentBookingRows serves Enrollment's booking port from Care Plan's
// booking store, with the effective days Care Plan derives.
type enrollmentBookingRows struct{ bookings *careplan.OfferingBookings }

func (r enrollmentBookingRows) CareOfferingBookingHistory(ctx context.Context, childIDs []int64) ([]enrollment.OfferingBooking, error) {
	rows, err := r.bookings.CareOfferingBookingHistory(ctx, childIDs)
	return enrollmentBookings(rows), err
}

func (r enrollmentBookingRows) CareOfferingBookingsAtDates(ctx context.Context, dates map[int64]enrollment.Date) ([]enrollment.OfferingBooking, error) {
	ownerDates := make(map[int64]careplan.Date, len(dates))
	for id, date := range dates {
		ownerDates[id] = careplan.Date(date)
	}
	rows, err := r.bookings.CareOfferingBookingsAtDates(ctx, ownerDates)
	return enrollmentBookings(rows), err
}

func (r enrollmentBookingRows) CareOfferingBookingsForOfferings(ctx context.Context, offeringIDs []int64) ([]enrollment.OfferingBooking, error) {
	rows, err := r.bookings.CareOfferingBookingsForOfferings(ctx, offeringIDs)
	return enrollmentBookings(rows), err
}

func (r enrollmentBookingRows) CareOfferingBookingsForPhase(ctx context.Context, phaseID int64) ([]enrollment.OfferingBooking, error) {
	rows, err := r.bookings.CareOfferingBookingsForPhase(ctx, phaseID)
	return enrollmentBookings(rows), err
}

func enrollmentBookings(rows []careplan.CareOfferingBooking) []enrollment.OfferingBooking {
	if rows == nil {
		return nil
	}
	bookings := make([]enrollment.OfferingBooking, 0, len(rows))
	for _, row := range rows {
		bookings = append(bookings, enrollment.OfferingBooking{
			ID: row.ID, TenantID: row.TenantID, RequestChildID: row.RequestChildID, CareOfferingID: row.CareOfferingID,
			SelectedDays: row.EffectiveSelectedDays(), ManualSelectedDays: row.ManualSelectedDays, AutomaticSelectedDays: row.AutomaticSelectedDays,
			ValidFrom: enrollmentBookingDay(row.ValidFrom), ValidUntil: enrollmentBookingDay(row.ValidUntil),
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return bookings
}

func enrollmentBookingDay(date *careplan.Date) *enrollment.Date {
	if date == nil {
		return nil
	}
	return new(enrollment.Date(*date))
}
