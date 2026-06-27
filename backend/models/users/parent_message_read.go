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

// InboxThread is the list projection of a conversation: the child, the
// guardian (name + relationship), the school (for the parent-facing "OGS
// [Schulname]" label), a preview of the latest message, and the viewer's
// unread count. Used by both the staff inbox and the parent thread list. Built
// by a join in the repository (bun tags drive the scan; json tags are unused
// here — the handler maps to a DTO).
type InboxThread struct {
	ThreadID          int64      `bun:"thread_id" json:"thread_id"`
	StudentID         int64      `bun:"student_id" json:"student_id"`
	StudentName       string     `bun:"student_name" json:"student_name"`
	SchoolClass       string     `bun:"school_class" json:"school_class"`
	SchoolName        string     `bun:"school_name" json:"school_name"`
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

// ThreadHeader is the minimal chat-window header projection: the child and
// guardian display names plus the relationship type. Built by a light join, it
// deliberately omits the inbox projection's unread count.
type ThreadHeader struct {
	StudentName      string `bun:"student_name"`
	GuardianName     string `bun:"guardian_name"`
	RelationshipType string `bun:"relationship_type"`
}

// ParentMessageReadRepository tracks read cursors and answers inbox/unread
// queries for staff. All methods run inside a tenant transaction.
type ParentMessageReadRepository interface {
	// MarkRead upserts the reader's cursor for a thread to now().
	MarkRead(ctx context.Context, tenantID, threadID, accountID int64) error
	// MarkReadUpTo upserts the reader's cursor to readAt (the timestamp of the
	// newest message actually shown) rather than now(), for callers that record
	// the read AFTER snapshotting the message list. Marking to now() would advance
	// the cursor past a message that committed between the snapshot and the mark,
	// silently dropping it from the reader's unread count even though it was never
	// shown. The cursor never moves backward (GREATEST of old/new).
	MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time) error
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
	// ListThreadsForGuardianStudent returns the guardian's own threads about one
	// of their children in the current tenant; unread counts staff messages. Runs
	// under the child's tenant tx (the caller resolves ownership first).
	ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*InboxThread, error)
	// ListThreadsForGuardianTenants is the cross-tenant variant: it returns the
	// guardian's threads across the given tenants in ONE query (newest-activity
	// first), so the parent portal does not open one transaction per school.
	// Intended to run under WithAdminTx (no per-request tenant scope).
	ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*InboxThread, error)
	// ListThreadsForStudent returns the staff view of one child's threads,
	// newest-activity first. The caller must have authorized read access.
	ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*InboxThread, error)
	// UnreadThreadCountForGuardian counts the guardian's threads in the current
	// tenant with unread staff-side activity (the parent-portal badge source).
	UnreadThreadCountForGuardian(ctx context.Context, accountID int64) (int, error)
	// UnreadThreadCountForGuardianTenants is the cross-tenant variant: it counts
	// the guardian's unread threads across the given tenants in ONE query, so the
	// portal badge does not open one transaction per school. Run under WithAdminTx.
	UnreadThreadCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
	// FindThreadHeader returns just the chat-window header fields (student name,
	// guardian name, relationship) for a single thread with a light join, or nil
	// if absent. Unlike the inbox projection it does NOT run the correlated
	// unread COUNT subquery the chat header never uses.
	FindThreadHeader(ctx context.Context, threadID int64) (*ThreadHeader, error)
	// UnreadCountInThread counts messages in the thread from the given
	// sender side (ParentMessageSenderGuardian / ...Staff) created after the
	// reader's cursor.
	UnreadCountInThread(ctx context.Context, threadID, accountID int64, fromSenderKind string) (int, error)
	// LatestReadAtByOther returns the newest read cursor in the thread among
	// accounts other than excludeAccountID.
	LatestReadAtByOther(ctx context.Context, threadID, excludeAccountID int64) (*time.Time, error)
}
