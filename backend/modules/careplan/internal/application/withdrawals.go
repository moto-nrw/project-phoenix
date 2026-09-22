package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (s *Service) FindWithdrawalCompletion(ctx context.Context, id int64, lock bool) (domain.WithdrawalCompletion, error) {
	return runValue(s, "find_withdrawal_completion", func() (domain.WithdrawalCompletion, domain.OperationStats, error) {
		value, found, stats, err := s.store.FindWithdrawalCompletion(ctx, id, lock)
		if err == nil && !found {
			err = domain.ErrWithdrawalNotFound
		}
		return value, stats, err
	})
}

func (s *Service) ListWithdrawalStudentIDs(ctx context.Context, state string, studentID int64) ([]int64, error) {
	return runValue(s, "list_withdrawal_student_ids", func() ([]int64, domain.OperationStats, error) {
		return s.store.ListWithdrawalStudentIDs(ctx, state, studentID)
	})
}

// ListWithdrawals pages the tasks in state together with their total count.
func (s *Service) ListWithdrawals(ctx context.Context, state string, filter domain.WithdrawalListFilter) ([]domain.WithdrawalCompletion, int, error) {
	var total int
	values, err := runValue(s, "list_withdrawals", func() ([]domain.WithdrawalCompletion, domain.OperationStats, error) {
		values, count, stats, err := s.store.ListWithdrawals(ctx, state, filter)
		total = count
		return values, stats, err
	})
	return values, total, err
}

func (s *Service) ListPendingWithdrawalsByStudent(ctx context.Context, studentIDs []int64) (map[int64]domain.WithdrawalCompletion, error) {
	return runValue(s, "list_pending_withdrawals_by_student", func() (map[int64]domain.WithdrawalCompletion, domain.OperationStats, error) {
		return s.store.ListPendingWithdrawalsByStudent(ctx, studentIDs)
	})
}

func (s *Service) ListPendingWithdrawalStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	return runValue(s, "list_pending_withdrawal_student_ids", func() (map[int64]bool, domain.OperationStats, error) {
		return s.store.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
	})
}

func (s *Service) ListPendingWithdrawalBoundaries(ctx context.Context, studentIDs []int64, includeBookingExpired bool) (map[int64]domain.Date, error) {
	return runValue(s, "list_pending_withdrawal_boundaries", func() (map[int64]domain.Date, domain.OperationStats, error) {
		return s.store.ListPendingWithdrawalBoundaries(ctx, studentIDs, includeBookingExpired)
	})
}

func (s *Service) UpsertPendingWithdrawal(ctx context.Context, value domain.WithdrawalCompletion) (domain.WithdrawalCompletion, error) {
	return runValue(s, "upsert_pending_withdrawal", func() (domain.WithdrawalCompletion, domain.OperationStats, error) {
		return s.store.UpsertPendingWithdrawal(ctx, value)
	})
}

func (s *Service) ResolveWithdrawal(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	return runValue(s, "resolve_withdrawal", func() (bool, domain.OperationStats, error) {
		return s.store.ResolveWithdrawal(ctx, id, actorAccountID, at)
	})
}

func (s *Service) MarkWithdrawalObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn domain.Date, at time.Time) (bool, error) {
	return runValue(s, "obsolete_withdrawal_for_rebooking", func() (bool, domain.OperationStats, error) {
		return s.store.MarkWithdrawalObsoleteForRebooking(ctx, studentID, careStartsOn, at)
	})
}

func (s *Service) MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error) {
	return runValue(s, "obsolete_pending_withdrawals_for_weekly_plans", func() (int, domain.OperationStats, error) {
		return s.store.MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx, at)
	})
}

func (s *Service) ReopenWithdrawalAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, error) {
	return runValue(s, "reopen_withdrawal_after_cancelled_exit", func() (bool, domain.OperationStats, error) {
		return s.store.ReopenWithdrawalAfterCancelledExit(ctx, completionID, studentID, at)
	})
}
