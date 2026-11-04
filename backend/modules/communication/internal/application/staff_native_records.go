package application

import (
	"context"
	"time"
)

type StaffAnnouncementDeclarationSettings AnnouncementDeclarationSettings
type StaffAnnouncementDeliveryRecipient AnnouncementDeliveryRecipient
type StaffAnnouncementLetterChildStatus AnnouncementLetterChildStatus
type StaffAnnouncementPollChildStatus AnnouncementPollChildStatus
type StaffAnnouncementPollOptionResult AnnouncementPollOptionResult
type StaffAnnouncementPollReminderRecipient AnnouncementPollReminderRecipient
type StaffAnnouncementPollResults struct {
	ChildCount       int
	TargetChildCount int
	AnsweredCount    int
	Options          []*StaffAnnouncementPollOptionResult
}
type StaffAnnouncementRecipient AnnouncementRecipient
type StaffAnnouncementRecipientStatus AnnouncementRecipientStatus
type StaffAnnouncementStats AnnouncementStats
type StaffDeclarationAttachmentDigest DeclarationAttachmentDigest
type StaffDeclarationChild DeclarationChild
type StaffDeclarationSigner DeclarationSigner
type StaffDeclarationSubmission DeclarationSubmission
type StaffDeclarationVersion struct {
	ID             int64
	TenantID       int64
	AnnouncementID int64
	VersionNo      int
	Title          string
	Body           string
	Kind           string
	Attachments    []StaffDeclarationAttachmentDigest
	ContentHash    string
	PublishedAt    time.Time
}
type StaffParentAnnouncement struct {
	ID                      int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
	TenantID                int64
	Title                   string
	Body                    string
	Priority                string
	LinkURL                 *string
	RequiresAcknowledgement bool
	SendEmail               bool
	PublishedAt             *time.Time
	ExpiresAt               *time.Time
	Active                  bool
	CreatedBy               int64
	ResponseType            string
	ResponseDeadline        *time.Time
	DeliveryMode            string
	EmailAudience           string
	SystemKind              *string
	SenderName              *string
	SenderSchoolClass       *string
	ReminderAt              *time.Time
	ReminderText            *string
	ReminderSentAt          *time.Time
	Targets                 []*StaffParentAnnouncementTarget
	Options                 []*StaffParentAnnouncementOption
	Declaration             StaffAnnouncementDeclarationSettings
}
type StaffParentAnnouncementOption ParentAnnouncementOption
type StaffParentAnnouncementTarget ParentAnnouncementTarget
type ParentAnnouncementRepository interface {
	Create(ctx context.Context, a *StaffParentAnnouncement) error
	Update(ctx context.Context, a *StaffParentAnnouncement) error
	FindByID(ctx context.Context, id int64) (*StaffParentAnnouncement, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*StaffParentAnnouncement, error)
	Delete(ctx context.Context, id int64) error
	ListForTenant(ctx context.Context, includeInactive bool) ([]*StaffParentAnnouncement, error)
	SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error
	ClearEngagement(ctx context.Context, announcementID int64) error
	PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error)
	SetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error)
	ListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*StaffParentAnnouncement, error)
	ClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error)
	ReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error)
	ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*StaffParentAnnouncementTarget) error
	ListTargets(ctx context.Context, announcementID int64) ([]*StaffParentAnnouncementTarget, error)
	ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*StaffParentAnnouncementOption) error
	ListOptions(ctx context.Context, announcementID int64) ([]*StaffParentAnnouncementOption, error)
	ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*StaffParentAnnouncementOption, error)
	PollResults(ctx context.Context, tenantID, announcementID int64) (*StaffAnnouncementPollResults, error)
	PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*StaffAnnouncementPollChildStatus, error)
	UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*StaffAnnouncementPollReminderRecipient, error)
	ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*StaffAnnouncementRecipient, error)
	UnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*StaffAnnouncementPollReminderRecipient, error)
	LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*StaffAnnouncementLetterChildStatus, error)
	ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*StaffAnnouncementDeliveryRecipient, error)
	SchoolName(ctx context.Context, tenantID int64) (string, error)
	AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*StaffAnnouncementRecipientStatus, error)
	CountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error)
	Stats(ctx context.Context, tenantID, announcementID int64) (*StaffAnnouncementStats, error)
	LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*StaffDeclarationVersion, error)
	ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*StaffDeclarationVersion, error)
	InsertDeclarationVersion(ctx context.Context, version *StaffDeclarationVersion) error
	CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error)
	ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*StaffDeclarationSubmission, error)
	DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*StaffDeclarationChild, error)
	DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*StaffDeclarationSigner, error)
	IsPublishedConflict(error) bool
}

func (v *StaffParentAnnouncement) GetTenantID() int64   { return v.TenantID }
func (v *StaffParentAnnouncement) SetTenantID(id int64) { v.TenantID = id }

type StaffAnnouncementFeedItem AnnouncementFeedItem
type StaffAnnouncementFeedScope AnnouncementFeedScope
type StaffAnnouncementPollChild AnnouncementPollChild
type ParentDeclarationRepository interface {
	LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*StaffDeclarationVersion, error)
	ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*StaffDeclarationVersion, error)
	InsertDeclarationVersion(ctx context.Context, version *StaffDeclarationVersion) error
	CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error)
	ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*StaffDeclarationSubmission, error)
	DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*StaffDeclarationChild, error)
	DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*StaffDeclarationSigner, error)
}
