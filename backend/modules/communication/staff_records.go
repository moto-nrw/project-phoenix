package communication

import (
	"context"
	"time"
)

// StaffMessageRecord and StaffThreadRecord retain all stored conversation data,
// separate from the HTTP views. Tenant IDs never come from a write selector.
type StaffMessageRecord struct {
	ID              int64
	TenantID        int64
	ThreadID        int64
	SenderAccountID int64
	SenderName      string
	Body            string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
type StaffThreadRecord struct {
	ID             int64
	TenantID       int64
	ParticipantKey string
	Kind           string
	// Name is the display name of a group chat (#4776); empty for a direct chat.
	Name string
	// CreatedByAccountID is who started a group chat; nil for a direct chat
	// and after the creator's account was deleted.
	CreatedByAccountID  *int64
	LastMessageAt       *time.Time
	LastMessageID       *int64
	LastMessageBody     string
	LastSenderAccountID *int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
type StaffInboxRecord struct {
	ThreadID             int64
	TenantID             int64
	CounterpartAccountID int64
	CounterpartName      string
	CounterpartRoleKind  string
	LastMessageAt        *time.Time
	LastMessageBody      string
	LastSenderAccountID  *int64
	UnreadCount          int
	// Kind is "direct" or "group" (#4776). A group row carries GroupName and
	// MemberCount instead of a counterpart.
	Kind           string
	GroupName      string
	MemberCount    int
	LastSenderName string
}

// StaffParticipantRecord is one member of a conversation with the instant
// their visible history starts (nil: the whole history) and their read cursor.
type StaffParticipantRecord struct {
	AccountID         int64
	HistoryFrom       *time.Time
	LastReadAt        *time.Time
	LastReadMessageID int64
}
type AddressableStaffRecord struct {
	AccountID int64
	Name      string
	RoleKind  string
}

// StaffThreadRecords is the exact existing participant-scoped thread capability.
type StaffThreadRecords interface {
	GetOrCreateStaffThread(context.Context, int64, int64) (*StaffThreadRecord, error)
	FindStaffThread(context.Context, int64) (*StaffThreadRecord, error)
	LockStaffThreadForAppend(context.Context, int64) error
	AdvanceStaffThreadPreview(context.Context, int64, time.Time, int64, int64, string) error
	StaffParticipantAccountIDs(context.Context, int64) ([]int64, error)
	HasStaffParticipant(context.Context, int64, int64) (bool, error)
	DeleteEmptyStaffThreads(context.Context, time.Time) (int64, error)
}

// StaffGroupRecords is the group-chat membership capability (#4776). It is a
// facet of its own so the thread capability stays narrow; the thread store
// implements both.
type StaffGroupRecords interface {
	CreateStaffGroupThread(ctx context.Context, name string, creatorAccountID int64, memberAccountIDs []int64) (*StaffThreadRecord, error)
	RenameStaffGroupThread(ctx context.Context, threadID int64, name string) error
	AddStaffGroupParticipants(ctx context.Context, threadID int64, accountIDs []int64) error
	RemoveStaffParticipant(ctx context.Context, threadID, accountID int64) error
	StaffParticipantStates(ctx context.Context, threadID int64) ([]*StaffParticipantRecord, error)
	DeleteStaffThread(ctx context.Context, threadID int64) error
}

// StaffConversationRecords is the thread store with both facets.
type StaffConversationRecords interface {
	StaffThreadRecords
	StaffGroupRecords
}
type StaffMessageRecords interface {
	CreateStaffMessage(context.Context, *StaffMessageRecord) error
	ListStaffMessages(context.Context, int64) ([]*StaffMessageRecord, error)
	DeleteStaffMessagesBefore(context.Context, time.Time) (int64, error)
}
type StaffReadRecords interface {
	MarkStaffReadUpTo(context.Context, int64, int64, time.Time, int64) error
	StaffUnreadCount(context.Context, int64) (int, error)
	ListStaffInbox(context.Context, int64, bool) ([]*StaffInboxRecord, error)
	ListAddressableStaff(context.Context, int64) ([]*AddressableStaffRecord, error)
	IsAddressableStaff(context.Context, int64) (bool, error)
	StaffRoleKindsForAccounts(context.Context, []int64) (map[int64]string, error)
}

// StaffSenderName is the precise name projection the conversation freezes.
type StaffSenderName struct {
	FirstName string
	LastName  string
}
type StaffSenderNames interface {
	FindStaffSenderName(context.Context, int64) (*StaffSenderName, error)
}
