package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type AnnouncementAudienceReads interface {
	DeclarationAudienceReads
	DeliveryAudienceReads
	FeedAudienceReads
	PollAudienceReads
}

type DeclarationAudienceReads interface {
	DeclarationChildren(ctx context.Context, schoolID, announcementID int64, history []*communication.PortalDeclarationSubmission) ([]*communication.PortalDeclarationChild, error)
	DeclarationSigners(ctx context.Context, schoolID int64, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error)
	DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error)
	DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64, history []*communication.PortalDeclarationSubmission) ([]*communication.PortalDeclarationChild, error)
	HoldDeclarationSigner(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (*communication.PortalDeclarationSignerContext, error)
	HoldHistoricalDeclarationSigner(ctx context.Context, schoolID, accountID, studentID int64) (*communication.PortalDeclarationSignerContext, error)
	HistoricalDeclarationAccess(ctx context.Context, schoolID, accountID int64, history []*communication.PortalDeclarationSubmission) (bool, error)
	HistoricalDeclarationFeed(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope, history []*communication.PortalDeclarationSubmission) ([]*communication.PortalAnnouncementFeedItem, error)
}

type DeliveryAudienceReads interface {
	CountAudience(ctx context.Context, schoolID, announcementID int64) (int, error)
	AccountMatchesAnnouncement(ctx context.Context, schoolID, announcementID, accountID int64) (bool, error)
	ResolveAudienceEmails(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementRecipient, error)
	LetterChildStatuses(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementLetterChildStatus, error)
	ResolveDeliveryRecipients(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementDeliveryRecipient, error)
	AudienceRecipients(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementRecipientStatus, error)
	CountReachableGuardiansForStudents(ctx context.Context, schoolID int64, studentIDs []int64) (int, error)
}

type FeedAudienceReads interface {
	ListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error)
	CountOutstandingForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) (int, error)
	Stats(ctx context.Context, schoolID, announcementID int64) (*communication.PortalAnnouncementStats, error)
}

type PollAudienceReads interface {
	AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalAnnouncementPollChild, error)
	AccountMayAnswerForStudent(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error)
	HoldAnswerPermission(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error)
	PollResults(ctx context.Context, schoolID, announcementID int64) (*communication.PortalAnnouncementPollResults, error)
	PollChildren(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementPollChildStatus, error)
	UnansweredReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error)
	UnacknowledgedReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error)
}
