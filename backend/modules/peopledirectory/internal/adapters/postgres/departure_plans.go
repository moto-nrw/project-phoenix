package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentStore) ListDepartureModes(ctx context.Context, ids []int64) (map[int64]map[string][]string, domain.OperationStats, error) {
	rows, stats, err := s.reads.ListDepartureModes(ctx, ids)
	return rows, domain.OperationStats(stats), projectionError(err)
}
