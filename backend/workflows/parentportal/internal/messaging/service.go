// Package messaging holds the communication side of the guardian portal:
// announcements and their attachments, the request-sharing ledger, the
// parent-OGS conversation and the self-service chat pills (#3227). The child
// flows in workflows/parentportal/care reach the pills and the ledger through
// their ports; workflows/parentportal composes both flow packages into one
// portal (#3420).
package messaging

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// Child is the resolved guardian-child context the messaging flows need.
type Child = care.Child

// ChildResolver is the port to the guardian-child resolution, which checks the
// relationship's parent-portal permission.
type ChildResolver interface {
	ResolvePermittedChild(ctx context.Context, accountID, studentID int64, permission string) (*Child, error)
}

// ConversationCore is the consumer-owned port for Communication's shared
// parent-OGS conversation rules. Both portals mark reads, stamp receipts, and
// fan out over the SAME implementation, so the two chats' unread counts and
// receipts cannot drift; Communication supplies it at the composition seam.
type ConversationCore interface {
	// AppendMessage serializes the thread, persists the message, and advances
	// the thread preview off the row's DB-stamped created_at.
	AppendMessage(ctx context.Context, msg *ParentMessage) error
	// MarkReadToNewest advances the reader's cursor to the newest counterpart
	// message in the snapshot and reports whether it moved.
	MarkReadToNewest(ctx context.Context, tenantID, threadID, accountID int64, staffReader bool, messages []*ParentMessage) (bool, error)
	// DecorateReadReceipts stamps the "OGS hat gelesen" indicator.
	DecorateReadReceipts(ctx context.Context, threadID, otherAccountID int64, messages []*ParentMessage)
	// Broadcast wakes the guardian's tabs and the school's staff after a commit.
	Broadcast(tenantID, guardianAccountID, threadID, studentID int64)
	// BroadcastRead wakes the same fan-out for a read-receipt refresh.
	BroadcastRead(tenantID, guardianAccountID, threadID, studentID int64)
}

// MessagingGate reports the owner's fail-open feature flag for a school.
// It must be bound for conversation reads and writes, independently of the
// optional event emitter. It never replaces guardian-child authorization.
type MessagingGate func(context.Context, int64) bool

// Config is the dependency bundle of the retained messaging portal services.
type Config struct {
	GuardianPolicy PermissionValues
	SharePolicy    RequestSharePolicy
	Fingerprints   ContentFingerprints
	Declarations   DeclarationPolicy
	// Signatures stores the signatures of signed Erklärungen (#4785).
	Signatures SignatureLedger
	Runtime    care.Runtime
	Logger     *slog.Logger
	Now        func() time.Time

	Children ChildResolver

	ChildRepo             care.ChildReads
	EnrollmentRequestRepo care.EnrollmentRequestReads
	Settings              care.SettingsService

	StudentRepo         care.StudentReads
	StudentGuardianRepo care.StudentGuardianReads
	GuardianProfileRepo GuardianProfileReads

	AnnouncementRepo AnnouncementAccess

	MessageThreadRepo     MessageThreadAccess
	MessageRepo           MessageReads
	MessageReadRepo       MessageInboxReads
	Conversations         ConversationCore
	MessagingEnabled      MessagingGate
	GroupThreads          GroupThreads
	Surveys               Surveys
	ParentMessageNotifier care.StaffParentMessageNotifier
	Emitter               care.ChildEvents

	ChangeRequestRepo         care.DataRequestReads
	CareRequestRepo           CareRequestReads
	ExcusedRequestRepo        ExcusedRequestReads
	OfferingChangeRequestRepo OfferingChangeRequestReads
	FamilyProtectionEvents    FamilyProtectionReads
	ParentRequestShares       RequestShareRecords
	ParentRequestEvents       RequestEvents
}

// Service implements the retained messaging portal operations.
type Service struct {
	Config
}

// New wires messaging with its required runtime, guardian policy and fingerprints.
func New(cfg Config) *Service {
	if cfg.Runtime == nil {
		panic("messaging: runtime is required")
	}
	if cfg.SharePolicy == nil {
		panic("messaging: request share policy is required")
	}
	if cfg.GuardianPolicy == nil {
		panic("messaging: guardian policy is required")
	}
	if cfg.Declarations == nil {
		panic("messaging: declaration policy is required")
	}
	if cfg.Fingerprints == nil {
		panic("messaging: fingerprints are required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = calendar.Now
	}
	return &Service{Config: cfg}
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return calendar.Now()
	}
	return s.Now()
}

func (s *Service) todayDate() calendar.Date {
	return calendar.DateFromTime(s.now())
}

func (s *Service) resolveOwnedChild(ctx context.Context, accountID, studentID int64) (*Child, error) {
	return s.resolvePermittedChild(ctx, accountID, studentID, care.GuardianPermissionPortalAccess)
}

func (s *Service) resolvePermittedChild(ctx context.Context, accountID, studentID int64, permission string) (*Child, error) {
	return s.Children.ResolvePermittedChild(ctx, accountID, studentID, permission)
}
