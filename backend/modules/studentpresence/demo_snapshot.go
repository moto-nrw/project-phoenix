package studentpresence

import (
	"context"
	"time"
)

// DemoVisit combines current room occupancy with the latest school booking.
type DemoVisit struct {
	StudentID   int64
	Active, Web bool
	ChangedAt   time.Time
}

// DemoVisits reads the demo snapshot of current occupancy and latest
// bookings. It requires a tenant-scoped transaction context and a bounded
// attendance date window.
type DemoVisits interface {
	LatestDemoVisits(ctx context.Context, webDeviceID int64, fromDate, untilDate string) ([]DemoVisit, error)
}
