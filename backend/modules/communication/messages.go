package communication

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrParentMessagingForbidden             = errors.New("messaging: forbidden")
	ErrParentMessageThreadNotFound          = errors.New("messaging: thread not found")
	ErrParentMessageEmptyBody               = errors.New("messaging: message body must not be empty")
	ErrParentMessageBodyTooLong             = errors.New("messaging: message body too long")
	ErrParentMessageHandledBoundaryRequired = errors.New("messaging: handled message boundary required")
	ErrParentMessageInvalidGuardian         = errors.New("messaging: recipient is not a guardian of this child")
	ErrParentMessageGuardianAccessRevoked   = errors.New("messaging: recipient no longer has access to this child")
	ErrParentMessagingDisabled              = errors.New("messaging: messaging disabled for this school")
	ErrStaffMessagingDisabled               = errors.New("staffmessaging: internal messaging disabled for this school")
	ErrStaffMessagingNotParticipant         = errors.New("staffmessaging: not a participant of this thread")
	ErrStaffMessageThreadNotFound           = errors.New("staffmessaging: thread not found")
	ErrStaffMessageRecipientNotAvailable    = errors.New("staffmessaging: recipient is not an active member of this school")
	ErrStaffMessageCounterpartUnavailable   = errors.New("staffmessaging: the other participant is no longer available")
	ErrStaffMessageSelfConversation         = errors.New("staffmessaging: cannot start a conversation with yourself")
	ErrStaffMessageEmptyBody                = errors.New("staffmessaging: message body must not be empty")
	ErrStaffMessageBodyTooLong              = errors.New("staffmessaging: message body is too long")
	ErrStaffMessagingNoActor                = errors.New("staffmessaging: no authenticated account in context")
	ErrStaffMessageRetentionUnresolved      = errors.New("staffmessaging: retention window could not be resolved")
)

// ParentMessage is the Communication view of one parent/OGS chat entry.
// Payload is raw JSON so the public contract does not leak an ORM model or an
// unbounded map while the HTTP adapter can preserve the existing JSON object.
type ParentMessage struct {
	ID             int64
	SenderKind     string
	SenderName     string
	Body           string
	CreatedAt      time.Time
	Kind           string
	EventType      string
	RequestType    string
	RequestStatus  string
	Payload        json.RawMessage
	RefTable       string
	RefID          *int64
	AppliedAt      *time.Time
	AppliedBy      *int64
	DecisionReason string
	ReadByStaff    bool
	ReadByGuardian bool
}

type ParentMessageInboxThread struct {
	ThreadID         int64
	StudentID        int64
	StudentName      string
	SchoolClass      string
	GroupName        string
	GuardianName     string
	RelationshipType string
	LastMessageAt    *time.Time
	LastSenderKind   string
	LastMessageBody  string
	UnreadCount      int
}

type ParentMessageThread struct {
	ThreadID         int64
	StudentID        int64
	StudentName      string
	GuardianName     string
	RelationshipType string
	Messages         []ParentMessage
}

type MessageableGuardian struct {
	AccountID        int64
	Name             string
	RelationshipType string
	IsPrimary        bool
}

// ParentMessagingQuery exposes staff-side reads of parent/OGS conversations.
type ParentMessagingQuery interface {
	ListParentMessageInbox(context.Context, bool) ([]ParentMessageInboxThread, error)
	CountUnreadParentMessages(context.Context) (int, error)
	ListMessageableGuardians(context.Context, int64) ([]MessageableGuardian, error)
	ListParentMessageThreadsForStudent(context.Context, int64) ([]ParentMessageInboxThread, error)
}

// ParentMessagingCommand owns staff-side parent/OGS conversation writes.
// GetParentMessageThread advances read state and is therefore a command.
type ParentMessagingCommand interface {
	GetParentMessageThread(context.Context, int64) (*ParentMessageThread, error)
	StartParentMessageThread(context.Context, int64, int64, string) (*ParentMessageThread, error)
	OpenParentMessageThread(context.Context, int64, int64) (*ParentMessageThread, error)
	PostParentMessage(context.Context, int64, string, int64) ([]ParentMessage, error)
}

type ParentMessagingCapability interface {
	ParentMessagingQuery
	ParentMessagingCommand
}

type StaffMessage struct {
	ID              int64
	SenderAccountID int64
	SenderName      string
	Body            string
	CreatedAt       time.Time
}

type StaffMessageInboxThread struct {
	ThreadID             int64
	CounterpartAccountID int64
	CounterpartName      string
	CounterpartRoleKind  string
	LastMessageAt        *time.Time
	LastMessageBody      string
	LastSenderAccountID  *int64
	UnreadCount          int
}

type StaffMessageThread struct {
	ThreadID             int64
	CounterpartAccountID int64
	CounterpartName      string
	CounterpartRoleKind  string
	Messages             []StaffMessage
}

type MessageableStaff struct {
	AccountID int64
	Name      string
	RoleKind  string
}

type StaffMessageCleanupResult struct {
	MessagesDeleted int
	ThreadsDeleted  int
	RetentionDays   int
}

type StaffMessageCleanup interface {
	CleanupExpiredStaffMessages(context.Context) (StaffMessageCleanupResult, error)
}

// StaffMessagingQuery exposes team-chat reads without append or cleanup
// authority. GetStaffMessageThread is a command because it advances read state.
type StaffMessagingQuery interface {
	ListStaffMessageInbox(context.Context, bool) ([]StaffMessageInboxThread, error)
	CountUnreadStaffMessages(context.Context) (int, error)
	ListMessageableStaff(context.Context) ([]MessageableStaff, error)
}

// StaffMessagingCommand owns thread creation, read-cursor writes, and appends.
// Each multi-table operation joins the ambient UnitOfWork.
type StaffMessagingCommand interface {
	OpenStaffMessageThread(context.Context, int64) (*StaffMessageThread, error)
	GetStaffMessageThread(context.Context, int64) (*StaffMessageThread, error)
	PostStaffMessage(context.Context, int64, string) (*StaffMessage, error)
}

type StaffMessagingCapability interface {
	StaffMessagingQuery
	StaffMessagingCommand
}

// StaffMessagingRuntime is the full application composition contract. HTTP
// adapters receive StaffMessagingCapability; the worker receives only
// StaffMessageCleanup.
type StaffMessagingRuntime interface {
	StaffMessagingCapability
	StaffMessageCleanup
}
