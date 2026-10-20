package application

import (
	"context"
)

// CountChangeRequestsForReview counts the status-filtered queue without pagination.
func (m *Service) CountChangeRequestsForReview(ctx context.Context, statuses []string) (int, error) {
	if len(statuses) == 0 {
		return 0, nil
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.CountChangeRequestsForReview(ctx, statuses)
}
