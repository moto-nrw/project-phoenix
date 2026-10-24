package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListVisitRetentionCounts(ctx context.Context, policy studentpresence.VisitRetentionPolicy) (result []studentpresence.VisitRetentionCount, err error) {
	err = s.runWrite(ctx, "list_visit_retention_counts", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListVisitRetentionCounts(ctx, policy)
		return stats, err
	})
	return listResult(result, err)
}
func (s *Service) CountExpiredVisits(ctx context.Context, policy studentpresence.VisitRetentionPolicy) (result int64, err error) {
	err = s.runWrite(ctx, "count_expired_visits", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.CountExpiredVisits(ctx, policy)
		return stats, err
	})
	return result, err
}
func (s *Service) OldestExpiredVisitDate(ctx context.Context, policy studentpresence.VisitRetentionPolicy) (result *time.Time, err error) {
	err = s.runWrite(ctx, "oldest_expired_visit_date", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.OldestExpiredVisitDate(ctx, policy)
		return stats, err
	})
	return result, err
}
func (s *Service) ListExpiredVisitMonths(ctx context.Context, policy studentpresence.VisitRetentionPolicy) (result []studentpresence.VisitMonthCount, err error) {
	err = s.runWrite(ctx, "list_expired_visit_months", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListExpiredVisitMonths(ctx, policy)
		return stats, err
	})
	return listResult(result, err)
}
