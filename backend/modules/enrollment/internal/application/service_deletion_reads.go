package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) DeletionRequestCounts(ctx context.Context, requestID int64) (*enrollment.DeletionRequestCounts, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.DeletionRequestCounts(ctx, requestID)
}

func (m *Service) DeletionChildTarget(ctx context.Context, requestID, childID int64) (*enrollment.DeletionChildTarget, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.DeletionChildTarget(ctx, requestID, childID)
}

func (m *Service) DeletionChildCounts(ctx context.Context, requestID, childID int64) (*enrollment.DeletionChildCounts, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.DeletionChildCounts(ctx, requestID, childID)
}

func (m *Service) DeletionGuardianProfileIDs(ctx context.Context, requestID int64) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.DeletionGuardianProfileIDs(ctx, requestID)
}

func (m *Service) DeletionBlockingStudentIDs(ctx context.Context, requestID int64, childID *int64) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.DeletionBlockingStudentIDs(ctx, requestID, childID)
}
