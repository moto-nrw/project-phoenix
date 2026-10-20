package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) InsertRequest(ctx context.Context, req *enrollment.Request) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.InsertRequest(ctx, req)
}

func (m *Service) RequestsByID(ctx context.Context, ids []int64) ([]*enrollment.Request, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.RequestsByID(ctx, ids)
}

func (m *Service) RequestByID(ctx context.Context, id int64, forUpdate bool) (*enrollment.Request, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.RequestByID(ctx, id, forUpdate)
}

func (m *Service) AdminRequests(ctx context.Context, filters enrollment.RequestListFilters) ([]*enrollment.Request, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.AdminRequests(ctx, filters)
}

func (m *Service) UpdateRequestGuardian(ctx context.Context, req *enrollment.Request, includeEmail bool) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.UpdateRequestGuardian(ctx, req, includeEmail)
}

// RequestByToken uses the caller's authorized transaction. Public status flows
// discover the school by token in an admin transaction before tenant access.
func (m *Service) RequestByToken(ctx context.Context, token string, forUpdate bool) (*enrollment.Request, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.RequestByToken(ctx, token, forUpdate)
}
