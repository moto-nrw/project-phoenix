package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentService) ListStudentCareEnds(ctx context.Context, ids []int64) (bounds map[int64]string, err error) {
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	err = s.run(ctx, "list_student_care_ends", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		bounds, queryStats, err = s.store.ListCareEnds(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return bounds, err
}
