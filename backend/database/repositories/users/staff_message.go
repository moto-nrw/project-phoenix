package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableExprStaffMessagesAsMessage = `users.staff_messages AS "staff_message"`

// StaffMessageRepository is the tenant-scoped data-access layer for the
// OGS-internal message log.
type StaffMessageRepository struct {
	*base.Repository[*users.StaffMessage]
}

// NewStaffMessageRepository wires a fresh repository.
func NewStaffMessageRepository(db *bun.DB) users.StaffMessageRepository {
	repo := base.NewRepository[*users.StaffMessage](db, "users.staff_messages", "StaffMessage")
	repo.TenantScoped = true
	return &StaffMessageRepository{Repository: repo}
}

// Create appends one message and reads the persisted row back.
//
// created_at is deliberately NOT set by the application: the column defaults to
// clock_timestamp() so the message is stamped at insert time rather than at
// transaction start. The RETURNING clause pulls that value (and the id) back,
// because both halves of the (created_at, id) composite are what the caller
// feeds into TouchLastMessage and the read cursor — computing them in Go would
// reintroduce exactly the skew the database default exists to avoid.
func (r *StaffMessageRepository) Create(ctx context.Context, message *users.StaffMessage) error {
	base.EnsureTenantID(ctx, message)

	if _, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(message).
		ModelTableExpr(r.TableName).
		ExcludeColumn("created_at", "updated_at").
		Returning("id, created_at, updated_at").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create staff message", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// ListByThread returns the thread's messages oldest-first, which is the order
// the chat window renders. Ties on created_at break by id, matching the
// composite the unread cursor compares against.
func (r *StaffMessageRepository) ListByThread(ctx context.Context, threadID int64) ([]*users.StaffMessage, error) {
	var messages []*users.StaffMessage
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&messages).
		ModelTableExpr(tableExprStaffMessagesAsMessage).
		Where(`"staff_message".thread_id = ?`, threadID).
		OrderExpr(`"staff_message".created_at ASC, "staff_message".id ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_message")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff messages by thread", Err: base.TranslateNotFound(err)}
	}
	return messages, nil
}

// DeleteOlderThan removes messages created before the cutoff across the whole
// tenant (retention housekeeping) and reports how many rows went.
func (r *StaffMessageRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	query := base.GetDB(ctx, r.DB).NewDelete().
		Model((*users.StaffMessage)(nil)).
		ModelTableExpr(tableExprStaffMessagesAsMessage).
		Where(`"staff_message".created_at < ?`, cutoff)

	query = base.WithTenantFilter(ctx, query, "staff_message")

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete old staff messages", Err: base.TranslateNotFound(err)}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete old staff messages", Err: base.TranslateNotFound(err)}
	}
	return affected, nil
}
