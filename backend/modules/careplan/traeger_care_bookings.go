package careplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BookedCareOffering is a care offering that counts as care with the children
// of the online enrollment (request child IDs) booked on it on one day; empty
// for an offering of a running phase without such a booking. Capacity is its
// limit, nil without one. Active is false for an offering the school no
// longer offers; it shows only while it still holds bookings.
type BookedCareOffering struct {
	ID              int64
	Name            string
	Capacity        *int
	Active          bool
	RequestChildIDs []int64
}

// TraegerCareBookings answers the Träger's occupancy of a school (#4676): the
// offerings of each school that count as care and hold a booking valid on the
// day, together with the active ones of the running phases (#5977), ordered
// by name. The running phases come from Enrollment, which owns the service
// period. The map is keyed by school; a school without such an offering is
// missing. The caller decides which children it counts.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type TraegerCareBookings interface {
	CareOfferingBookingsByTenant(ctx context.Context, schools []int64, day calendar.Date, runningPhases []int64) (map[int64][]BookedCareOffering, error)
}
