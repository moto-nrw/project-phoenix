package enrollment

import (
	"context"
	"time"
)

func (m *Module) SetChangeRequestStatus(ctx context.Context, id int64, status string) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.SetChangeRequestStatus(ctx, id, status)
}

func (m *Module) MarkChangeRequestReviewed(ctx context.Context, id int64, status string, note *string, reviewerID int64, at time.Time) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.MarkChangeRequestReviewed(ctx, id, status, note, reviewerID, at)
}
