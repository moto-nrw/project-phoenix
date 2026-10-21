package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) InsertChangeRequest(ctx context.Context, row *enrollment.ChangeRequest) error {
	if row.Status == "" {
		row.Status = "pending_review"
	}
	if row.Origin == "" {
		row.Origin = "parent"
	}
	if row.BaseSnapshot == nil {
		row.BaseSnapshot = []byte("{}")
	}
	if row.ProposedSnapshot == nil {
		row.ProposedSnapshot = []byte("{}")
	}
	if row.Diff == nil {
		row.Diff = []byte("{}")
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.InsertChangeRequest(ctx, row)
}
