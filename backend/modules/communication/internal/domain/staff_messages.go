package domain

import "time"

// StaffMessageThread is one continuous conversation between staff accounts of
// a single school (#2598). ParticipantKey identifies the conversation
// independently of its id; for a direct chat it is the two account ids sorted
// ascending and colon-joined, unique per school.
type StaffMessageThread struct {
	ID             int64
	TenantID       int64
	ParticipantKey string
	Kind           string
	LastMessageAt  *time.Time
	// LastMessageID is the second half of the (created_at, id) composite the
	// message list orders by. Every write of LastMessageAt sets it in the same
	// statement, so ties on created_at resolve to the higher id.
	LastMessageID       *int64
	LastMessageBody     string
	LastSenderAccountID *int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// StaffMessage is one append-only entry in a staff conversation. SenderName
// is frozen at send time so history stays readable after a rename.
type StaffMessage struct {
	ID              int64
	TenantID        int64
	ThreadID        int64
	SenderAccountID int64
	SenderName      string
	Body            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// StaffInboxThread is one row of a reader's staff inbox: the conversation, the
// counterpart resolved for that reader, and the reader's unread count.
type StaffInboxThread struct {
	ThreadID             int64
	TenantID             int64
	CounterpartAccountID int64
	CounterpartName      string
	LastMessageAt        *time.Time
	LastMessageBody      string
	LastSenderAccountID  *int64
	UnreadCount          int
}
