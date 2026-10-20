package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) CreateRequestGuardian(ctx context.Context, guardian *enrollment.RequestGuardian) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.CreateRequestGuardian(ctx, guardian)
}

func (m *Service) RequestGuardians(ctx context.Context, requestIDs []int64) ([]*enrollment.RequestGuardian, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.RequestGuardians(ctx, requestIDs)
}

func (m *Service) DeleteRequestGuardians(ctx context.Context, requestID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeleteRequestGuardians(ctx, requestID)
}

func (m *Service) StampRequestGuardianProfile(ctx context.Context, id, profileID int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.StampRequestGuardianProfile(ctx, id, profileID)
}
