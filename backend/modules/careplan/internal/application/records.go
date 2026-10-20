package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func runValue[T any](ctx context.Context, s *Service, operation string, call func(context.Context) (T, domain.OperationStats, error)) (result T, err error) {
	err = s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = call(ctx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func runCommand(ctx context.Context, s *Service, operation string, call func(context.Context) (domain.OperationStats, error)) error {
	return s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := call(ctx)
		stats.Add(queryStats)
		return err
	})
}
