package enrollment

import "context"

// RestoreWithdrawnChildren restores waitlisted and submitted children atomically.
func (m *Module) RestoreWithdrawnChildren(ctx context.Context, id int64, waitlistedIDs []int64) ([]int64, error) {
	var result []int64
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.RestoreWithdrawnChildren(ctx, id, waitlistedIDs)
		return err
	})
	return result, err
}

func (m *Module) TransitionPhaseChildren(ctx context.Context, id int64, currentStatus, newStatus string) (int, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		count, err = m.engine.TransitionPhaseChildren(ctx, id, currentStatus, newStatus)
		return err
	})
	return count, err
}
