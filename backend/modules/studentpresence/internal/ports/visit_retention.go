package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// VisitRetentionStore reads completed visits past their retention window. A
// child's window is the shortest of its accepted consents; every other child
// uses defaultDays, the tenant's gdpr.privacy_consent_retention_days (#3835).
type VisitRetentionStore interface {
	ListVisitRetentionCounts(ctx context.Context, defaultDays int) ([]studentpresence.VisitRetentionCount, Stats, error)
	CountExpiredVisits(ctx context.Context, defaultDays int) (int64, Stats, error)
	OldestExpiredVisitDate(ctx context.Context, defaultDays int) (*time.Time, Stats, error)
	ListExpiredVisitMonths(ctx context.Context, defaultDays int) ([]studentpresence.VisitMonthCount, Stats, error)
}
