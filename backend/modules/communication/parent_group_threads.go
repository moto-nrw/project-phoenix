package communication

import (
	"context"
	"errors"
	"time"
)

// Group thread targets (#4784).
const (
	ParentGroupTargetClass         = "class"
	ParentGroupTargetGroup         = "group"
	ParentGroupTargetActivityGroup = "activity_group"
)

var (
	ErrParentGroupThreadNotFound     = errors.New("messaging: group thread not found")
	ErrParentGroupThreadClosed       = errors.New("messaging: group thread is closed")
	ErrParentGroupThreadInvalid      = errors.New("messaging: invalid group thread")
	ErrParentGroupThreadNoRecipients = errors.New("messaging: the group thread reaches no parents")
)

// ParentGroupTarget is the group, class or AG a group thread writes to.
// Label is the name the OGS and the parents read ("Klasse 2a").
type ParentGroupTarget struct {
	Type    string
	RefID   *int64
	RefText *string
	Label   string
}

// ParentGroupThread is a thread the OGS writes to every family of a group,
// class or AG. Families answer only the OGS, in their child's conversation.
type ParentGroupThread struct {
	ID            int64
	TenantID      int64
	Target        ParentGroupTarget
	Title         string
	ClosedAt      *time.Time
	LastMessageAt *time.Time
	CreatedAt     time.Time
}

// ParentGroupMessage is one OGS message of a group thread.
type ParentGroupMessage struct {
	ID         int64
	SenderName string
	Body       string
	CreatedAt  time.Time
}

// ParentGroupChildRead is one child a group thread reached and whether one
// of its guardians read the newest message that reached the child.
type ParentGroupChildRead struct {
	StudentID int64
	Read      bool
}

// ParentGroupRecipient is one child and guardian a group message reached.
type ParentGroupRecipient struct {
	StudentID         int64
	GuardianAccountID int64
}

// ParentGroupThreadSummary is a staff list row with the family read count.
type ParentGroupThreadSummary struct {
	Thread          ParentGroupThread
	LastMessageBody string
	MessageCount    int
	FamilyCount     int
	ReadCount       int
}

// ParentGroupChild is one child of a group thread for the OGS: in the
// audience preview whether a parent can be reached (ADR 0079), in the read
// status whether a parent read the newest message it got.
type ParentGroupChild struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
	Reachable   bool
	Read        bool
}

// ParentGroupThreadDetail is the OGS view of one group thread.
type ParentGroupThreadDetail struct {
	Thread      ParentGroupThread
	Messages    []ParentGroupMessage
	Children    []ParentGroupChild
	FamilyCount int
	ReadCount   int
}

// ParentGroupThreadReadCounts is the family-level read status of a group
// thread. A family is one guardian account that the thread reached.
type ParentGroupThreadReadCounts struct {
	FamilyCount int
	ReadCount   int
}

// ParentGroupThreadInput starts a group thread with its first message.
type ParentGroupThreadInput struct {
	Target ParentGroupTarget
	Title  string
	Body   string
}

// ParentGroupThreadCapability is the OGS side of the group threads. Every
// method runs in the caller's school transaction.
type ParentGroupThreadCapability interface {
	ListParentGroupThreads(context.Context) ([]ParentGroupThreadSummary, error)
	PreviewParentGroupAudience(context.Context, ParentGroupTarget) ([]ParentGroupChild, error)
	CreateParentGroupThread(context.Context, ParentGroupThreadInput) (*ParentGroupThreadDetail, error)
	GetParentGroupThread(context.Context, int64) (*ParentGroupThreadDetail, error)
	PostParentGroupMessage(context.Context, int64, string) (*ParentGroupThreadDetail, error)
	CloseParentGroupThread(context.Context, int64) (*ParentGroupThreadDetail, error)
}

// ParentGroupAudienceReads resolves a target to its children and their
// reachable guardians, and names children by id. The root binds it to the
// parent audience projection.
type ParentGroupAudienceReads interface {
	GroupAudience(ctx context.Context, schoolID int64, target ParentGroupTarget) ([]ParentGroupAudienceChild, error)
	GroupStudents(ctx context.Context, schoolID int64, studentIDs []int64) ([]ParentGroupChild, error)
}

// ParentGroupAudienceChild is one reached child with the guardian accounts
// that may read the thread; empty when no parent is reachable.
type ParentGroupAudienceChild struct {
	StudentID          int64
	FirstName          string
	LastName           string
	SchoolClass        string
	GuardianAccountIDs []int64
}

// PortalGuardianGroupThread is one group thread in a guardian's list. It
// carries only what reached this guardian.
type PortalGuardianGroupThread struct {
	Thread          ParentGroupThread
	LastMessageBody string
	LastMessageAt   *time.Time
	UnreadCount     int
	StudentIDs      []int64
}

// PortalGuardianGroupThreadView is one group thread as a guardian reads it:
// the messages that reached them and the children they got them for.
type PortalGuardianGroupThreadView struct {
	Thread     ParentGroupThread
	Messages   []ParentGroupMessage
	StudentIDs []int64
}

// PortalGroupThreads is the guardian side of the group threads, bound into
// the parent portal workflow. studentIDs are the children the workflow
// resolved under the guardian's live parent_portal.access; a message reaches
// a guardian only through a recipient row for one of them.
type PortalGroupThreads interface {
	PortalListGroupThreads(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) ([]PortalGuardianGroupThread, error)
	PortalCountUnreadGroupThreads(ctx context.Context, accountID int64, studentIDs, tenantIDs []int64) (int, error)
	PortalReadGroupThread(ctx context.Context, threadID, accountID int64, studentIDs []int64) (*PortalGuardianGroupThreadView, bool, error)
	PortalGroupReplyTarget(ctx context.Context, threadID, accountID, studentID int64) (*ParentGroupThread, error)
	PortalGroupThreadTitles(ctx context.Context, threadIDs []int64) (map[int64]string, error)
}
