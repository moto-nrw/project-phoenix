package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableExprParentMessagesAsMessage = `users.parent_messages AS "parent_message"`

// ParentMessageRepository is the tenant-scoped data-access layer for messages.
type ParentMessageRepository struct {
	*base.Repository[*users.ParentMessage]
}

// NewParentMessageRepository wires a fresh repository.
func NewParentMessageRepository(db *bun.DB) users.ParentMessageRepository {
	repo := base.NewRepository[*users.ParentMessage](db, "users.parent_messages", "ParentMessage")
	repo.TenantScoped = true
	return &ParentMessageRepository{Repository: repo}
}

// ListByThread returns a thread's messages oldest-first (chat order).
// limit <= 0 returns every message.
func (r *ParentMessageRepository) ListByThread(ctx context.Context, threadID int64, limit int) ([]*users.ParentMessage, error) {
	var messages []*users.ParentMessage
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&messages).
		ModelTableExpr(tableExprParentMessagesAsMessage).
		Where(`"parent_message".thread_id = ?`, threadID).
		OrderExpr(`"parent_message".created_at ASC`).
		OrderExpr(`"parent_message".id ASC`)

	if where, val, ok := base.TenantWhere(ctx, "parent_message"); ok {
		query = query.Where(where, val)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list parent messages", Err: err}
	}
	return messages, nil
}
