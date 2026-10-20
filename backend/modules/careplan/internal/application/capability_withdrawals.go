package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// The withdrawal commands deliberately do not open a transaction: like the
// retained repository they replace, each statement joins the caller's
// transaction when one is active and otherwise runs on its own (#3221).

func (s *Service) FindWithdrawalCompletion(ctx context.Context, id int64, lock bool) (careplan.WithdrawalCompletion, error) {
	value, err := runValue(ctx, s, "find_withdrawal_completion", func(ctx context.Context) (domain.WithdrawalCompletion, domain.OperationStats, error) {
		value, found, stats, err := s.store.FindWithdrawalCompletion(ctx, id, lock)
		if err == nil && !found {
			err = careplan.ErrWithdrawalNotFound
		}
		return value, stats, err
	})
	return value, err
}

func (s *Service) ListPendingWithdrawals(ctx context.Context, filter careplan.WithdrawalListFilter) ([]careplan.WithdrawalCompletion, int, error) {
	return s.listWithdrawals(ctx, careplan.WithdrawalStatePending, filter)
}

func (s *Service) ListResolvedWithdrawals(ctx context.Context, filter careplan.WithdrawalListFilter) ([]careplan.WithdrawalCompletion, int, error) {
	return s.listWithdrawals(ctx, careplan.WithdrawalStateResolved, filter)
}

func (s *Service) ListPendingWithdrawalsByStudent(ctx context.Context, studentIDs []int64) (map[int64]careplan.WithdrawalCompletion, error) {
	if len(studentIDs) == 0 {
		return map[int64]careplan.WithdrawalCompletion{}, nil
	}

	values, err := runValue(ctx, s, "list_pending_withdrawals_by_student", func(ctx context.Context) (map[int64]domain.WithdrawalCompletion, domain.OperationStats, error) {
		return s.store.ListPendingWithdrawalsByStudent(ctx, studentIDs)
	})
	return values, err
}

func (s *Service) ListPendingWithdrawalStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	if len(studentIDs) == 0 {
		return map[int64]bool{}, nil
	}

	values, err := runValue(ctx, s, "list_pending_withdrawal_student_ids", func(ctx context.Context) (map[int64]bool, domain.OperationStats, error) {
		return s.store.ListPendingWithdrawalStudentIDs(ctx, studentIDs)
	})
	return values, err
}

func (s *Service) ListPendingWithdrawalBoundaries(ctx context.Context, studentIDs []int64, includeBookingExpired bool) (map[int64]careplan.Date, error) {
	if len(studentIDs) == 0 {
		return map[int64]careplan.Date{}, nil
	}

	values, err := runValue(ctx, s, "list_pending_withdrawal_boundaries", func(ctx context.Context) (map[int64]domain.Date, domain.OperationStats, error) {
		return s.store.ListPendingWithdrawalBoundaries(ctx, studentIDs, includeBookingExpired)
	})
	return values, err
}

func (s *Service) UpsertPendingWithdrawal(ctx context.Context, value careplan.WithdrawalCompletion) (careplan.WithdrawalCompletion, error) {
	if value.StudentID == nil || *value.StudentID <= 0 {
		return careplan.WithdrawalCompletion{}, invalid(careplan.ErrInvalidWithdrawal, "care withdrawal completion requires a student")
	}

	stored, err := runValue(ctx, s, "upsert_pending_withdrawal", func(ctx context.Context) (domain.WithdrawalCompletion, domain.OperationStats, error) {
		return s.store.UpsertPendingWithdrawal(ctx, value)
	})
	return stored, err
}

func (s *Service) ResolveWithdrawal(ctx context.Context, id, actorAccountID int64, at time.Time) (bool, error) {
	changed, err := runValue(ctx, s, "resolve_withdrawal", func(ctx context.Context) (bool, domain.OperationStats, error) {
		return s.store.ResolveWithdrawal(ctx, id, actorAccountID, at)
	})
	return changed, err
}

func (s *Service) MarkWithdrawalObsoleteForRebooking(ctx context.Context, studentID int64, careStartsOn careplan.Date, at time.Time) (bool, error) {
	changed, err := runValue(ctx, s, "obsolete_withdrawal_for_rebooking", func(ctx context.Context) (bool, domain.OperationStats, error) {
		return s.store.MarkWithdrawalObsoleteForRebooking(ctx, studentID, careStartsOn, at)
	})
	return changed, err
}

func (s *Service) MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx context.Context, at time.Time) (int, error) {
	changed, err := runValue(ctx, s, "obsolete_pending_withdrawals_for_weekly_plans", func(ctx context.Context) (int, domain.OperationStats, error) {
		return s.store.MarkPendingWithdrawalsObsoleteForWeeklyPlans(ctx, at)
	})
	return changed, err
}

func (s *Service) ReopenWithdrawalAfterCancelledExit(ctx context.Context, completionID, studentID int64, at time.Time) (bool, error) {
	changed, err := runValue(ctx, s, "reopen_withdrawal_after_cancelled_exit", func(ctx context.Context) (bool, domain.OperationStats, error) {
		return s.store.ReopenWithdrawalAfterCancelledExit(ctx, completionID, studentID, at)
	})
	return changed, err
}
