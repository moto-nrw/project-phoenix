package enrollment

import "context"

func (m *Module) InsertRequest(ctx context.Context, req *Request) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.InsertRequest(ctx, req) })
}
func (m *Module) RequestsByID(ctx context.Context, ids []int64) ([]*Request, error) {
	var result []*Request
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.RequestsByID(ctx, ids)
		return err
	})
	return result, err
}
func (m *Module) RequestByID(ctx context.Context, id int64, forUpdate bool) (*Request, error) {
	var result *Request
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.RequestByID(ctx, id, forUpdate)
		return err
	})
	return result, err
}
func (m *Module) AdminRequests(ctx context.Context, filters RequestListFilters) ([]*Request, error) {
	var result []*Request
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.AdminRequests(ctx, filters)
		return err
	})
	return result, err
}
func (m *Module) UpdateRequestGuardian(ctx context.Context, req *Request, includeEmail bool) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.UpdateRequestGuardian(ctx, req, includeEmail) })
}

// RequestByToken uses the caller's authorized transaction. Public status flows
// discover the school by token in an admin transaction before tenant access.
func (m *Module) RequestByToken(ctx context.Context, token string, forUpdate bool) (*Request, error) {
	var result *Request
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.RequestByToken(ctx, token, forUpdate)
		return err
	})
	return result, err
}
