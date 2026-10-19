package enrollment

import "context"

func (m *Module) ChangeRequestByID(ctx context.Context, id int64) (*ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChangeRequestByID(ctx, id)
}
func (m *Module) ChangeRequestByIDForUpdate(ctx context.Context, id int64) (*ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChangeRequestByIDForUpdate(ctx, id)
}
func (m *Module) ChangeRequestsForRequest(ctx context.Context, requestID int64) ([]*ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChangeRequestsForRequest(ctx, requestID)
}
func (m *Module) OpenChangeRequestsForRequestForUpdate(ctx context.Context, requestID int64) ([]*ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.OpenChangeRequestsForRequestForUpdate(ctx, requestID)
}
func (m *Module) ListChangeRequests(ctx context.Context, filters ChangeRequestListFilters) ([]*ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ListChangeRequests(ctx, filters)
}
func (m *Module) ChangeRequestsForReview(ctx context.Context, filters ChangeRequestReviewFilters) ([]*ChangeRequest, error) {
	if len(filters.Statuses) == 0 || filters.Limit <= 0 {
		return []*ChangeRequest{}, nil
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChangeRequestsForReview(ctx, filters)
}
