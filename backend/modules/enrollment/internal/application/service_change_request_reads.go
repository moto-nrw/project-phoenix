package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) ChangeRequestByID(ctx context.Context, id int64) (*enrollment.ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChangeRequestByID(ctx, id)
}

func (m *Service) ChangeRequestByIDForUpdate(ctx context.Context, id int64) (*enrollment.ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChangeRequestByIDForUpdate(ctx, id)
}

func (m *Service) ChangeRequestsForRequest(ctx context.Context, requestID int64) ([]*enrollment.ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChangeRequestsForRequest(ctx, requestID)
}

func (m *Service) OpenChangeRequestsForRequestForUpdate(ctx context.Context, requestID int64) ([]*enrollment.ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.OpenChangeRequestsForRequestForUpdate(ctx, requestID)
}

func (m *Service) ListChangeRequests(ctx context.Context, filters enrollment.ChangeRequestListFilters) ([]*enrollment.ChangeRequest, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ListChangeRequests(ctx, filters)
}

func (m *Service) ChangeRequestsForReview(ctx context.Context, filters enrollment.ChangeRequestReviewFilters) ([]*enrollment.ChangeRequest, error) {
	if len(filters.Statuses) == 0 || filters.Limit <= 0 {
		return []*enrollment.ChangeRequest{}, nil
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChangeRequestsForReview(ctx, filters)
}
