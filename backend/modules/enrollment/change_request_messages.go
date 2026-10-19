package enrollment

import (
	"context"
	"time"
)

type ChangeRequestMessage struct {
	ID              int64     `json:"id"`
	TenantID        int64     `json:"tenant_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	ChangeRequestID int64     `json:"change_request_id"`
	AuthorType      string    `json:"author_type"`
	AuthorAccountID *int64    `json:"author_account_id,omitempty"`
	Body            string    `json:"body"`
	InternalOnly    bool      `json:"internal_only"`
}

func (m *Module) InsertChangeRequestMessage(ctx context.Context, message *ChangeRequestMessage) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.InsertChangeRequestMessage(ctx, message)
}

func (m *Module) ChangeRequestMessages(ctx context.Context, ids []int64, includeInternal bool) ([]*ChangeRequestMessage, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.ChangeRequestMessages(ctx, ids, includeInternal)
}
