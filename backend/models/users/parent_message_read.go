package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// ParentMessageRead is a reader's read cursor in a thread. The reader is an
// auth.account — guardian or staff alike. Unread for that reader is any message
// in the thread after the cursor (LastReadAt, LastReadMessageID) that the reader
// did not send.
type ParentMessageRead struct {
	bun.BaseModel `bun:"table:users.parent_message_reads,alias:pmr"`
	base.TenantModel
	ThreadID   int64     `bun:"thread_id,pk" json:"thread_id"`
	AccountID  int64     `bun:"account_id,pk" json:"account_id"`
	LastReadAt time.Time `bun:"last_read_at,notnull" json:"last_read_at"`
	// LastReadMessageID is the id tie-breaker for LastReadAt: the cursor is the
	// composite (LastReadAt, LastReadMessageID). Two messages in a thread can
	// share a created_at, and the message list orders ties by id DESC, so unread
	// is "(created_at, id) > (last_read_at, last_read_message_id)". Without the id,
	// a tied message that committed after the reader's snapshot would be wrongly
	// counted as read. 0 means "before any message".
	LastReadMessageID int64 `bun:"last_read_message_id,notnull" json:"last_read_message_id"`
}

// InboxThread is the list projection of a conversation: the child, the
// guardian (name + relationship), the school (for the parent-facing "OGS
// [Schulname]" label), a preview of the latest message, and the viewer's
// unread count. Used by both the staff inbox and the parent thread list. Built
// by a join in the repository (bun tags drive the scan; json tags are unused
// here — the handler maps to a DTO).
type InboxThread struct {
	ThreadID int64 `bun:"thread_id" json:"thread_id"`
	// TenantID is the thread's owning school. Internal only (json:"-"): the parent
	// service uses it to suppress the unread pill for a school that has turned
	// messaging off, so the per-row count matches the sidebar badge. StudentID
	// alone is NOT globally unique across tenants, so the row must carry its own
	// tenant to be mapped to the per-tenant enabled flag.
	TenantID          int64      `bun:"tenant_id" json:"-"`
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
	// Structured fields of the thread's last message (kind / event_type /
	// request_type / request_status), projected from a join on last_message_id.
	// Internal only (json:"-"): the German-only staff inbox renders
	// LastMessageBody directly, but the LOCALIZED parents portal cannot — a
	// request title or a decision/withdrawal system-event body is German on the
	// wire. The parent handler exposes these on its own DTO so the parents portal
	// localizes the preview from fields (the same way the full conversation
	// localizes each card) instead of showing the German body until the next
	// plain message arrives.
	LastMessageKind        string `bun:"last_message_kind" json:"-"`
	LastEventType          string `bun:"last_event_type" json:"-"`
	LastRequestType        string `bun:"last_request_type" json:"-"`
	LastRequestStatus      string `bun:"last_request_status" json:"-"`
	LastMessageReadByStaff bool   `bun:"last_message_read_by_staff" json:"-"`
	UnreadCount            int    `bun:"unread_count" json:"unread_count"`
}

// ReadCursor is the composite position of a read cursor: the read instant and
// its message-id tie-breaker. The "OGS hat gelesen" receipt compares guardian
// messages against it using the SAME composite tie-break as the unread
// predicates, so a message tied on created_at with a higher id is only marked
// staff-read once the staff cursor actually reached that id.
type ReadCursor struct {
	LastReadAt        time.Time `bun:"last_read_at"`
	LastReadMessageID int64     `bun:"last_read_message_id"`
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
	// MarkReadUpTo upserts the reader's cursor to (readAt, readMessageID) — the
	// created_at AND id of the newest message actually shown — rather than now(),
	// for callers that record the read AFTER snapshotting the message list. Marking
	// to now() would advance the cursor past a message that committed between the
	// snapshot and the mark, silently dropping it from the reader's unread count
	// even though it was never shown. readMessageID is the tie-breaker that keeps a
	// message sharing the newest created_at from being swallowed (see the unread
	// predicates). The cursor never moves backward (it advances only when the new
	// composite is greater than the stored one). Returns whether the cursor actually
	// advanced, so the read-receipt SSE push can fire only on a real move and not
	// ping-pong with the refetch it triggers on the counterpart.
	MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error)
	// MarkStaffHandledUpTo advances the team-wide handled boundary to the newest
	// guardian activity covered by a staff reply. It never moves backward.
	MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error
	// UnreadMessageCountForStaff counts unread guardian MESSAGES the staff reader
	// has not seen (sent by the other side), within the given student scope — the
	// sidebar badge source. Counts messages, not threads, so the badge matches the
	// per-thread unread pills. allStudents = whole tenant; otherwise scoped to
	// groupIDs.
	UnreadMessageCountForStaff(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64) (int, error)
	// ListInboxForStaff returns the staff member's readable threads,
	// newest-activity first; unread counts guardian messages.
	ListInboxForStaff(ctx context.Context, accountID int64, allStudents bool, groupIDs []int64, onlyUnread bool) ([]*InboxThread, error)
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
	// UnreadMessageCountForGuardianTenants is the cross-tenant variant: it counts
	// the guardian's unread staff MESSAGES across the given tenants in ONE query, so
	// the portal badge does not open one transaction per school. Counts messages,
	// not threads (matching the staff side and the per-thread pills). Run under
	// WithAdminTx.
	UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
	// FindThreadHeader returns just the chat-window header fields (student name,
	// guardian name, relationship) for a single thread with a light join, or nil
	// if absent. Unlike the inbox projection it does NOT run the correlated
	// unread COUNT subquery the chat header never uses.
	FindThreadHeader(ctx context.Context, threadID int64) (*ThreadHeader, error)
	// LatestReadCursorByOther returns the furthest read cursor (by the composite
	// (last_read_at, last_read_message_id)) held in the thread by a STAFF account
	// other than excludeAccountID, or nil when no such cursor exists. Drives the
	// "OGS hat gelesen" receipt, which must compare on the same composite the
	// unread predicates use rather than the timestamp alone.
	LatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*ReadCursor, error)
	// GuardianReadCursor returns the read cursor of the thread's guardian account
	// (t.guardian_account_id), or nil when the guardian has not read anything yet.
	// Drives the staff-facing "von den Eltern gelesen" receipt — the mirror of
	// LatestReadCursorByOther, but simpler: a thread has exactly ONE guardian
	// account, so no aggregation over multiple readers is needed.
	GuardianReadCursor(ctx context.Context, threadID int64) (*ReadCursor, error)
}
