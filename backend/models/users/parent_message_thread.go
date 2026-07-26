package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Sender kinds shared by threads and messages. A message originates either
// from a guardian (parents portal) or from staff (tenant portal).
const (
	ParentMessageSenderGuardian = "guardian"
	ParentMessageSenderStaff    = "staff"
	ParentMessageSenderSystem   = "system"
)

// ParentMessageThread is the ONE continuous conversation between the OGS team
// and a SINGLE guardian about one child (chat model, no subject). There is
// exactly one thread per (tenant, student, guardian) — the "send" path is
// get-or-create. LastSenderKind lets the inbox flag "awaiting OGS reply"
// without scanning the messages.
type ParentMessageThread struct {
	base.Model `bun:"schema:users,table:parent_message_threads"`
	base.TenantModel
	StudentID         int64      `bun:"student_id,notnull" json:"student_id"`
	GuardianAccountID int64      `bun:"guardian_account_id,notnull" json:"guardian_account_id"`
	LastMessageAt     *time.Time `bun:"last_message_at" json:"last_message_at,omitempty"`
	// LastMessageID is the id of whichever message last set LastMessageAt. It is
	// the second half of the (created_at, id) composite the message log orders
	// by, so TouchLastMessage can break ties when two messages share a
	// created_at (clock_timestamp() can collide): the higher-id message wins,
	// exactly as ListByThread and the unread cursor order them. Every path that
	// updates LastMessageAt MUST set this in the same UPDATE.
	LastMessageID  *int64  `bun:"last_message_id" json:"last_message_id,omitempty"`
	LastSenderKind *string `bun:"last_sender_kind" json:"last_sender_kind,omitempty"`
	// LastMessageBody denormalizes the body of whichever message last set
	// LastMessageAt, so the inbox projection reads the preview straight off the
	// thread row instead of a correlated subquery that re-scans parent_messages
	// per thread row. Every code path that updates LastMessageAt MUST also set
	// this in the same UPDATE.
	LastMessageBody string `bun:"last_message_body" json:"last_message_body,omitempty"`
}

// ParentMessageThreadRepository is the tenant-scoped data-access contract for
// message threads. All methods must run inside a tenant transaction.
type ParentMessageThreadRepository interface {
	Create(ctx context.Context, thread *ParentMessageThread) error
	Update(ctx context.Context, thread *ParentMessageThread) error
	// TouchLastMessage atomically advances the denormalized last-activity fields
	// (last_message_at, last_message_id, last_sender_kind, last_message_body) that
	// drive the inbox preview/order, but ONLY when the message's (at, messageID)
	// composite is newer than the stored (last_message_at, last_message_id). This
	// is the single, concurrency-safe write path for those fields: it makes them
	// monotonic in the SAME (created_at, id) order the message log uses, so two
	// near-simultaneous sends (staff + guardian) to the same thread can't have the
	// later-committing-but-older one clobber the newer preview/order — including
	// when both messages share a created_at, where the higher id wins. Callers
	// must still insert the message row separately.
	TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error
	// FindByID returns the thread by id (tenant-scoped), or nil when absent.
	FindByID(ctx context.Context, id int64) (*ParentMessageThread, error)
	// FindByStudentGuardian returns the single conversation for a (student,
	// guardian) pair in the current tenant, or nil when none exists yet.
	// Powers the get-or-create "send" path on both portals.
	FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64) (*ParentMessageThread, error)
	// GetOrCreate returns the existing (student, guardian) conversation in the
	// given tenant, atomically creating it when absent. It is concurrency-safe:
	// two simultaneous first-messages (parent double-click, or parent + staff at
	// once) both resolve to the SAME thread via INSERT ... ON CONFLICT DO NOTHING
	// against the (tenant_id, student_id, guardian_account_id) unique constraint,
	// instead of the loser hitting a unique violation that surfaces as a 500 on an
	// actually-successful send. This is the single owner of the get-or-create
	// logic for both the staff and parent services.
	GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64) (*ParentMessageThread, error)
	// ListGuardiansForStudent returns the child's guardians that have a portal
	// account (i.e. can receive messages), primary first. Powers the staff
	// "new conversation" recipient picker.
	ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*MessageableGuardian, error)
}

// MessageableGuardian is one guardian of a child who has a parent-portal
// account and can therefore be the recipient of a thread.
type MessageableGuardian struct {
	AccountID        int64  `bun:"account_id" json:"account_id"`
	Name             string `bun:"name" json:"name"`
	RelationshipType string `bun:"relationship_type" json:"relationship_type"`
	IsPrimary        bool   `bun:"is_primary" json:"is_primary"`
}
