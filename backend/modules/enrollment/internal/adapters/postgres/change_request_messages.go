package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func (r *Store) InsertChangeRequestMessage(ctx context.Context, message *enrollment.ChangeRequestMessage) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	err = db.NewRaw(`INSERT INTO enrollment.change_request_messages
		(tenant_id, change_request_id, author_type, author_account_id, body, internal_only)
		SELECT ?, id, ?, ?, ?, ? FROM enrollment.change_requests WHERE id = ? AND tenant_id = ?
		RETURNING id, tenant_id, created_at, updated_at`, tenantID, message.AuthorType, message.AuthorAccountID,
		message.Body, message.InternalOnly, message.ChangeRequestID, tenantID).Scan(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to create enrollment change request message: %w", err)
	}
	return nil
}

func (r *Store) ChangeRequestMessages(ctx context.Context, ids []int64, includeInternal bool) ([]*enrollment.ChangeRequestMessage, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var messages []*enrollment.ChangeRequestMessage
	q := db.NewSelect().TableExpr("enrollment.change_request_messages").
		ColumnExpr("id, tenant_id, created_at, updated_at, change_request_id, author_type, author_account_id, body, internal_only").
		Where("tenant_id = ?", tenantID).Where("change_request_id IN (?)", bun.List(ids)).
		OrderExpr("change_request_id, created_at, id")
	if !includeInternal {
		q = q.Where("internal_only = FALSE")
	}
	if err := q.Scan(ctx, &messages); err != nil {
		return nil, fmt.Errorf("failed to list enrollment change request messages: %w", err)
	}
	return messages, nil
}
