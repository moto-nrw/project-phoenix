package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) ApprovedBookings(ctx context.Context) ([]enrollment.ApprovedBooking, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ApprovedBookings(ctx)
}
