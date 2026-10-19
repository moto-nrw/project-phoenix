package enrollment

import "context"

func (m *Module) DeleteRequestTree(ctx context.Context, requestID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.DeleteRequestTree(ctx, requestID)
}

func (m *Module) DeleteRequestChildTree(ctx context.Context, requestID, childID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.DeleteRequestChildTree(ctx, requestID, childID)
}
