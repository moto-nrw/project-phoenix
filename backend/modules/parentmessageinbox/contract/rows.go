package contract

import (
	"encoding/json"
	"time"
)

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
	// Audience is "team" or "leadership" (#5200).
	Audience        string
	LastMessageAt   *time.Time
	LastSenderKind  string
	LastMessageBody string
	// The last message's structured fields. The localized parents portal builds
	// its preview from these instead of the German LastMessageBody.
	LastMessageKind        string
	LastEventType          string
	LastRequestType        string
	LastRequestStatus      string
	LastMessagePayload     json.RawMessage
	LastMessageReadByStaff bool
	UnreadCount            int
	ReadBoundAt            *time.Time
	ReadBoundMessageID     *int64
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

// StaffReach is which conversations a staff reader may list and count:
// AllStudents opens every child of the school (nothing without it), and
// Leadership adds the conversations parents addressed to the leadership only
// (#5200). Without Leadership those conversations stay out of every list and
// number.
type StaffReach struct {
	AllStudents bool
	Leadership  bool
}
