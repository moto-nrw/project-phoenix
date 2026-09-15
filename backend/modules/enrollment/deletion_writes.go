package enrollment

import "context"

func (m *Module) DeleteRequestTree(ctx context.Context, requestID int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.DeleteRequestTree(ctx, requestID)
	})
}

func (m *Module) DeleteRequestChildTree(ctx context.Context, requestID, childID int64) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		return m.engine.DeleteRequestChildTree(ctx, requestID, childID)
	})
}
