package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// ListDirectory serves one page of the staff directory.
func (s *StudentService) ListDirectory(
	ctx context.Context,
	filter domain.StudentDirectoryFilter,
) (result []domain.StudentRecord, err error) {
	err = s.run(ctx, "list_student_directory", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListDirectory(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// CountDirectory counts the same selection without its page window, so the
// reported total names every child of the selection rather than of the page.
func (s *StudentService) CountDirectory(
	ctx context.Context,
	filter domain.StudentDirectoryFilter,
) (total int, err error) {
	err = s.run(ctx, "count_student_directory", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		total, queryStats, err = s.store.CountDirectory(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return total, err
}

// ListDirectoryIDs returns the lightweight candidate set of the dated
// participation rule.
func (s *StudentService) ListDirectoryIDs(ctx context.Context) (result []int64, err error) {
	err = s.run(ctx, "list_student_directory_ids", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListDirectoryIDs(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
