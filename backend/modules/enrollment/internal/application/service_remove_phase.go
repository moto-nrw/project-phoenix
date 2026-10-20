package application

import (
	"context"
	"fmt"
)

// RemovePhase removes applications before the phase cascade. The ordering
// clears child-offering references before their offerings are deleted.
// Application workflows detach sourced templates before invoking this command
// in the same transaction. Materialized students are preserved.
func (m *Service) RemovePhase(ctx context.Context, id int64) (int, error) {
	deletedRequests := 0
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		if _, err := m.store.Phase(txCtx, id); err != nil {
			return fmt.Errorf("phase delete: find phase: %w", err)
		}
		var err error
		deletedRequests, err = m.store.DeletePhaseRequests(txCtx, id)
		if err != nil {
			return fmt.Errorf("phase delete: requests: %w", err)
		}
		if err := m.store.DeletePhase(txCtx, id); err != nil {
			return fmt.Errorf("phase delete: phase: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deletedRequests, nil
}
