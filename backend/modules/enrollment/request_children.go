package enrollment

import "context"

func (m *Module) InsertChild(ctx context.Context, child *RequestChild) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.InsertChild(ctx, child) })
}
func (m *Module) ChildByID(ctx context.Context, id int64) (*RequestChild, error) {
	var result *RequestChild
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error { var err error; result, err = m.engine.ChildByID(ctx, id); return err })
	return result, err
}
func (m *Module) ChildrenByID(ctx context.Context, ids []int64) ([]*RequestChild, error) {
	var result []*RequestChild
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.ChildrenByID(ctx, ids)
		return err
	})
	return result, err
}
func (m *Module) ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*RequestChild, error) {
	var result []*RequestChild
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.ChildrenForRequest(ctx, requestID, forUpdate)
		return err
	})
	return result, err
}
func (m *Module) ChildrenForRequests(ctx context.Context, requestIDs []int64) ([]*RequestChild, error) {
	var result []*RequestChild
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.ChildrenForRequests(ctx, requestIDs)
		return err
	})
	return result, err
}
func (m *Module) ChildrenByPhaseStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*RequestChild, error) {
	var result []*RequestChild
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.ChildrenByPhaseStatuses(ctx, phaseID, statuses)
		return err
	})
	return result, err
}
func (m *Module) UpdateChildData(ctx context.Context, child *RequestChild) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.UpdateChildData(ctx, child) })
}
