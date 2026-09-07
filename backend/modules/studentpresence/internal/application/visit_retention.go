package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListVisitRetentionCounts(ctx context.Context) (result []ports.VisitRetentionCount, err error) {
	err = s.run("list_visit_retention_counts", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListVisitRetentionCounts(ctx)
		return stats, err
	})
	return result, err
}
func (s *Service) CountExpiredVisits(ctx context.Context) (result int64, err error) {
	err = s.run("count_expired_visits", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.CountExpiredVisits(ctx)
		return stats, err
	})
	return result, err
}
func (s *Service) OldestExpiredVisitDate(ctx context.Context) (result *time.Time, err error) {
	err = s.run("oldest_expired_visit_date", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.OldestExpiredVisitDate(ctx)
		return stats, err
	})
	return result, err
}
func (s *Service) ListExpiredVisitMonths(ctx context.Context) (result []ports.VisitMonthCount, err error) {
	err = s.run("list_expired_visit_months", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListExpiredVisitMonths(ctx)
		return stats, err
	})
	return result, err
}
