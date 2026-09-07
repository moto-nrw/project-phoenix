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
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ParentMessageThread is the single continuous conversation between the OGS
// team and one guardian about one child. There is exactly one thread per
// (school, student, guardian); the send path is get-or-create.
type ParentMessageThread struct {
	ID                int64
	TenantID          int64
	StudentID         int64
	GuardianAccountID int64
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
	// LastStaffMessageNotificationAt is the database-clock claim that collapses
	// a burst of guardian messages into one staff notification.
	LastStaffMessageNotificationAt *time.Time
	CreatedAt                      time.Time
	UpdatedAt                      time.Time
}

// ParentInboxThread is the list projection of one conversation: the child, the
// guardian, a preview of the latest message, and the viewer's unread count.
// Both the staff inbox and the guardian thread list read it.
type ParentInboxThread struct {
	ThreadID int64
	// TenantID is the thread's owning school. The parent portal maps it to the
	// per-school messaging flag, so a school that switched messaging off shows
	// no unread pill.
	TenantID          int64
	StudentID         int64
	StudentName       string
	SchoolClass       string
	GroupID           *int64
	GuardianAccountID int64
	GuardianName      string
	RelationshipType  string
	LastMessageAt     *time.Time
	LastSenderKind    string
	LastMessageBody   string
	// The last message's structured fields. The localized parents portal builds
	// its preview from these instead of the German LastMessageBody.
	LastMessageKind        string
	LastEventType          string
	LastRequestType        string
	LastRequestStatus      string
	LastMessagePayload     map[string]any
	LastMessageReadByStaff bool
	UnreadCount            int
}

// ParentReadCursor is a reader's position in a thread: the read instant and its
// message-id tie-breaker. Two messages can share a created_at, so every unread
// and receipt comparison uses the composite, never the timestamp alone.
type ParentReadCursor struct {
	LastReadAt        time.Time
	LastReadMessageID int64
}

// ParentThreadHeader is the chat window's header: the child and guardian
// display names plus the relationship. It deliberately omits the inbox
// projection's unread count.
type ParentThreadHeader struct {
	StudentName      string
	GuardianName     string
	RelationshipType string
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
