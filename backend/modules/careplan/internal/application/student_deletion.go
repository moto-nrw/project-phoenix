package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// The permanent-deletion commands join the caller's transaction; the
// coordinating workflow holds the student locks and decides the order.

func (s *Service) FindPendingWithdrawalStudent(ctx context.Context, completionID int64, lock bool) (int64, error) {
	return runValue(s, "find_pending_withdrawal_student", func() (int64, domain.OperationStats, error) {
		return s.store.FindPendingWithdrawalStudent(ctx, completionID, lock)
	})
}

func (s *Service) ResolvePendingWithdrawalAsDeleted(ctx context.Context, completionID, actorAccountID int64, at time.Time) (bool, error) {
	return runValue(s, "resolve_pending_withdrawal_as_deleted", func() (bool, domain.OperationStats, error) {
		return s.store.ResolvePendingWithdrawalAsDeleted(ctx, completionID, actorAccountID, at)
	})
}

func (s *Service) RedactWithdrawalsForDeletedStudent(ctx context.Context, studentID, actorAccountID int64, at time.Time) (int, error) {
	return runValue(s, "redact_withdrawals_for_deleted_student", func() (int, domain.OperationStats, error) {
		return s.store.RedactWithdrawalsForDeletedStudent(ctx, studentID, actorAccountID, at)
	})
}

// QueueCareDocumentCleanupForDeletedStudent records an immediately eligible
// cleanup intent for every document of the child whose bytes still exist.
// The intents carry no student foreign key, so they survive the cascade that
// removes the document rows with the child.
func (s *Service) QueueCareDocumentCleanupForDeletedStudent(ctx context.Context, studentID int64, at time.Time) (int, error) {
	return runValue(s, "queue_care_document_cleanup_for_deleted_student", func() (int, domain.OperationStats, error) {
		documents, stats, err := s.store.ListPendingCareDocumentCleanup(ctx, studentID)
		if err != nil {
			return 0, stats, err
		}
		for _, document := range documents {
			_, queueStats, err := s.store.QueueCareDocumentCleanup(ctx, domain.CareDocumentCleanup{
				OwnerID: studentID, FilenameStored: document.FilenameStored, RetryAfter: at,
			})
			stats.Add(queueStats)
			if err != nil {
				return 0, stats, err
			}
		}
		return len(documents), stats, nil
	})
}
