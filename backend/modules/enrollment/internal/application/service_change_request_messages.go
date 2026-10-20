package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) InsertChangeRequestMessage(ctx context.Context, message *enrollment.ChangeRequestMessage) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.InsertChangeRequestMessage(ctx, message)
}

func (m *Service) ChangeRequestMessages(ctx context.Context, ids []int64, includeInternal bool) ([]*enrollment.ChangeRequestMessage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ChangeRequestMessages(ctx, ids, includeInternal)
}
