package legacy

import (
	"context"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/messaging"
)

// The communication side of the guardian portal lives in
// workflows/parentportal/messaging (#3227). These aliases keep the
// parent-service vocabulary that api/parent names unchanged.
type (
	ConversationCore        = messaging.ConversationCore
	MessageThreadView       = messaging.MessageThreadView
	RequestSharingRecipient = messaging.RequestSharingRecipient
	RequestSharingState     = messaging.RequestSharingState
	RequestSharingService   = messaging.RequestSharingService
	ParentRequestEventView  = messaging.ParentRequestEventView

	requestShareVisibility = messaging.RequestShareVisibility
)

const (
	RequestShareMasterData   = messaging.RequestShareMasterData
	RequestShareCareSchedule = messaging.RequestShareCareSchedule
	RequestSharePickupChange = messaging.RequestSharePickupChange
	RequestShareOffering     = messaging.RequestShareOffering
	RequestShareExcused      = messaging.RequestShareExcused
)

var (
	ErrAnnouncementNotFound       = messaging.ErrAnnouncementNotFound
	ErrAnnouncementAckNotRequired = messaging.ErrAnnouncementAckNotRequired
	ErrAnnouncementStale          = messaging.ErrAnnouncementStale
	ErrAnnouncementNotAPoll       = messaging.ErrAnnouncementNotAPoll
	ErrPollClosed                 = messaging.ErrPollClosed
	ErrInvalidPollResponse        = messaging.ErrInvalidPollResponse
	ErrChildNotAnswerable         = messaging.ErrChildNotAnswerable
	ErrRequestSharingInvalid      = messaging.ErrRequestSharingInvalid
	ErrRequestSharingForbidden    = messaging.ErrRequestSharingForbidden
	ErrRequestSharingNotFound     = messaging.ErrRequestSharingNotFound
)

// ResolvePermittedChild implements messaging.ChildResolver with the care
// package's guardian-child resolution.
func (s *service) ResolvePermittedChild(ctx context.Context, accountID, studentID int64, permission string) (*messaging.Child, error) {
	return s.care.ResolvePermittedChild(ctx, accountID, studentID, permission)
}

// ShareRequestInTx implements care.RequestSharer.
func (s *service) ShareRequestInTx(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64, recipientProfileIDs []int64,
) error {
	return s.messaging.ShareRequestInTx(ctx, accountID, studentID, requestType, requestID, recipientProfileIDs)
}

// LoadRequestShareVisibility implements care.RequestSharer.
func (s *service) LoadRequestShareVisibility(ctx context.Context, studentID int64) (care.RequestShareVisibility, error) {
	visibility, err := s.messaging.LoadRequestShareVisibility(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return visibility, nil
}

func (s *service) loadRequestShareVisibility(ctx context.Context, studentID int64) (*requestShareVisibility, error) {
	return s.messaging.LoadRequestShareVisibility(ctx, studentID)
}

// SharedRecipientAccountIDs names the guardians one request was shared with.
func (s *service) SharedRecipientAccountIDs(ctx context.Context, studentID int64, requestType string, requestID int64) ([]int64, error) {
	return s.messaging.SharedRecipientAccountIDs(ctx, studentID, requestType, requestID)
}

func (s *service) GuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64) (int64, error) {
	return s.messaging.GuardianAnnouncementTenant(ctx, accountID, announcementID)
}

func (s *service) ListAnnouncements(ctx context.Context, accountID int64) ([]*usersModels.AnnouncementFeedItem, error) {
	return s.messaging.ListAnnouncements(ctx, accountID)
}

func (s *service) UnreadAnnouncementCount(ctx context.Context, accountID int64) (int, error) {
	return s.messaging.UnreadAnnouncementCount(ctx, accountID)
}

func (s *service) MarkAnnouncementRead(ctx context.Context, accountID, announcementID int64, expectedPublishedAt time.Time) error {
	return s.messaging.MarkAnnouncementRead(ctx, accountID, announcementID, expectedPublishedAt)
}

func (s *service) AcknowledgeAnnouncement(ctx context.Context, accountID, announcementID int64, expectedPublishedAt time.Time) error {
	return s.messaging.AcknowledgeAnnouncement(ctx, accountID, announcementID, expectedPublishedAt)
}

func (s *service) RespondToAnnouncement(ctx context.Context, accountID, announcementID, studentID int64, optionIDs []int64, expectedPublishedAt time.Time) error {
	return s.messaging.RespondToAnnouncement(ctx, accountID, announcementID, studentID, optionIDs, expectedPublishedAt)
}

func (s *service) ListMessageThreads(ctx context.Context, accountID int64) ([]*usersModels.InboxThread, error) {
	return s.messaging.ListMessageThreads(ctx, accountID)
}

func (s *service) ListChildThreads(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error) {
	return s.messaging.ListChildThreads(ctx, accountID, studentID)
}

func (s *service) UnreadMessageCount(ctx context.Context, accountID int64) (int, error) {
	return s.messaging.UnreadMessageCount(ctx, accountID)
}

func (s *service) GetChildConversation(ctx context.Context, accountID, studentID int64) (*MessageThreadView, error) {
	return s.messaging.GetChildConversation(ctx, accountID, studentID)
}

func (s *service) PostChildMessage(ctx context.Context, accountID, studentID int64, body string) (*MessageThreadView, error) {
	return s.messaging.PostChildMessage(ctx, accountID, studentID, body)
}

func (s *service) GetRequestSharingOptions(ctx context.Context, accountID, studentID int64) (*RequestSharingState, error) {
	return s.messaging.GetRequestSharingOptions(ctx, accountID, studentID)
}

func (s *service) GetRequestSharing(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64,
) (*RequestSharingState, error) {
	return s.messaging.GetRequestSharing(ctx, accountID, studentID, requestType, requestID)
}

func (s *service) SetRequestSharing(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64,
	recipientGuardianProfileIDs []int64,
) (*RequestSharingState, error) {
	return s.messaging.SetRequestSharing(ctx, accountID, studentID, requestType, requestID, recipientGuardianProfileIDs)
}

func (s *service) ListRequestEvents(
	ctx context.Context, accountID, studentID int64, requestType string, requestID int64,
) ([]ParentRequestEventView, error) {
	return s.messaging.ListRequestEvents(ctx, accountID, studentID, requestType, requestID)
}
