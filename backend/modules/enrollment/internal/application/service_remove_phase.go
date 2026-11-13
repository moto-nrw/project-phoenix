package application

import (
	"context"
	"fmt"
)

// RemovePhase deletes an existing phase through DeletePhase in its own
// transaction, so a phase with enrollment requests is refused (#6836).
// Application workflows detach sourced templates before invoking this
// command in the same transaction.
func (m *Service) RemovePhase(ctx context.Context, id int64) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		if _, err := m.store.Phase(txCtx, id); err != nil {
			return fmt.Errorf("phase delete: find phase: %w", err)
		}
		return m.DeletePhase(txCtx, id)
	})
}
