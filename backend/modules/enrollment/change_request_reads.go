package enrollment

import "context"

func (m *Module) ChangeRequestByID(ctx context.Context, id int64) (*ChangeRequest, error) {
	var rows *ChangeRequest
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = m.engine.ChangeRequestByID(ctx, id)
		return err
	})
	return rows, err
}
func (m *Module) ChangeRequestByIDForUpdate(ctx context.Context, id int64) (*ChangeRequest, error) {
	var rows *ChangeRequest
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = m.engine.ChangeRequestByIDForUpdate(ctx, id)
		return err
	})
	return rows, err
}
func (m *Module) ChangeRequestsForRequest(ctx context.Context, requestID int64) ([]*ChangeRequest, error) {
	var rows []*ChangeRequest
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = m.engine.ChangeRequestsForRequest(ctx, requestID)
		return err
	})
	return rows, err
}
func (m *Module) OpenChangeRequestsForRequestForUpdate(ctx context.Context, requestID int64) ([]*ChangeRequest, error) {
	var rows []*ChangeRequest
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = m.engine.OpenChangeRequestsForRequestForUpdate(ctx, requestID)
		return err
	})
	return rows, err
}
func (m *Module) ListChangeRequests(ctx context.Context, filters ChangeRequestListFilters) ([]*ChangeRequest, error) {
	var rows []*ChangeRequest
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = m.engine.ListChangeRequests(ctx, filters)
		return err
	})
	return rows, err
}
func (m *Module) ChangeRequestsForReview(ctx context.Context, filters ChangeRequestReviewFilters) ([]*ChangeRequest, error) {
	if len(filters.Statuses) == 0 || filters.Limit <= 0 {
		return []*ChangeRequest{}, nil
	}
	var rows []*ChangeRequest
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = m.engine.ChangeRequestsForReview(ctx, filters)
		return err
	})
	return rows, err
}
