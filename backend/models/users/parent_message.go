package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableUsersParentMessages = "users.parent_messages"

const (
	ParentMessageKindMessage = "message"
	ParentMessageKindEvent   = "event"
	ParentMessageKindRequest = "request"

	ParentMessageRequestStatusOpen        = "offen"
	ParentMessageRequestStatusDone        = "erledigt"
	ParentMessageRequestStatusRejected    = "abgelehnt"
	ParentMessageRequestStatusWithdrawn   = "zurueckgezogen"
	ParentMessageRequestCareSchedule      = "care_schedule"
	ParentMessageRequestStudentMasterData = "student_master_data"
)

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
	// Kind discriminates a plain chat message ('message') from a system event
	// ('event', e.g. a quick-action pill posting a Raumwechsel/Abholung notice
	// into the thread) and a structured change request ('request'). Defaults to
	// 'message' on insert via BeforeAppendModel.
	Kind      string `bun:"kind,notnull,default:'message'" json:"kind"`
	EventType string `bun:"event_type,nullzero" json:"event_type,omitempty"`
	// EventActorKind records which side TRIGGERED a system event ('staff' or
	// 'guardian'), set from the action context when the event is created. The
	// unread query attributes the event to this side instead of inferring it from
	// sender_account_id, which is ambiguous for a dual-role staff+guardian
	// account. NULL for plain messages / requests (they carry their side in
	// SenderKind).
	EventActorKind string         `bun:"event_actor_kind,nullzero" json:"event_actor_kind,omitempty"`
	RequestType    string         `bun:"request_type,nullzero" json:"request_type,omitempty"`
	RequestStatus  string         `bun:"request_status,nullzero" json:"request_status,omitempty"`
	Payload        map[string]any `bun:"payload,type:jsonb" json:"payload,omitempty"`
	RefTable       string         `bun:"ref_table,nullzero" json:"ref_table,omitempty"`
	RefID          *int64         `bun:"ref_id" json:"ref_id,omitempty"`
	AppliedAt      *time.Time     `bun:"applied_at" json:"applied_at,omitempty"`
	AppliedBy      *int64         `bun:"applied_by" json:"applied_by,omitempty"`
	DecisionReason string         `bun:"decision_reason,nullzero" json:"decision_reason,omitempty"`
	// ReadByStaff is the parent-facing "OGS hat gelesen" indicator on a
	// guardian-authored message; ReadByGuardian is the staff-facing "von den
	// Eltern gelesen" indicator on a staff-authored message. Both are derived
	// presentation flags (bun:"-"), stamped by the Stamp*ReadReceipts helpers from
	// the relevant side's read cursor — never persisted.
	ReadByStaff    bool `bun:"-" json:"read_by_staff,omitempty"`
	ReadByGuardian bool `bun:"-" json:"read_by_guardian,omitempty"`
}

func (m *ParentMessage) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableUsersParentMessages)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableUsersParentMessages)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		if m.Kind == "" {
			m.Kind = ParentMessageKindMessage
		}
		q.ModelTableExpr(tableUsersParentMessages)
	}
	return nil
}

// GetID/GetCreatedAt/GetUpdatedAt are provided by the embedded base.Model.
func (m *ParentMessage) TableName() string { return tableUsersParentMessages }

// atOrBeforeCursor reports whether the message sits at or before the read cursor,
// comparing on the SAME composite (created_at, id) the unread predicates use, not
// the timestamp alone: two messages can share a created_at, and a timestamp-only
// "not after cutoff" test would mark a tied message with a higher id as read while
// the unread badge still counts it. Shared by both receipt directions so they
// cannot diverge from the unread maths.
func atOrBeforeCursor(msg *ParentMessage, cutoff *ReadCursor) bool {
	return msg.CreatedAt.Before(cutoff.LastReadAt) ||
		(msg.CreatedAt.Equal(cutoff.LastReadAt) && msg.ID <= cutoff.LastReadMessageID)
}

// StampStaffReadReceipts marks every guardian-authored message at or before the
// staff read cursor with ReadByStaff=true (the "OGS hat gelesen" indicator).
// cutoff is the furthest staff read cursor in the thread (nil = staff has not
// read anything yet, so nothing is stamped). Pure presentation logic shared by
// the parent side (services/parent) and the staff side (services/messaging) so
// the receipt rule cannot drift between the two views of the same thread.
func StampStaffReadReceipts(messages []*ParentMessage, cutoff *ReadCursor) {
	if cutoff == nil {
		return
	}
	for _, msg := range messages {
		// Gate POSITIVELY on guardian authorship. A "!= staff" test would also stamp
		// a system message (sender_kind 'system', introduced by the #1672 requests
		// follow-up) as staff-read — a wrong receipt on a row no staff member wrote.
		if msg.SenderKind != ParentMessageSenderGuardian {
			continue
		}
		if atOrBeforeCursor(msg, cutoff) {
			msg.ReadByStaff = true
		}
	}
}

// StampGuardianReadReceipts is the mirror of StampStaffReadReceipts for the staff
// view: it marks every STAFF-authored message at or before the guardian's read
// cursor with ReadByGuardian=true (the staff-facing "von den Eltern gelesen"
// indicator). cutoff is the thread guardian's read cursor (nil = the guardian has
// not read anything yet, so nothing is stamped). A thread has exactly one guardian
// account, so the cursor is that single account's position — no aggregation.
func StampGuardianReadReceipts(messages []*ParentMessage, cutoff *ReadCursor) {
	if cutoff == nil {
		return
	}
	for _, msg := range messages {
		// Gate POSITIVELY on staff authorship so a 'system' message is never stamped
		// as guardian-read (mirrors the guardian-authorship gate above).
		if msg.SenderKind != ParentMessageSenderStaff {
			continue
		}
		if atOrBeforeCursor(msg, cutoff) {
			msg.ReadByGuardian = true
		}
	}
}

// ParentMessageRepository is the tenant-scoped data-access contract for
// messages. All methods must run inside a tenant transaction.
type ParentMessageRepository interface {
	Create(ctx context.Context, message *ParentMessage) error
	Update(ctx context.Context, message *ParentMessage) error
	FindByID(ctx context.Context, id int64) (*ParentMessage, error)
	// FindByIDForUpdate is FindByID with a SELECT … FOR UPDATE row lock, so a
	// confirm/reject re-reads and locks the request inside the request
	// transaction before applying — two staff confirming the same request
	// serialize instead of both applying it.
	FindByIDForUpdate(ctx context.Context, id int64) (*ParentMessage, error)
	// ListByThread returns a thread's messages oldest-first (chat order).
	// limit <= 0 returns all.
	ListByThread(ctx context.Context, threadID int64, limit int) ([]*ParentMessage, error)
}
