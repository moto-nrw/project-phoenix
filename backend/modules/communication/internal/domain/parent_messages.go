package domain

import "time"

// Sender kinds shared by parent threads and messages. A message originates
// from a guardian (parents portal), from staff (tenant portal), or from the
// system (a request decision or withdrawal pill).
const (
	ParentSenderGuardian = "guardian"
	ParentSenderStaff    = "staff"
	ParentSenderSystem   = "system"
)

// ParentMessageEventRequestCreated marks the pill for a submitted change
// request. Its actionable signal is the request queue badge, never the unread
// chat count, so every unread predicate excludes it.
const ParentMessageEventRequestCreated = "request_created"

// ParentMessage is one entry in a child's parent/OGS conversation. The log is
// append-only and SenderName is frozen at send time, so a later rename or
// offboarding never rewrites history.
type ParentMessage struct {
	ClientMessageID string `json:"client_message_id,omitempty"`
	SendReplayed    bool   `json:"-"`
	ID              int64
	TenantID        int64
	ThreadID        int64
	StudentID       int64
	SenderAccountID int64
	SenderKind      string
	SenderName      string
	// StaffNameVisible freezes, per row, whether this staff message may show the
	// individual sender's name to guardians. Toggling the school setting later
	// must never retroactively reveal or re-hide an already-sent reply.
	StaffNameVisible bool
	Body             string
	Kind             string
	EventType        string
	// EventActorKind records which side triggered a system event. Unread
	// attribution reads it instead of inferring a side from SenderAccountID,
	// which is ambiguous for a dual-role staff+guardian account.
	EventActorKind string
	RequestType    string
	RequestStatus  string
	Payload        map[string]any
	RefTable       string
	RefID          *int64
	AppliedAt      *time.Time
	AppliedBy      *int64
	DecisionReason string
	// GroupThreadID names the group thread a guardian's message answers
	// (#4784). The answer lives in this 1:1 conversation only.
	GroupThreadID *int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Thread audiences (#5200). A team conversation is read by every staff member
// who reads the child; a leadership conversation only by accounts holding
// parent_messages:leadership.
const (
	ParentAudienceTeam       = "team"
	ParentAudienceLeadership = "leadership"
)

// ParentMessageThread is one continuous conversation between the OGS and one
// guardian about one child. There is exactly one thread per (school, student,
// guardian, audience); the send path is get-or-create.
type ParentMessageThread struct {
	ID                int64
	TenantID          int64
	StudentID         int64
	GuardianAccountID int64
	// Audience is ParentAudienceTeam or ParentAudienceLeadership.
	Audience string
	// LastMessageAt / LastMessageID / LastSenderKind / LastMessageBody are the
	// denormalized preview the inbox orders and renders by. Every write path
	// advances them together and only forward.
	LastMessageAt   *time.Time
	LastMessageID   *int64
	LastSenderKind  *string
	LastMessageBody string
	// StaffHandledUpTo* is the team-wide boundary for guardian activity already
	// covered by a staff reply. Personal read cursors stay per account.
	StaffHandledUpToAt        *time.Time
	StaffHandledUpToMessageID *int64
	// StaffMarkedUnread* is the team-wide "als ungelesen markiert" state. It is
	// cleared when a staff member opens or answers the conversation.
	StaffMarkedUnreadAt          *time.Time
	StaffMarkedUnreadByAccountID *int64
	// LastStaffMessageNotificationAt is the database-clock claim that collapses
	// a burst of guardian messages into one staff notification.
	LastStaffMessageNotificationAt *time.Time
	CreatedAt                      time.Time
	UpdatedAt                      time.Time
}

// ReadCursorBound is a counterpart message selected in the inbox snapshot.
type ReadCursorBound struct {
	ThreadID  int64
	ReadAt    time.Time
	MessageID int64
}

// MessageableGuardian is one guardian of a child who holds a parent-portal
// account and may therefore be a thread's recipient.
type MessageableGuardian struct {
	AccountID        int64
	Name             string
	RelationshipType string
	IsPrimary        bool
	PortalLocale     string
}
