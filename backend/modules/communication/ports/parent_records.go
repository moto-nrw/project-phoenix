package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type ParentAnnouncementRepository interface {
	communication.StaffAnnouncementRecords
	communication.PortalAnnouncementAccess
	ParentDeclarationRepository
	AnnouncementDrafts
	AnnouncementReminders
	AnnouncementPolls
	AnnouncementAudience
	AnnouncementFeed
}
type AnnouncementDrafts interface {
	Create(ctx context.Context, a *communication.PortalParentAnnouncement) error
	Update(ctx context.Context, a *communication.PortalParentAnnouncement) error
	FindByID(ctx context.Context, id int64) (*communication.PortalParentAnnouncement, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*communication.PortalParentAnnouncement, error)
	Delete(ctx context.Context, id int64) error
	ListForTenant(ctx context.Context, includeInactive bool) ([]*communication.PortalParentAnnouncement, error)
	SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error
	ClearEngagement(ctx context.Context, announcementID int64) error
	PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error)
}
type AnnouncementReminders interface {
	SetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error)
	ListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*communication.PortalParentAnnouncement, error)
	ClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error)
	ReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error)
}
type AnnouncementPolls interface {
	ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*communication.PortalParentAnnouncementTarget) error
	ListTargets(ctx context.Context, announcementID int64) ([]*communication.PortalParentAnnouncementTarget, error)
	ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*communication.PortalParentAnnouncementOption) error
	ListOptions(ctx context.Context, announcementID int64) ([]*communication.PortalParentAnnouncementOption, error)
	ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*communication.PortalParentAnnouncementOption, error)
	AnswerableChildren(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalAnnouncementPollChild, error)
	AccountMayAnswerForStudent(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (bool, error)
	SetResponse(ctx context.Context, tenantID, announcementID, studentID, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error)
	PollResults(ctx context.Context, tenantID, announcementID int64) (*communication.PortalAnnouncementPollResults, error)
	PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollChildStatus, error)
	UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error)
}
type AnnouncementAudience interface {
	CountAudience(ctx context.Context, tenantID, announcementID int64) (int, error)
	AccountMatchesAnnouncement(ctx context.Context, tenantID, announcementID, accountID int64) (bool, error)
	ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementRecipient, error)
	UnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementPollReminderRecipient, error)
	LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementLetterChildStatus, error)
	ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementDeliveryRecipient, error)
	SchoolName(ctx context.Context, tenantID int64) (string, error)
	AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalAnnouncementRecipientStatus, error)
	CountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error)
}
type AnnouncementFeed interface {
	ListFeedForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) ([]*communication.PortalAnnouncementFeedItem, error)
	CountUnreadForAccount(ctx context.Context, accountID int64, scope communication.PortalAnnouncementFeedScope) (int, error)
	MarkRead(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error)
	MarkAcknowledged(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error)
	Stats(ctx context.Context, tenantID, announcementID int64) (*communication.PortalAnnouncementStats, error)
}
type ParentDeclarationRepository interface {
	DeclarationVersions
	DeclarationSubmissions
	DeclarationAudience
}
type DeclarationVersions interface {
	LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*communication.PortalDeclarationVersion, error)
	ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationVersion, error)
	LatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*communication.PortalDeclarationVersion, error)
	InsertDeclarationVersion(ctx context.Context, version *communication.PortalDeclarationVersion) error
	DeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]communication.PortalAnnouncementDeclarationSettings, error)
}
type DeclarationSubmissions interface {
	CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error)
	ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationSubmission, error)
	ListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs, studentIDs []int64) ([]*communication.PortalDeclarationSubmission, error)
	LockDeclarationChild(ctx context.Context, announcementID, studentID int64) error
	InsertDeclarationSubmission(ctx context.Context, submission *communication.PortalDeclarationSubmission) error
}
type DeclarationAudience interface {
	DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationChild, error)
	DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error)
	DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalDeclarationChild, error)
	DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error)
	HoldDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*communication.PortalDeclarationSignerContext, error)
	HoldHistoricalDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*communication.PortalDeclarationSignerContext, error)
	ReadOpenDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error)
}
type ParentMessageRepository interface {
	communication.ParentMessageRecords
	communication.PortalMessageReads

	Create(ctx context.Context, message *communication.PortalParentMessage) error
	Update(ctx context.Context, message *communication.PortalParentMessage) error
	FindByID(ctx context.Context, id int64) (*communication.PortalParentMessage, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*communication.PortalParentMessage, error)
	ListByThread(ctx context.Context, threadID int64, limit int) ([]*communication.PortalParentMessage, error)
	FindEventByRef(ctx context.Context, threadID int64, eventType, refTable string, refID int64) (*communication.PortalParentMessage, error)
}
type ParentMessageThreadRepository interface {
	communication.ParentThreadRecords
	communication.PortalMessageThreadAccess

	Create(ctx context.Context, thread *communication.PortalParentMessageThread) error
	Update(ctx context.Context, thread *communication.PortalParentMessageThread) error
	LockForMessageAppend(ctx context.Context, threadID int64) error
	TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error
	ClaimStaffMessageNotification(ctx context.Context, threadID int64, cooldown time.Duration) (bool, error)
	FindByID(ctx context.Context, id int64) (*communication.PortalParentMessageThread, error)
	FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64, audience string) (*communication.PortalParentMessageThread, error)
	GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64, audience string) (*communication.PortalParentMessageThread, error)
	ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*communication.ParentGuardianRecipient, error)
	ListStudentIDsWithMessageableGuardian(ctx context.Context) ([]int64, error)
}
type ParentMessageReadRepository interface {
	communication.ParentReadRecords
	communication.PortalMessageInboxReads
	MessageReadCommands
	MessageInboxQueries
}
type MessageReadCommands interface {
	MarkReadUpTo(ctx context.Context, tenantID, threadID, accountID int64, readAt time.Time, readMessageID int64) (bool, error)
	ClearUnreadForStaff(ctx context.Context, tenantID, accountID int64, bounds []communication.PortalReadCursorBound) ([]int64, error)
	MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error
	MarkStaffUnread(ctx context.Context, tenantID, threadID, accountID int64) error
	ClearStaffUnreadMark(ctx context.Context, tenantID, threadID int64, observedAt time.Time) (bool, error)
}
type MessageInboxQueries interface {
	ListInboxForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, onlyUnread bool) ([]*communication.PortalInboxThread, error)
	ListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach communication.ParentMessageStaffReach) ([]*communication.PortalInboxThread, error)
	ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error)
	ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error)
	UnreadMessageCountForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach) (int, error)
	UnreadMessageCountForStaffInGroups(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, groupIDs []int64) (int, error)
	UnreadMessageCountForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) (int, error)
	FindThreadHeader(ctx context.Context, threadID int64) (*communication.ParentThreadHeaderRecord, error)
	LatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*communication.ParentReadCursorRecord, error)
	GuardianReadCursor(ctx context.Context, threadID int64) (*communication.ParentReadCursorRecord, error)
}
