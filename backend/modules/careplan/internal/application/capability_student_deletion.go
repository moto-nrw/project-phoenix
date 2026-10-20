package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// The permanent-deletion commands join the caller's transaction; the
// coordinating workflow holds the student locks and decides the order.

func (s *Service) FindPendingWithdrawalStudent(ctx context.Context, completionID int64, lock bool) (int64, error) {
	if completionID <= 0 {
		return 0, careplan.ErrWithdrawalNotFound
	}

	value, err := runValue(ctx, s, "find_pending_withdrawal_student", func(ctx context.Context) (int64, domain.OperationStats, error) {
		return s.store.FindPendingWithdrawalStudent(ctx, completionID, lock)
	})
	return value, err
}

func (s *Service) ResolvePendingWithdrawalAsDeleted(ctx context.Context, completionID, actorAccountID int64, at time.Time) (bool, error) {
	if completionID <= 0 || actorAccountID <= 0 || at.IsZero() {
		return false, careplan.ErrInvalidCareExit
	}

	value, err := runValue(ctx, s, "resolve_pending_withdrawal_as_deleted", func(ctx context.Context) (bool, domain.OperationStats, error) {
		return s.store.ResolvePendingWithdrawalAsDeleted(ctx, completionID, actorAccountID, at)
	})
	return value, err
}

// RedactWithdrawalsForDeletedStudent redacts the withdrawal tasks of a child
// the student deletion removes. An actorAccountID of 0 is the nightly
// retention run (#3962): it resolves a pending task without an actor.
func (s *Service) RedactWithdrawalsForDeletedStudent(ctx context.Context, studentID, actorAccountID int64, at time.Time) (int, error) {
	if studentID <= 0 || actorAccountID < 0 || at.IsZero() {
		return 0, careplan.ErrInvalidCareExit
	}

	value, err := runValue(ctx, s, "redact_withdrawals_for_deleted_student", func(ctx context.Context) (int, domain.OperationStats, error) {
		return s.store.RedactWithdrawalsForDeletedStudent(ctx, studentID, actorAccountID, at)
	})
	return value, err
}

func (s *Service) QueueCareDocumentCleanupForDeletedStudent(ctx context.Context, studentID int64, at time.Time) (int, error) {
	if studentID <= 0 || at.IsZero() {
		return 0, careplan.ErrInvalidCareDocument
	}

	value, err := runValue(ctx, s, "queue_care_document_cleanup_for_deleted_student", func(ctx context.Context) (int, domain.OperationStats, error) {
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
	return value, err
}
