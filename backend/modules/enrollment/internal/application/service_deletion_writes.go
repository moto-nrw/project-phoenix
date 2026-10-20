package application

import (
	"context"
)

func (m *Service) DeleteRequestTree(ctx context.Context, requestID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeleteRequestTree(ctx, requestID)
}

func (m *Service) DeleteRequestChildTree(ctx context.Context, requestID, childID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeleteRequestChildTree(ctx, requestID, childID)
}
