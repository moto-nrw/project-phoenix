package enrollment

import "context"

func (m *Module) DeletionRequestCounts(ctx context.Context, requestID int64) (*DeletionRequestCounts, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.DeletionRequestCounts(ctx, requestID)
}
func (m *Module) DeletionChildTarget(ctx context.Context, requestID, childID int64) (*DeletionChildTarget, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.DeletionChildTarget(ctx, requestID, childID)
}
func (m *Module) DeletionChildCounts(ctx context.Context, requestID, childID int64) (*DeletionChildCounts, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.DeletionChildCounts(ctx, requestID, childID)
}
func (m *Module) DeletionGuardianProfileIDs(ctx context.Context, requestID int64) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.DeletionGuardianProfileIDs(ctx, requestID)
}
func (m *Module) DeletionBlockingStudentIDs(ctx context.Context, requestID int64, childID *int64) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.DeletionBlockingStudentIDs(ctx, requestID, childID)
}
