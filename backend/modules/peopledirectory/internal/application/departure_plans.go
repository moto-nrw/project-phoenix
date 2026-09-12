package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentService) ListDepartureModes(ctx context.Context, ids []int64) (result map[int64]map[string][]string, err error) {
	err = s.run(ctx, "list_student_departure_modes", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListDepartureModes(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
