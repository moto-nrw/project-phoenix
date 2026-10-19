package enrollment

import "context"

// RestoreWithdrawnChildren restores waitlisted and submitted children atomically.
func (m *Module) RestoreWithdrawnChildren(ctx context.Context, id int64, waitlistedIDs []int64) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.RestoreWithdrawnChildren(ctx, id, waitlistedIDs)
}

func (m *Module) TransitionPhaseChildren(ctx context.Context, id int64, currentStatus, newStatus string) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.TransitionPhaseChildren(ctx, id, currentStatus, newStatus)
}
