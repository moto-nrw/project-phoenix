package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) PhaseExpirySnapshots(ctx context.Context, input enrollment.PhaseExpiryInput) ([]*enrollment.PhaseExpirySnapshot, error) {
	if input.AsOf.IsZero() || input.WarningThrough.IsZero() {
		return nil, errors.New("phase expiry report dates are required")
	}
	if input.WarningThrough.Before(input.AsOf) {
		return nil, errors.New("phase expiry warning horizon must not be before the report date")
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PhaseExpirySnapshots(ctx, input)
}
