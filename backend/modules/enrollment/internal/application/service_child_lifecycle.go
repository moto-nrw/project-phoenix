package application

import (
	"context"
)

// DeleteRequestChildren removes intake children, not materialized students.
func (m *Service) DeleteRequestChildren(ctx context.Context, requestID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeleteRequestChildren(ctx, requestID)
}

func (m *Service) CountCreatedStudentsByPhase(ctx context.Context, phaseID int64) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.CountCreatedStudentsByPhase(ctx, phaseID)
}
