package enrollment

import "context"

// CountChangeRequestsForReview counts the status-filtered queue without pagination.
func (m *Module) CountChangeRequestsForReview(ctx context.Context, statuses []string) (int, error) {
	if len(statuses) == 0 {
		return 0, nil
	}
	var count int
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		count, err = m.engine.CountChangeRequestsForReview(ctx, statuses)
		return err
	})
	return count, err
}
