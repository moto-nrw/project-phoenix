package application

import (
	"context"
	"time"
)

func (m *Service) SetChangeRequestStatus(ctx context.Context, id int64, status string) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.SetChangeRequestStatus(ctx, id, status)
}

func (m *Service) MarkChangeRequestReviewed(ctx context.Context, id int64, status string, note *string, reviewerID int64, at time.Time) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.MarkChangeRequestReviewed(ctx, id, status, note, reviewerID, at)
}
