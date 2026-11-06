package careplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BookedCareOffering is a care offering that counts as care with the children
// of the online enrollment (request child IDs) booked on it on one day.
// Capacity is its limit, nil without one.
type BookedCareOffering struct {
	ID              int64
	Name            string
	Capacity        *int
	RequestChildIDs []int64
}

// TraegerCareBookings answers the Träger's occupancy of a school (#4676): the
// offerings of each school that count as care and hold a booking valid on the
// day, ordered by name. The map is keyed by school; a school without such an
// offering is missing. The caller decides which children it counts.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type TraegerCareBookings interface {
	CareOfferingBookingsByTenant(ctx context.Context, schools []int64, day calendar.Date) (map[int64][]BookedCareOffering, error)
}
