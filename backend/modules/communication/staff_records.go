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
	ID                  int64
	TenantID            int64
	ParticipantKey      string
	Kind                string
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
