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

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	notificationsSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
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
	AppendMessage(ctx context.Context, msg *usersModels.ParentMessage) error
	// MarkReadToNewest advances the reader's cursor to the newest counterpart
	// message in the snapshot and reports whether it moved.
	MarkReadToNewest(ctx context.Context, tenantID, threadID, accountID int64, staffReader bool, messages []*usersModels.ParentMessage) (bool, error)
	// DecorateReadReceipts stamps the "OGS hat gelesen" indicator.
	DecorateReadReceipts(ctx context.Context, threadID, otherAccountID int64, messages []*usersModels.ParentMessage)
	// Broadcast wakes the guardian's tabs and the school's staff after a commit.
	Broadcast(tenantID, guardianAccountID, threadID, studentID int64)
	// BroadcastRead wakes the same fan-out for a read-receipt refresh.
	BroadcastRead(tenantID, guardianAccountID, threadID, studentID int64)
}

// Config is the dependency bundle of the retained messaging portal services.
type Config struct {
	Logger *slog.Logger
	Now    func() time.Time

	Children ChildResolver

	ChildRepo             care.ChildReads
	EnrollmentRequestRepo care.EnrollmentRequestReads
	Settings              configService.SettingsService

	StudentRepo         care.StudentReads
	StudentGuardianRepo care.StudentGuardianReads
	GuardianProfileRepo GuardianProfileReads

	AnnouncementRepo usersModels.ParentAnnouncementRepository

	MessageThreadRepo     usersModels.ParentMessageThreadRepository
	MessageRepo           usersModels.ParentMessageRepository
	MessageReadRepo       usersModels.ParentMessageReadRepository
	Conversations         ConversationCore
	ParentMessageNotifier notificationsSvc.StaffParentMessageNotifier
	Emitter               *parentmessaging.Emitter

	ChangeRequestRepo         care.DataRequestReads
	CareRequestRepo           CareRequestReads
	ExcusedRequestRepo        ExcusedRequestReads
	OfferingChangeRequestRepo OfferingChangeRequestReads
	FamilyProtectionEvents    FamilyProtectionReads
	ParentRequestShares       usersModels.ParentRequestShareEventRepository
	ParentRequestEvents       usersModels.ParentRequestEventRepository
}

// Service implements the retained messaging portal operations.
type Service struct {
	Config
}

// New wires the retained messaging portal services.
func New(cfg Config) *Service {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = timezone.Now
	}
	return &Service{Config: cfg}
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return timezone.Now()
	}
	return s.Now()
}

func (s *Service) todayDate() timezone.Date {
	return timezone.DateFromTime(s.now())
}

func (s *Service) resolveOwnedChild(ctx context.Context, accountID, studentID int64) (*Child, error) {
	return s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
}

func (s *Service) resolvePermittedChild(ctx context.Context, accountID, studentID int64, permission string) (*Child, error) {
	return s.Children.ResolvePermittedChild(ctx, accountID, studentID, permission)
}
