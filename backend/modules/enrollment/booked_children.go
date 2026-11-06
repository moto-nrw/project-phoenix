package enrollment

import "context"

// BookedChildren answers the Träger's occupancy of a school (#4676): the child
// each approved child of the online enrollment became, created or matched,
// so a care booking reaches the child it belongs to. The map is keyed by
// school, then by request child, and carries IDs only.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type BookedChildren interface {
	ApprovedChildStudentIDsByTenant(ctx context.Context, schools []int64) (map[int64]map[int64]int64, error)
}
