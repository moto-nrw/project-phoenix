package announcement

import (
	"context"
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

type AnnouncementDeclarationSettings values.AnnouncementDeclarationSettings
type AnnouncementDeliveryRecipient values.AnnouncementDeliveryRecipient
type AnnouncementLetterChildStatus values.AnnouncementLetterChildStatus
type AnnouncementPollChildStatus values.AnnouncementPollChildStatus
type AnnouncementPollOptionResult values.AnnouncementPollOptionResult
type AnnouncementPollReminderRecipient values.AnnouncementPollReminderRecipient
type AnnouncementPollResults struct {
	ChildCount       int
	TargetChildCount int
	AnsweredCount    int
	Options          []*AnnouncementPollOptionResult
}
type AnnouncementRecipient values.AnnouncementRecipient
type AnnouncementRecipientStatus values.AnnouncementRecipientStatus
type AnnouncementStats values.AnnouncementStats
type DeclarationAttachmentDigest values.DeclarationAttachmentDigest
type DeclarationChild values.DeclarationChild
type DeclarationSigner values.DeclarationSigner
type DeclarationSubmission values.DeclarationSubmission
type DeclarationVersion struct {
	ID             int64
	TenantID       int64
	AnnouncementID int64
	VersionNo      int
	Title          string
	Body           string
	Kind           string
	Attachments    []DeclarationAttachmentDigest
	ContentHash    string
	PublishedAt    time.Time
}
type ParentAnnouncement struct {
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
	ReminderAt              *time.Time
	ReminderText            *string
	ReminderSentAt          *time.Time
	Targets                 []*ParentAnnouncementTarget
	Options                 []*ParentAnnouncementOption
	Declaration             AnnouncementDeclarationSettings
}
type ParentAnnouncementOption values.ParentAnnouncementOption
type ParentAnnouncementTarget values.ParentAnnouncementTarget
type ParentAnnouncementRepository interface {
	Create(ctx context.Context, a *ParentAnnouncement) error
	Update(ctx context.Context, a *ParentAnnouncement) error
	FindByID(ctx context.Context, id int64) (*ParentAnnouncement, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*ParentAnnouncement, error)
	Delete(ctx context.Context, id int64) error
	ListForTenant(ctx context.Context, includeInactive bool) ([]*ParentAnnouncement, error)
	SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error
	ClearEngagement(ctx context.Context, announcementID int64) error
	PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error)
	SetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error)
	ListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*ParentAnnouncement, error)
	ClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error)
	ReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error)
	ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*ParentAnnouncementTarget) error
	ListTargets(ctx context.Context, announcementID int64) ([]*ParentAnnouncementTarget, error)
	ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*ParentAnnouncementOption) error
	ListOptions(ctx context.Context, announcementID int64) ([]*ParentAnnouncementOption, error)
	ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*ParentAnnouncementOption, error)
	PollResults(ctx context.Context, tenantID, announcementID int64) (*AnnouncementPollResults, error)
	PollChildren(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollChildStatus, error)
	UnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollReminderRecipient, error)
	ResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementRecipient, error)
	UnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementPollReminderRecipient, error)
	LetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementLetterChildStatus, error)
	ResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementDeliveryRecipient, error)
	SchoolName(ctx context.Context, tenantID int64) (string, error)
	AudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*AnnouncementRecipientStatus, error)
	CountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error)
	Stats(ctx context.Context, tenantID, announcementID int64) (*AnnouncementStats, error)
	LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*DeclarationVersion, error)
	ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationVersion, error)
	InsertDeclarationVersion(ctx context.Context, version *DeclarationVersion) error
	CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error)
	ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationSubmission, error)
	DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationChild, error)
	DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*DeclarationSigner, error)
	IsPublishedConflict(error) bool
}

func (v *ParentAnnouncement) GetTenantID() int64   { return v.TenantID }
func (v *ParentAnnouncement) SetTenantID(id int64) { v.TenantID = id }

type AnnouncementFeedItem values.AnnouncementFeedItem
type AnnouncementFeedScope values.AnnouncementFeedScope
type AnnouncementPollChild values.AnnouncementPollChild
type ParentDeclarationRepository interface {
	LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*DeclarationVersion, error)
	ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationVersion, error)
	InsertDeclarationVersion(ctx context.Context, version *DeclarationVersion) error
	CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error)
	ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationSubmission, error)
	DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*DeclarationChild, error)
	DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*DeclarationSigner, error)
}
