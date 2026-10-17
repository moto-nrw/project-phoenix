package users

import (
	"context"
	"time"
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
	// ParentMessageRequestSickAbsence distinguishes a gated Krankmeldung from an
	// excused absence in localized parent timeline events and decision pushes.
	ParentMessageRequestSickAbsence = "sick_absence"

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
	Model `bun:"schema:users,table:parent_messages"`
	TenantModel
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
