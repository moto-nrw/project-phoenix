package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// The permanent-deletion commands run in the caller's transaction only: the
// workflow that coordinates a child deletion holds the row locks and decides
// the order, so none of these opens a transaction of its own.

func (s *StudentService) CountGuardianLinks(ctx context.Context, studentID, personID int64) (count int, err error) {
	err = s.run(ctx, "count_student_guardian_links", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		count, queryStats, err = s.store.CountGuardianLinks(txCtx, studentID, personID)
		stats.Add(queryStats)
		return err
	})
	return count, err
}

func (s *StudentService) DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (rows int64, err error) {
	err = s.run(ctx, "delete_legacy_guardian_links", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		rows, writeStats, err = s.store.DeleteLegacyGuardianLinks(txCtx, personID)
		stats.Add(writeStats)
		return err
	})
	return rows, err
}

func (s *StudentService) Delete(ctx context.Context, id int64) (rows int64, err error) {
	err = s.run(ctx, "delete_student", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		rows, writeStats, err = s.store.Delete(txCtx, id)
		stats.Add(writeStats)
		return err
	})
	return rows, err
}

func (s *Service) AnonymizeIfUnchanged(ctx context.Context, personID int64, updatedAt time.Time) (anonymized bool, err error) {
	err = s.runWrite(ctx, "anonymize_deleted_student_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		anonymized, writeStats, err = s.store.AnonymizeIfUnchanged(txCtx, personID, updatedAt)
		stats.Add(writeStats)
		return err
	})
	return anonymized, err
}
