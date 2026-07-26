package enrollment

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const changeRequestMessageTableExpr = `enrollment.change_request_messages AS "change_request_message"`

type ChangeRequestMessageRepository struct {
	db *bun.DB
}

func NewChangeRequestMessageRepository(db *bun.DB) enrollment.ChangeRequestMessageRepository {
	return &ChangeRequestMessageRepository{db: db}
}

func (r *ChangeRequestMessageRepository) Create(ctx context.Context, row *enrollment.ChangeRequestMessage) error {
	base.EnsureTenantID(ctx, row)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr(changeRequestMessageTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create enrollment change request message: %w", err)
	}
	return nil
}

func (r *ChangeRequestMessageRepository) ListByChangeRequestID(ctx context.Context, changeRequestID int64, includeInternal bool) ([]*enrollment.ChangeRequestMessage, error) {
	if changeRequestID <= 0 {
		return nil, nil
	}
	return r.list(ctx, []int64{changeRequestID}, includeInternal)
}

func (r *ChangeRequestMessageRepository) ListByChangeRequestIDs(ctx context.Context, changeRequestIDs []int64, includeInternal bool) ([]*enrollment.ChangeRequestMessage, error) {
	if len(changeRequestIDs) == 0 {
		return nil, nil
	}
	return r.list(ctx, changeRequestIDs, includeInternal)
}

func (r *ChangeRequestMessageRepository) list(ctx context.Context, changeRequestIDs []int64, includeInternal bool) ([]*enrollment.ChangeRequestMessage, error) {
	var rows []*enrollment.ChangeRequestMessage
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(changeRequestMessageTableExpr).
		Where(`"change_request_message".change_request_id IN (?)`, bun.List(changeRequestIDs)).
		OrderExpr(`"change_request_message".change_request_id, "change_request_message".created_at, "change_request_message".id`)
	if !includeInternal {
		q = q.Where(`"change_request_message".internal_only = FALSE`)
	}
	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list enrollment change request messages: %w", err)
	}
	return rows, nil
}
