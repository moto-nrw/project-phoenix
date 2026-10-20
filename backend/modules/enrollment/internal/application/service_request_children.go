package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) InsertChild(ctx context.Context, child *enrollment.RequestChild) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.InsertChild(ctx, child)
}

func (m *Service) ChildByID(ctx context.Context, id int64) (*enrollment.RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChildByID(ctx, id)
}

func (m *Service) ChildrenByID(ctx context.Context, ids []int64) ([]*enrollment.RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChildrenByID(ctx, ids)
}

func (m *Service) PhaseResponseChildren(ctx context.Context, phaseID int64, statuses []string) ([]enrollment.PhaseResponseChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PhaseResponseChildren(ctx, phaseID, statuses)
}

func (m *Service) ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*enrollment.RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChildrenForRequest(ctx, requestID, forUpdate)
}

func (m *Service) ChildrenForRequests(ctx context.Context, requestIDs []int64) ([]*enrollment.RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChildrenForRequests(ctx, requestIDs)
}

func (m *Service) ChildrenByPhaseStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*enrollment.RequestChild, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChildrenByPhaseStatuses(ctx, phaseID, statuses)
}

func (m *Service) UpdateChildData(ctx context.Context, child *enrollment.RequestChild) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.UpdateChildData(ctx, child)
}
