package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentStore) ListCareEnds(ctx context.Context, ids []int64) (map[int64]string, domain.OperationStats, error) {
	bounds, stats, err := s.reads.ListCareEnds(ctx, ids)
	return bounds, domain.OperationStats(stats), projectionError(err)
}
