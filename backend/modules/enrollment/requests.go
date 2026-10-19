package enrollment

import "context"

func (m *Module) InsertRequest(ctx context.Context, req *Request) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.InsertRequest(ctx, req)
}
func (m *Module) RequestsByID(ctx context.Context, ids []int64) ([]*Request, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.RequestsByID(ctx, ids)
}
func (m *Module) RequestByID(ctx context.Context, id int64, forUpdate bool) (*Request, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.RequestByID(ctx, id, forUpdate)
}
func (m *Module) AdminRequests(ctx context.Context, filters RequestListFilters) ([]*Request, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.AdminRequests(ctx, filters)
}
func (m *Module) UpdateRequestGuardian(ctx context.Context, req *Request, includeEmail bool) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.UpdateRequestGuardian(ctx, req, includeEmail)
}

// RequestByToken uses the caller's authorized transaction. Public status flows
// discover the school by token in an admin transaction before tenant access.
func (m *Module) RequestByToken(ctx context.Context, token string, forUpdate bool) (*Request, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.RequestByToken(ctx, token, forUpdate)
}
