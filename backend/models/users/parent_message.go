package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

const (
	ParentMessageKindMessage = "message"
	ParentMessageKindEvent   = "event"
	ParentMessageKindRequest = "request"

	ParentMessageRequestStatusOpen      = "offen"
	ParentMessageRequestStatusDone      = "erledigt"
	ParentMessageRequestStatusRejected  = "abgelehnt"
	ParentMessageRequestStatusWithdrawn = "zurueckgezogen"
	ParentMessageRequestCareSchedule    = "care_schedule"
	// ParentMessageRequestPickupChange tags pills for a SINGLE-DAY pickup time
	// change (schedule.care_schedule_change_requests with
	// request_kind='pickup_change'). Distinct from care_schedule, which is the
	// permanent weekly plan: the localized parents portal and the decision
	// push both render from this token, so sharing it would announce a
	// one-day change as a permanent one.
	ParentMessageRequestPickupChange = "pickup_change"
	// ParentMessageRequestMasterData tags notification pills for Stammdaten
	// change requests (users.student_data_change_requests). Unlike
	// care_schedule it was never a chat-request kind — it only ever appears on
	// kind='event' rows.
	ParentMessageRequestMasterData = "master_data"
	// ParentMessageRequestExcusedAbsence tags notification pills for excused
	// absence approval requests (active.excused_absence_requests, #1845). Like
	// master_data it only ever appears on kind='event' rows, never as a chat
	// request.
	ParentMessageRequestExcusedAbsence = "excused_absence"

	// Event types for kind='event' system pills that mirror a change request's
	// lifecycle. ParentMessageEventRequestCreated marks a submitted request (its
	// actionable signal is the Änderungsanfragen queue badge, NOT the unread chat
	// count — see IsCounterpartMessage / counterpartUnread); ParentMessageEventRequestStatus
	// CLOSES it (done / rejected / withdrawn). Kept as constants so the model
	// helpers, the emitter's reconcile gate, and the SQL predicate stay aligned.
	ParentMessageEventRequestCreated = "request_created"
	ParentMessageEventRequestStatus  = "request_status"
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
	// StaffNameVisible freezes, at send time, whether this staff message may show
	// the individual sender's name to guardians (the value of the
	// operations.parent_message_staff_name_visible setting when the row was
	// written). Frozen per-row — exactly like SenderName — so toggling the setting
	// later never retroactively reveals or re-hides already-sent replies: only
	// messages written while the setting was on carry true. Meaningful only for
	// sender_kind='staff'; always false on guardian/system rows. The parent-facing
	// serializer shows the person's name only when this is true, else the neutral
	// "OGS [Schulname]" label.
	StaffNameVisible bool   `bun:"staff_name_visible,notnull,default:false" json:"staff_name_visible"`
	Body             string `bun:"body,notnull" json:"body"`
	// Kind discriminates a plain chat message ('message') from a system event
	// ('event', e.g. a quick-action pill posting a Raumwechsel/Abholung notice
	// into the thread) and a structured change request ('request'). Creators
	// set it explicitly; the column default is 'message'.
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

// IsCounterpartMessage reports whether msg is "from the OTHER party" relative to
// a reader on the given side — the SAME attribution the unread SQL uses
// (database/repositories/users.counterpartUnread). A staff reader's counterpart
// is the guardian side; a guardian reader's is the staff side. A system event
// (request decision / withdrawal) carries sender_kind='system' and records the
// triggering side in EventActorKind, so for those rows the side comes from
// EventActorKind, not SenderKind. Keeping this in lock-step with counterpartUnread
// is what lets MarkReadToNewest bound the read cursor to exactly the set of
// messages the unread count considers — advancing past a later staff/system row
// (not the reader's, but not a counterpart either) would skip an earlier
// counterpart message still committing in a concurrent send/read race.
//
// request_created pills are excluded here in lock-step with counterpartUnread's
// `event_type IS DISTINCT FROM 'request_created'`: a submitted request is
// surfaced on the Änderungsanfragen queue badge, never as an unread chat message,
// so the unread SQL ignores it. Without the SAME exclusion here, MarkReadToNewest
// would advance the read cursor onto such a pill when it is the newest
// guardian-side row — leaping past an earlier, still-committing guardian message
// (lower created_at, later commit) that the unread query would then never surface.
// For a guardian reader it is a no-op (a guardian-side pill is not their
// counterpart anyway), so this only affects the staff side, exactly like the SQL.
func IsCounterpartMessage(msg *ParentMessage, staffReader bool) bool {
	if msg.EventType == ParentMessageEventRequestCreated {
		return false
	}
	side := ParentMessageSenderStaff
	if staffReader {
		side = ParentMessageSenderGuardian
	}
	if msg.SenderKind == ParentMessageSenderSystem {
		return msg.EventActorKind == side
	}
	return msg.SenderKind == side
}

// IsReaderAuthored reports whether msg is one the reader sent THEMSELVES and must
// therefore be excluded from their own unread set — the model mirror of the unread
// SQL's notReaderAuthored predicate
// (database/repositories/users.notReaderAuthored = sender_kind='system' OR
// sender_account_id <> ?). System events are DELIBERATELY never "reader authored":
// they are attributed by triggering SIDE via EventActorKind, not by account, so a
// dual-role (staff+guardian) account's own confirm/reject/withdrawal must still
// count — and advance the cursor — on the opposite portal. Applying a bare
// SenderAccountID == reader check to those rows would strand them as permanently
// unread once viewed. Keeping this in lock-step with notReaderAuthored is what lets
// MarkReadToNewest bound the read cursor to exactly the unread set.
func IsReaderAuthored(msg *ParentMessage, accountID int64) bool {
	return msg.SenderKind != ParentMessageSenderSystem && msg.SenderAccountID == accountID
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
	// FindEventByRef returns the system-event message referencing a specific
	// record (event_type + ref_table + ref_id) in a thread, or nil when none
	// exists. Locates a change request's "request created" pill so a decision
	// can mark it read for the deciding admin.
	FindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*ParentMessage, error)
}
