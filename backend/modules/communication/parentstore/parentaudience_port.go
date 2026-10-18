package parentstore

import (
	"context"

	parentaudience "github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
)

type AnnouncementAudienceReads interface {
	DeclarationAudienceReads
	DeliveryAudienceReads
	FeedAudienceReads
	PollAudienceReads
}

type DeclarationAudienceReads interface {
	DeclarationChildren(ctx context.Context, schoolID, announcementID int64, history []parentaudience.DeclarationHistoryLink) ([]*parentaudience.DeclarationChild, error)
	DeclarationSigners(ctx context.Context, schoolID int64, studentIDs []int64) ([]*parentaudience.DeclarationSigner, error)
	DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*parentaudience.DeclarationSigner, error)
	DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64, history []parentaudience.DeclarationHistoryLink) ([]*parentaudience.DeclarationChild, error)
	HoldDeclarationSigner(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (*parentaudience.DeclarationSignerContext, error)
	HoldHistoricalDeclarationSigner(ctx context.Context, schoolID, accountID, studentID int64) (*parentaudience.DeclarationSignerContext, error)
	HistoricalDeclarationAccess(ctx context.Context, schoolID, accountID int64, history []parentaudience.DeclarationHistoryLink) (bool, error)
	HistoricalDeclarationFeed(ctx context.Context, accountID int64, scope parentaudience.ParentAnnouncementFeedScope, history []parentaudience.DeclarationHistoryLink) ([]*parentaudience.ParentAnnouncementFeedItem, error)
}

type DeliveryAudienceReads interface {
	CountAudience(ctx context.Context, schoolID, announcementID int64) (int, error)
	AccountMatchesAnnouncement(ctx context.Context, schoolID, announcementID, accountID int64) (bool, error)
	ResolveAudienceEmails(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementRecipient, error)
	LetterChildStatuses(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementLetterChildStatus, error)
	ResolveDeliveryRecipients(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementDeliveryRecipient, error)
	AudienceRecipients(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementRecipientStatus, error)
	CountReachableGuardiansForStudents(ctx context.Context, schoolID int64, studentIDs []int64) (int, error)
}

type FeedAudienceReads interface {
	ListFeedForAccount(ctx context.Context, accountID int64, scope parentaudience.ParentAnnouncementFeedScope) ([]*parentaudience.ParentAnnouncementFeedItem, error)
	CountOutstandingForAccount(ctx context.Context, accountID int64, scope parentaudience.ParentAnnouncementFeedScope) (int, error)
	Stats(ctx context.Context, schoolID, announcementID int64) (*parentaudience.ParentAnnouncementStats, error)
}

type PollAudienceReads interface {
	AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*parentaudience.ParentAnnouncementPollChild, error)
	AccountMayAnswerForStudent(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error)
	HoldAnswerPermission(ctx context.Context, schoolID, announcementID, accountID, studentID int64) (bool, error)
	PollResults(ctx context.Context, schoolID, announcementID int64) (*parentaudience.ParentAnnouncementPollResults, error)
	PollChildren(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementPollChildStatus, error)
	UnansweredReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementReminderRecipient, error)
	UnacknowledgedReminderRecipients(ctx context.Context, schoolID, announcementID int64) ([]*parentaudience.ParentAnnouncementReminderRecipient, error)
}
