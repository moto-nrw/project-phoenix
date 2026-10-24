package communication

import (
	"context"
	"time"
)

// Complete native staff-announcement snapshots, without ORM contracts.
type PortalAnnouncementDeliveryRecipient struct {
	GuardianProfileID int64
	AccountID         *int64
	FirstName         string
	LastName          string
	Email             string
	HasPortalAccess   bool
	PortalLocale      string
}
type PortalAnnouncementLetterChildStatus struct {
	StudentID      int64      `json:"student_id"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	SchoolClass    string     `json:"school_class"`
	CanConfirm     bool       `json:"can_confirm"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	AckFirstName   string     `json:"ack_first_name,omitempty"`
	AckLastName    string     `json:"ack_last_name,omitempty"`
}
type PortalAnnouncementPollChildStatus struct {
	StudentID     int64      `json:"student_id"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	SchoolClass   string     `json:"school_class"`
	AnswerLabels  []string   `json:"answer_labels"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
	CanAnswer     bool       `json:"can_answer"`
	GuardianCount int        `json:"-"`
}
type PortalAnnouncementPollOptionResult struct {
	OptionID int64  `json:"option_id"`
	Label    string `json:"label"`
	Position int    `json:"position"`
	Count    int    `json:"count"`
}
type PortalAnnouncementPollReminderRecipient struct {
	AccountID    int64
	Email        string
	FirstName    string
	LastName     string
	PortalLocale string
}
type PortalAnnouncementPollResults struct {
	ChildCount       int                                   `json:"child_count"`
	TargetChildCount int                                   `json:"target_child_count"`
	AnsweredCount    int                                   `json:"answered_count"`
	Options          []*PortalAnnouncementPollOptionResult `json:"options"`
}
type PortalAnnouncementRecipient struct {
	AccountID int64
	Email     string
	FirstName string
	LastName  string
}
type PortalAnnouncementRecipientStatus struct {
	AccountID      int64      `json:"account_id"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	PortalLocale   string     `json:"-"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
}
type PortalAnnouncementStats struct {
	TargetCount       int `json:"target_count"`
	ReadCount         int `json:"read_count"`
	AcknowledgedCount int `json:"acknowledged_count"`
}
type AnnouncementDraftRecords interface {
	AnnouncementCreate(ctx context.Context, a *PortalParentAnnouncement) error
	AnnouncementUpdate(ctx context.Context, a *PortalParentAnnouncement) error
	PortalFindByID(ctx context.Context, id int64) (*PortalParentAnnouncement, error)
	AnnouncementFindByIDForUpdate(ctx context.Context, id int64) (*PortalParentAnnouncement, error)
	AnnouncementDelete(ctx context.Context, id int64) error
	AnnouncementListForTenant(ctx context.Context, includeInactive bool) ([]*PortalParentAnnouncement, error)
	AnnouncementSetPublished(ctx context.Context, id int64, publishedAt *time.Time) error
	AnnouncementClearEngagement(ctx context.Context, announcementID int64) error
	AnnouncementPublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error)
}
type AnnouncementReminderRecords interface {
	AnnouncementSetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error)
	AnnouncementListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*PortalParentAnnouncement, error)
	AnnouncementClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error)
	AnnouncementReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error)
}
type AnnouncementTargetRecords interface {
	AnnouncementReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*PortalParentAnnouncementTarget) error
	AnnouncementListTargets(ctx context.Context, announcementID int64) ([]*PortalParentAnnouncementTarget, error)
	AnnouncementReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*PortalParentAnnouncementOption) error
	PortalListOptions(ctx context.Context, announcementID int64) ([]*PortalParentAnnouncementOption, error)
	PortalListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*PortalParentAnnouncementOption, error)
}
type AnnouncementPollRecords interface {
	AnnouncementPollResults(ctx context.Context, tenantID, announcementID int64) (*PortalAnnouncementPollResults, error)
	AnnouncementPollChildren(ctx context.Context, tenantID, announcementID int64) ([]*PortalAnnouncementPollChildStatus, error)
	AnnouncementUnansweredReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*PortalAnnouncementPollReminderRecipient, error)
}
type AnnouncementAudienceRecords interface {
	AnnouncementResolveAudienceEmails(ctx context.Context, tenantID, announcementID int64) ([]*PortalAnnouncementRecipient, error)
	AnnouncementUnacknowledgedReminderRecipients(ctx context.Context, tenantID, announcementID int64) ([]*PortalAnnouncementPollReminderRecipient, error)
	AnnouncementLetterChildStatuses(ctx context.Context, tenantID, announcementID int64) ([]*PortalAnnouncementLetterChildStatus, error)
	AnnouncementResolveDeliveryRecipients(ctx context.Context, tenantID, announcementID int64) ([]*PortalAnnouncementDeliveryRecipient, error)
	AnnouncementSchoolName(ctx context.Context, tenantID int64) (string, error)
	AnnouncementAudienceRecipients(ctx context.Context, tenantID, announcementID int64) ([]*PortalAnnouncementRecipientStatus, error)
	AnnouncementCountReachableGuardiansForStudents(ctx context.Context, tenantID int64, studentIDs []int64) (int, error)
	AnnouncementStats(ctx context.Context, tenantID, announcementID int64) (*PortalAnnouncementStats, error)
}
type AnnouncementDeclarationRecords interface {
	AnnouncementLatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*PortalDeclarationVersion, error)
	AnnouncementListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*PortalDeclarationVersion, error)
	AnnouncementInsertDeclarationVersion(ctx context.Context, version *PortalDeclarationVersion) error
	AnnouncementCountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error)
	AnnouncementListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*PortalDeclarationSubmission, error)
	AnnouncementDeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*PortalDeclarationChild, error)
	AnnouncementDeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*PortalDeclarationSigner, error)
}
type AnnouncementConflictClassifier interface{ IsParentAnnouncementPublishedConflict(error) bool }
type StaffAnnouncementRecords interface {
	AnnouncementDraftRecords
	AnnouncementReminderRecords
	AnnouncementTargetRecords
	AnnouncementPollRecords
	AnnouncementAudienceRecords
	AnnouncementDeclarationRecords
	AnnouncementConflictClassifier
}
