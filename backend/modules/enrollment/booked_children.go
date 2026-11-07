package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BookedChildren answers the Träger's occupancy of a school (#4676): the child
// each approved child of the online enrollment became, created or matched,
// so a care booking reaches the child it belongs to. The map is keyed by
// school, then by request child, and carries IDs only.
//
// RunningPhaseIDs names the active enrollment phases whose service period
// (service_start_date to service_end_date, both inclusive) holds the day, so
// the occupancy also lists the offerings of a running phase that nobody has
// booked yet (#5977). Phase IDs are unique across schools.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type BookedChildren interface {
	ApprovedChildStudentIDsByTenant(ctx context.Context, schools []int64) (map[int64]map[int64]int64, error)
	RunningPhaseIDs(ctx context.Context, schools []int64, day calendar.Date) ([]int64, error)
}
