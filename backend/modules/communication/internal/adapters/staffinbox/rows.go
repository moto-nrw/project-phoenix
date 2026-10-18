package staffinbox

import "time"

// The projection answers in its own vocabulary (ADR 0058); Communication's
// staff store maps the rows onto its repository contract.

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
