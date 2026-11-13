package ports

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
)

// VisitRetentionStore reads completed visits past their retention window. A
// child's window is the shortest of its accepted consents; every other child
// uses defaultDays, the tenant's gdpr.privacy_consent_retention_days (#3835).
type VisitRetentionStore interface {
	ListVisitRetentionCounts(ctx context.Context, policy studentpresence.VisitRetentionPolicy) ([]studentpresence.VisitRetentionCount, Stats, error)
	CountExpiredVisits(ctx context.Context, policy studentpresence.VisitRetentionPolicy) (int64, Stats, error)
	OldestExpiredVisitDate(ctx context.Context, policy studentpresence.VisitRetentionPolicy) (*time.Time, Stats, error)
	ListExpiredVisitMonths(ctx context.Context, policy studentpresence.VisitRetentionPolicy) ([]studentpresence.VisitMonthCount, Stats, error)
}

// VisitExpiry selects the completed visits of one student to delete: those
// that ended before Cutoff when ByExit is set, otherwise those created before
// Cutoff.
type VisitExpiry struct {
	StudentID int64
	Cutoff    time.Time
	ByExit    bool
}
