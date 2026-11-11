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
	// DeleteExpiredVisits deletes, in one statement, the completed visits of
	// every listed student past that student's retention: before
	// CompletedBefore when it is set, otherwise created more than
	// RetentionDays before now. It returns the deleted count per student;
	// students without deletions are absent.
	DeleteExpiredVisits(context.Context, []VisitRetentionCount, time.Time) (map[int64]int64, error)
}
