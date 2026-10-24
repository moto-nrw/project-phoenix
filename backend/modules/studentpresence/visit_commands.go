package studentpresence

import (
	"context"
	"time"
)

type VisitMaintenance interface {
	// CloseGroupVisits uses the database clock, clamped to each entry time.
	CloseGroupVisits(context.Context, []int64) (int64, error)
	TransferOpenVisits(context.Context, int64, int64) (int64, error)
	TransferRecentDeviceVisits(context.Context, int64, int64) (int64, error)
	DeleteCompletedVisitsBefore(context.Context, int64, VisitRetentionPolicy, time.Time) (int64, error)
}
