package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentService) CurrentFamilyProtection(ctx context.Context, ids []int64) (result map[int64]bool, err error) {
	err = s.run(ctx, "current_family_protection", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CurrentFamilyProtection(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
