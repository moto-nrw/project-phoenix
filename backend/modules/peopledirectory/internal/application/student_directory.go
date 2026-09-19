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

// FindRecord reads one owned row. lock is "UPDATE" when the caller needs the
// row held for the rest of its transaction — a status it validates cannot then
// be changed by a concurrent grade transition before its own write commits.
func (s *StudentService) FindRecord(ctx context.Context, id int64, lock string) (result domain.StudentRecord, err error) {
	run := s.tx.RunRead
	operation := "find_student_record"
	if lock != "" {
		run, operation = s.tx.RunWrite, "lock_student_record"
	}
	err = s.run(ctx, operation, run, func(txCtx context.Context, stats *domain.OperationStats) error {
		// A caller that locks the row is about to write it, so the shared
		// class-writes gate is taken first: holding the row and only then
		// queueing behind a grade transition that waits for exactly that row
		// is the deadlock this order exists to prevent. The gate is
		// re-entrant, so a caller that already took it pays nothing.
		if lock != "" {
			lockStats, err := s.store.LockEnrollmentClassWrites(txCtx)
			stats.Add(lockStats)
			if err != nil {
				return err
			}
		}
		record, found, queryStats, err := s.store.FindRecord(txCtx, id, lock)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrStudentNotFound
		}
		result = record
		return nil
	})
	return result, err
}

// ListRecordsByIDs reads the owned rows of the given children.
func (s *StudentService) ListRecordsByIDs(ctx context.Context, ids []int64) (result []domain.StudentRecord, err error) {
	err = s.run(ctx, "list_student_records_by_id", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListRecordsByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
