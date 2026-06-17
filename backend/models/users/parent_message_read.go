package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableUsersParentMessageReads = "users.parent_message_reads"

// ParentMessageRead is a reader's read cursor in a thread. The reader is an
// auth.account — guardian or staff alike. Unread for that reader is any
// message in the thread after LastReadAt that the reader did not send.
type ParentMessageRead struct {
	bun.BaseModel `bun:"table:users.parent_message_reads,alias:pmr"`
	base.TenantModel
	ThreadID   int64     `bun:"thread_id,pk" json:"thread_id"`
	AccountID  int64     `bun:"account_id,pk" json:"account_id"`
	LastReadAt time.Time `bun:"last_read_at,notnull" json:"last_read_at"`
}

// TableName returns the schema-qualified table name.
func (r *ParentMessageRead) TableName() string { return tableUsersParentMessageReads }

// InboxThread is the list projection of a thread: the thread (subject) plus
// the guardian (name + relationship to the child), the child, a preview of the
// latest message, and the viewer's unread count. Used by both the staff inbox
// and the parent thread list. Built by a join in the repository (bun tags
// drive the scan; json tags are unused here — the handler maps to a DTO).
type InboxThread struct {
	ThreadID          int64      `bun:"thread_id" json:"thread_id"`
	Subject           string     `bun:"subject" json:"subject"`
	StudentID         int64      `bun:"student_id" json:"student_id"`
	StudentName       string     `bun:"student_name" json:"student_name"`
	SchoolClass       string     `bun:"school_class" json:"school_class"`
	GroupID           *int64     `bun:"group_id" json:"group_id,omitempty"`
	GroupName         string     `bun:"group_name" json:"group_name,omitempty"`
	GuardianAccountID int64      `bun:"guardian_account_id" json:"guardian_account_id"`
	GuardianName      string     `bun:"guardian_name" json:"guardian_name"`
	RelationshipType  string     `bun:"relationship_type" json:"relationship_type"`
	LastMessageAt     *time.Time `bun:"last_message_at" json:"last_message_at,omitempty"`
	LastSenderKind    string     `bun:"last_sender_kind" json:"last_sender_kind,omitempty"`
	LastMessageBody   string     `bun:"last_message_body" json:"last_message_body,omitempty"`
	UnreadCount       int        `bun:"unread_count" json:"unread_count"`
}

// ParentMessageReadRepository tracks read cursors and answers inbox/unread
// queries for staff. All methods run inside a tenant transaction.
type ParentMessageReadRepository interface {
	// MarkRead upserts the reader's cursor for a thread to now().
	MarkRead(ctx context.Context, tenantID, threadID, accountID int64) error
	// UnreadCountForStudents counts threads with at least one message the
	// reader has not seen, sent by the other side, restricted to the given
	// student IDs. A nil studentIDs means "all students in tenant".
	UnreadThreadCountForStaff(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error)
	// ListInboxForStaff returns the staff member's readable threads,
	// newest-activity first; unread counts guardian messages.
	ListInboxForStaff(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64, onlyUnread bool) ([]*InboxThread, error)
	// ListThreadsForGuardian returns the guardian's own threads in the
	// current tenant, newest-activity first; unread counts staff messages.
	ListThreadsForGuardian(ctx context.Context, accountID int64) ([]*InboxThread, error)
	// FindThreadProjection returns the list projection (subject, child,
	// guardian, relationship) for a single thread, or nil if absent. Used to
	// build the chat-window header.
	FindThreadProjection(ctx context.Context, threadID, accountID int64) (*InboxThread, error)
	// UnreadCountInThread counts messages in the thread from the given
	// sender side (ParentMessageSenderGuardian / ...Staff) created after the
	// reader's cursor.
	UnreadCountInThread(ctx context.Context, threadID, accountID int64, fromSenderKind string) (int, error)
}
