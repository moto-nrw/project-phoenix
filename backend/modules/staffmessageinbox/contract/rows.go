package contract

import "time"

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
	// Kind is "direct" or "group" (#4776). A group row names the group and
	// its size instead of a counterpart.
	Kind           string
	GroupName      string
	MemberCount    int
	LastSenderName string
}
