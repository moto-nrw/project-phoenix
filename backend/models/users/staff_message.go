package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// StaffMessage is a single message in an OGS-internal colleague conversation
// (#2598). Append-only: each send is a new row, nothing is edited or deleted
// individually. SenderName is denormalized at send time so the history stays
// readable after an account is renamed or deactivated.
type StaffMessage struct {
	base.Model `bun:"schema:users,table:staff_messages"`
	base.TenantModel
	ThreadID        int64  `bun:"thread_id,notnull" json:"thread_id"`
	SenderAccountID int64  `bun:"sender_account_id,notnull" json:"sender_account_id"`
	SenderName      string `bun:"sender_name,notnull" json:"sender_name"`
	Body            string `bun:"body,notnull" json:"body"`
}

// StaffMessageRepository is the tenant-scoped data-access contract for the
// message log. All methods must run inside a tenant transaction.
type StaffMessageRepository interface {
	// Create appends one message. The database stamps CreatedAt with
	// clock_timestamp(), so the caller must read the persisted value back
	// (bun returning) before using it as a read-cursor bound.
	Create(ctx context.Context, message *StaffMessage) error
	// ListByThread returns the thread's messages oldest-first, which is the
	// order the chat window renders. Ties on created_at break by id, matching
	// the composite the unread cursor compares against.
	ListByThread(ctx context.Context, threadID int64) ([]*StaffMessage, error)
	// DeleteOlderThan removes messages created before the cutoff across the
	// whole tenant. Retention housekeeping; returns the number of rows deleted.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
