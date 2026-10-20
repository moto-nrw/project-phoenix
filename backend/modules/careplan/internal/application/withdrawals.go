package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// listWithdrawals pages the tasks in state together with their total count.
// The list reads the student directory, so it runs on its own store.
func (s *Service) listWithdrawals(ctx context.Context, state string, filter domain.WithdrawalListFilter) ([]domain.WithdrawalCompletion, int, error) {
	var total int
	values, err := runValue(ctx, s, "list_withdrawals", func(ctx context.Context) ([]domain.WithdrawalCompletion, domain.OperationStats, error) {
		values, count, stats, err := s.withdrawals.ListWithdrawals(ctx, state, filter)
		total = count
		return values, stats, err
	})
	return values, total, err
}
