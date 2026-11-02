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
	ErrParentMessageInvalidCountScope       = errors.New("messaging: invalid count scope")
	ErrParentMessageInvalidAudience         = errors.New("messaging: invalid conversation audience")
	// ErrParentMessageLeadershipUnavailable means no new leadership
	// conversation may begin: the school has the choice switched off or no
	// account holds parent_messages:leadership. Existing ones stay open.
	ErrParentMessageLeadershipUnavailable = errors.New("messaging: leadership conversations are not available")
	ErrStaffMessagingDisabled             = errors.New("staffmessaging: internal messaging disabled for this school")
	ErrStaffMessagingNotParticipant       = errors.New("staffmessaging: not a participant of this thread")
	ErrStaffMessageThreadNotFound         = errors.New("staffmessaging: thread not found")
	ErrStaffMessageRecipientNotAvailable  = errors.New("staffmessaging: recipient is not an active member of this school")
	ErrStaffMessageCounterpartUnavailable = errors.New("staffmessaging: the other participant is no longer available")
	ErrStaffMessageSelfConversation       = errors.New("staffmessaging: cannot start a conversation with yourself")
	ErrStaffMessageEmptyBody              = errors.New("staffmessaging: message body must not be empty")
	ErrStaffMessageBodyTooLong            = errors.New("staffmessaging: message body is too long")
	ErrStaffMessagingNoActor              = errors.New("staffmessaging: no authenticated account in context")
	ErrStaffMessageRetentionUnresolved    = errors.New("staffmessaging: retention window could not be resolved")
	ErrStaffMessageGroupCreateForbidden   = errors.New("staffmessaging: this role may not start a group chat")
	ErrStaffMessageGroupManageForbidden   = errors.New("staffmessaging: only the creator and the leadership may manage this group")
	ErrStaffMessageGroupNameInvalid       = errors.New("staffmessaging: group name must not be empty or too long")
	ErrStaffMessageGroupMembersRequired   = errors.New("staffmessaging: a group needs at least one other member")
	ErrStaffMessageGroupTooLarge          = errors.New("staffmessaging: too many members for one group")
)

type staffGroupNameInvalidError struct {
	error
	maxCharacters int
}

func (e staffGroupNameInvalidError) Unwrap() error { return e.error }

// NewStaffGroupNameInvalidError preserves the enforced name limit at the
// public capability boundary.
func NewStaffGroupNameInvalidError(maxCharacters int, cause error) error {
	return staffGroupNameInvalidError{error: cause, maxCharacters: maxCharacters}
}

// StaffGroupNameInvalidMaxCharacters returns the limit from a mapped group
// name error.
func StaffGroupNameInvalidMaxCharacters(err error) (int, bool) {
	var limit staffGroupNameInvalidError
	if !errors.As(err, &limit) {
		return 0, false
	}
	return limit.maxCharacters, true
}

type staffGroupTooLargeError struct {
	error
	maxMembers int
}

func (e staffGroupTooLargeError) Unwrap() error { return e.error }

// NewStaffGroupTooLargeError preserves the enforced member limit at the
// public capability boundary.
func NewStaffGroupTooLargeError(maxMembers int, cause error) error {
	return staffGroupTooLargeError{error: cause, maxMembers: maxMembers}
}

// StaffGroupTooLargeMaxMembers returns the limit from a mapped group-size
// error.
func StaffGroupTooLargeMaxMembers(err error) (int, bool) {
	var limit staffGroupTooLargeError
	if !errors.As(err, &limit) {
		return 0, false
	}
	return limit.maxMembers, true
}

// ParentMessage is the Communication view of one parent/OGS chat entry.
// Payload is raw JSON so the public contract does not leak an ORM model or an
// unbounded map while the HTTP adapter can preserve the existing JSON object.
type ParentMessage struct {
	SenderAccountID int64
	ClientMessageID string `json:"client_message_id,omitempty"`
	SendReplayed    bool   `json:"-"`
	ID              int64
	SenderKind      string
	SenderName      string
	Body            string
	CreatedAt       time.Time
	Kind            string
	EventType       string
	RequestType     string
	RequestStatus   string
	Payload         json.RawMessage
	RefTable        string
	RefID           *int64
	AppliedAt       *time.Time
	AppliedBy       *int64
	DecisionReason  string
	ReadByStaff     bool
	ReadByGuardian  bool
	// GroupThreadID and GroupThreadTitle name the group thread a guardian's
	// message answers (#4784).
	GroupThreadID    *int64
	GroupThreadTitle string
}

type ParentMessageInboxThread struct {
	ThreadID          int64
	StudentID         int64
	StudentName       string
	SchoolClass       string
	GroupName         string
	GuardianAccountID int64
	GuardianName      string
	RelationshipType  string
	// Audience is ParentMessageAudienceTeam or ParentMessageAudienceLeadership.
	// Readers names the school side by kind only; the leadership names come
	// with the conversation itself.
	Audience        string
	Readers         ParentMessageReaders
	LastMessageAt   *time.Time
	LastSenderKind  string
	LastMessageBody string
	UnreadCount     int
}

// ParentMessageCountSetting is the caller's count scope ("all",
// "own_groups" or "none") and whether they have an OGS group today, without
// which "own_groups" counts nothing.
type ParentMessageCountSetting struct {
	Scope        string
	HasOwnGroups bool
}

type ParentMessageThread struct {
	ThreadID         int64
	StudentID        int64
	StudentName      string
	GuardianName     string
	RelationshipType string
	Audience         string
	// Readers says who can read the conversation, resolved when it is read.
	Readers  ParentMessageReaders
	Messages []ParentMessage
}

// ParentMessageReaderPreview says who on the school side would read a
// conversation the caller is about to write in, and whether a new leadership
// conversation may begin.
type ParentMessageReaderPreview struct {
	Audience            string
	School              ParentMessageSchoolReaders
	LeadershipAvailable bool
}

type MessageableGuardian struct {
	AccountID        int64
	Name             string
	RelationshipType string
	IsPrimary        bool
	// Readers says who would read the conversation with this guardian in the
	// requested audience (#5200); LeadershipAvailable whether a new leadership
	// conversation may begin. It is false for a caller without the leadership
	// permission.
	Readers             ParentMessageReaders
	LeadershipAvailable bool
}

// ParentMessagingQuery exposes staff-side reads of parent/OGS conversations.
type ParentMessagingQuery interface {
	ListParentMessageInbox(context.Context, bool) ([]ParentMessageInboxThread, error)
	CountUnreadParentMessages(context.Context) (int, error)
	// ListMessageableGuardians takes the audience of the conversation about
	// to be written ("" is the team) and names its readers per guardian.
	ListMessageableGuardians(ctx context.Context, studentID int64, audience string) ([]MessageableGuardian, error)
	// ListParentMessageReachableStudents returns, ascending, the IDs of the
	// school's children with at least one guardian ListMessageableGuardians
	// would offer (#4362).
	ListParentMessageReachableStudents(context.Context) ([]int64, error)
	ListParentMessageThreadsForStudent(context.Context, int64) ([]ParentMessageInboxThread, error)
	// ParentMessageCountScope returns which conversations the caller's own
	// counter counts (#3673).
	ParentMessageCountScope(context.Context) (ParentMessageCountSetting, error)
}

// ParentMessagingCommand owns staff-side parent/OGS conversation writes.
// GetParentMessageThread advances read state and is therefore a command.
type ParentMessagingCommand interface {
	GetParentMessageThread(context.Context, int64) (*ParentMessageThread, error)
	// StartParentMessageThread and OpenParentMessageThread take the audience
	// last; an empty audience is the team conversation.
	StartParentMessageThread(ctx context.Context, studentID, guardianAccountID int64, body, audience string) (*ParentMessageThread, error)
	OpenParentMessageThread(ctx context.Context, studentID, guardianAccountID int64, audience string) (*ParentMessageThread, error)
	PostParentMessage(context.Context, int64, string, int64) ([]ParentMessage, error)
	// MarkParentMessageThreadUnread marks the conversation unread for the whole
	// team until a staff member opens or answers it.
	MarkParentMessageThreadUnread(context.Context, int64) error
	// MarkAllParentMessagesRead marks every conversation the caller sees as
	// unread as read for the caller's own account and returns the caller's new
	// unread count. Colleagues see no change.
	MarkAllParentMessagesRead(context.Context) (int, error)
	// SetParentMessageCountScope stores which conversations the caller's own
	// counter counts. It changes nothing for colleagues or parents.
	SetParentMessageCountScope(context.Context, string) error
}

type ParentMessagingCapability interface {
	ParentMessagingQuery
	ParentMessagingCommand
}

type StaffMessage struct {
	ClientMessageID string `json:"client_message_id,omitempty"`
	SendReplayed    bool   `json:"-"`
	ID              int64
	SenderAccountID int64
	SenderName      string
	Body            string
	CreatedAt       time.Time
	// Group chats only (#4776): who of the members read the message, and how
	// many members could have.
	ReadByAccountIDs         []int64
	EligibleReaderAccountIDs []int64
	ReaderCount              int
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
	// Kind is "direct" or "group" (#4776).
	Kind           string
	GroupName      string
	MemberCount    int
	LastSenderName string
}

type StaffMessageThread struct {
	ThreadID             int64
	CounterpartAccountID int64
	CounterpartName      string
	CounterpartRoleKind  string
	Messages             []StaffMessage
	// Group chats (#4776).
	Kind      string
	GroupName string
	CanManage bool
	Members   []StaffThreadMember
}

// StaffThreadMember is one person in a group chat.
type StaffThreadMember struct {
	AccountID int64
	Name      string
	RoleKind  string
}

type MessageableStaff struct {
	AccountID int64
	Name      string
	RoleKind  string
}

// ParentMessageCleanupResult reports one school's parent-message retention
// sweep (#3959).
type ParentMessageCleanupResult struct {
	MessagesDeleted int
	ThreadsDeleted  int
	RetentionDays   int
}

// Deleted is the number of rows the sweep deleted: messages and threads.
func (r ParentMessageCleanupResult) Deleted() int { return r.MessagesDeleted + r.ThreadsDeleted }

// ParentMessageCleanup deletes the parent/OGS messages of the school in
// context after the retention window the moto team recorded for it.
type ParentMessageCleanup interface {
	CleanupExpiredParentMessages(context.Context) (ParentMessageCleanupResult, error)
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
	// Group chats (#4776).
	CreateStaffMessageGroup(ctx context.Context, name string, memberAccountIDs []int64) (*StaffMessageThread, error)
	RenameStaffMessageGroup(ctx context.Context, threadID int64, name string) (*StaffMessageThread, error)
	AddStaffMessageGroupMembers(ctx context.Context, threadID int64, accountIDs []int64) (*StaffMessageThread, error)
	// Removing oneself is leaving the group; the result is then nil.
	RemoveStaffMessageGroupMember(ctx context.Context, threadID, accountID int64) (*StaffMessageThread, error)
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
