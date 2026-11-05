package workforce

import (
	"context"
	"time"
)

// OnSiteStaff answers the Träger's view of a school (#5686): the staff members
// at work on site right now, with an open work session in status present. Home
// office does not count: that person cares for no child at the school. A
// session opened before midnight counts for MaxOpenWorkSessionDuration. The
// map is keyed by school and carries staff IDs only; Workforce does not know
// who has left, so the caller keeps the staff members it counts.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type OnSiteStaff interface {
	OnSiteStaffIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error)
}
