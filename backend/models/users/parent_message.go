package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableUsersParentMessages = "users.parent_messages"

// ParentMessage is a single message in a child's parent-OGS thread.
// Append-only: each send is a new row. SenderName is denormalized at send
// time so the history stays stable if an account/profile is later renamed
// or removed.
type ParentMessage struct {
	base.Model `bun:"schema:users,table:parent_messages"`
	base.TenantModel
	ThreadID        int64  `bun:"thread_id,notnull" json:"thread_id"`
	StudentID       int64  `bun:"student_id,notnull" json:"student_id"`
	SenderAccountID int64  `bun:"sender_account_id,notnull" json:"sender_account_id"`
	SenderKind      string `bun:"sender_kind,notnull" json:"sender_kind"`
	SenderName      string `bun:"sender_name,notnull" json:"sender_name"`
	Body            string `bun:"body,notnull" json:"body"`
	ReadByStaff     bool   `bun:"-" json:"read_by_staff,omitempty"`
}

func (m *ParentMessage) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableUsersParentMessages)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableUsersParentMessages)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableUsersParentMessages)
	}
	return nil
}

// GetID/GetCreatedAt/GetUpdatedAt are provided by the embedded base.Model.
func (m *ParentMessage) TableName() string { return tableUsersParentMessages }

// StampStaffReadReceipts marks every guardian-authored message at or before the
// staff read cursor with ReadByStaff=true (the "OGS hat gelesen" indicator).
// cutoff is the newest staff read instant in the thread (nil = staff has not
// read anything yet, so nothing is stamped). Pure presentation logic shared by
// the parent side (services/parent) and the staff side (services/messaging) so
// the receipt rule cannot drift between the two views of the same thread.
func StampStaffReadReceipts(messages []*ParentMessage, cutoff *time.Time) {
	if cutoff == nil {
		return
	}
	for _, msg := range messages {
		if msg.SenderKind != ParentMessageSenderStaff && !msg.CreatedAt.After(*cutoff) {
			msg.ReadByStaff = true
		}
	}
}

// ParentMessageRepository is the tenant-scoped data-access contract for
// messages. All methods must run inside a tenant transaction.
type ParentMessageRepository interface {
	Create(ctx context.Context, message *ParentMessage) error
	FindByID(ctx context.Context, id int64) (*ParentMessage, error)
	// ListByThread returns a thread's messages oldest-first (chat order).
	// limit <= 0 returns all.
	ListByThread(ctx context.Context, threadID int64, limit int) ([]*ParentMessage, error)
}
