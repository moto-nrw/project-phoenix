package enrollment

import "context"

func (m *Module) InsertChild(ctx context.Context, child *RequestChild) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.InsertChild(ctx, child)
}
func (m *Module) ChildByID(ctx context.Context, id int64) (*RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChildByID(ctx, id)
}
func (m *Module) ChildrenByID(ctx context.Context, ids []int64) ([]*RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChildrenByID(ctx, ids)
}
func (m *Module) PhaseResponseChildren(ctx context.Context, phaseID int64, statuses []string) ([]PhaseResponseChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.PhaseResponseChildren(ctx, phaseID, statuses)
}
func (m *Module) ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChildrenForRequest(ctx, requestID, forUpdate)
}
func (m *Module) ChildrenForRequests(ctx context.Context, requestIDs []int64) ([]*RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChildrenForRequests(ctx, requestIDs)
}
func (m *Module) ChildrenByPhaseStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChildrenByPhaseStatuses(ctx, phaseID, statuses)
}
func (m *Module) UpdateChildData(ctx context.Context, child *RequestChild) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.UpdateChildData(ctx, child)
}
