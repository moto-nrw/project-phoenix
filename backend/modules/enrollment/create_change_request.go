package enrollment

import "context"

func (m *Module) InsertChangeRequest(ctx context.Context, row *ChangeRequest) error {
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
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.InsertChangeRequest(ctx, row) })
}
