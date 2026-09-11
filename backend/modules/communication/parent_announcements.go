package communication

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrParentAnnouncementNotFound        = errors.New("announcement: not found")
	ErrParentAnnouncementValidation      = errors.New("announcement: invalid input")
	ErrParentNewsDisabled                = errors.New("announcement: parent news is disabled for this school")
	ErrSystemParentAnnouncementImmutable = errors.New("announcement: system announcements cannot be changed")
	ErrPublishedParentAnnouncement       = errors.New("announcement: published announcements cannot be edited")
	ErrParentAnnouncementNotPublished    = errors.New("announcement: announcement is not published")
	ErrParentAnnouncementNothingDue      = errors.New("announcement: nothing is outstanding for this announcement")
	ErrParentAnnouncementNotPoll         = errors.New("announcement: not a poll")
	ErrParentAnnouncementPollClosed      = errors.New("announcement: poll is not open for answers")
	ErrCareCancellationDisabled          = errors.New("announcement: cancellation notice is disabled for this school")
)

type ParentAnnouncementTargetInput struct {
	TargetType string
	RefID      *int64
	RefText    *string
}

type ParentAnnouncementInput struct {
	Title                   string
	Body                    string
	Priority                string
	LinkURL                 *string
	RequiresAcknowledgement bool
	SendEmail               bool
	ExpiresAt               *time.Time
	Targets                 []ParentAnnouncementTargetInput
	ResponseType            string
	ResponseDeadline        *time.Time
	Options                 []string
	DeliveryMode            string
	EmailAudience           string
}

type ParentAnnouncement struct {
	ID                      int64
	Title                   string
	Body                    string
	Priority                string
	LinkURL                 *string
	RequiresAcknowledgement bool
	SendEmail               bool
	PublishedAt             *time.Time
	ExpiresAt               *time.Time
	Active                  bool
	CreatedAt               time.Time
	UpdatedAt               time.Time
	Targets                 []ParentAnnouncementTarget
	ResponseType            string
	ResponseDeadline        *time.Time
	Options                 []ParentAnnouncementOption
	DeliveryMode            string
	EmailAudience           string
	SystemKind              *string
}

type ParentAnnouncementTarget struct {
	TargetType string
	RefID      *int64
	RefText    *string
}

type ParentAnnouncementOption struct {
	ID    int64
	Label string
}

type ParentAnnouncementStats struct {
	TargetCount       int
	ReadCount         int
	AcknowledgedCount int
}

type ParentAnnouncementRecipient struct {
	AccountID      int64
	FirstName      string
	LastName       string
	ReadAt         *time.Time
	AcknowledgedAt *time.Time
}

type ParentAnnouncementPollOption struct {
	OptionID int64
	Label    string
	Count    int
}

type ParentAnnouncementPollResults struct {
	ChildCount       int
	TargetChildCount int
	AnsweredCount    int
	Options          []ParentAnnouncementPollOption
}

type ParentAnnouncementPollChild struct {
	StudentID    int64
	FirstName    string
	LastName     string
	SchoolClass  string
	AnswerLabels []string
	RespondedAt  *time.Time
	CanAnswer    bool
}

type ParentAnnouncementLetterRecipient struct {
	FirstName      string
	LastName       string
	RecipientEmail *string
	EmailStatus    string
	Reachability   string
	LastError      *string
	SentAt         *time.Time
}

type ParentAnnouncementLetterChild struct {
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
	CanConfirm     bool
	AcknowledgedAt *time.Time
	AckFirstName   string
	AckLastName    string
}

func (c ParentAnnouncementLetterChild) Fulfilled() bool { return c.AcknowledgedAt != nil }

type ParentAnnouncementLetterSummary struct {
	ChildrenTotal         int `json:"children_total"`
	ChildrenConfirmable   int `json:"children_confirmable"`
	ChildrenFulfilled     int `json:"children_fulfilled"`
	ChildrenOpen          int `json:"children_open"`
	ChildrenWithoutPortal int `json:"children_without_portal"`
	RecipientsTotal       int `json:"recipients_total"`
	EmailsSent            int `json:"emails_sent"`
	EmailsPending         int `json:"emails_pending"`
	EmailsFailed          int `json:"emails_failed"`
	WithoutEmail          int `json:"without_email"`
	WithoutPortal         int `json:"without_portal"`
}

type ParentAnnouncementLetterStatus struct {
	Recipients []ParentAnnouncementLetterRecipient
	Children   []ParentAnnouncementLetterChild
	Summary    ParentAnnouncementLetterSummary
}

// ParentAnnouncementEmailDelivery is one addressed guardian delivery created
// atomically with publication.
type ParentAnnouncementEmailDelivery struct {
	OutboxID          *int64
	GuardianProfileID *int64
	AccountID         *int64
	RecipientEmail    *string
	Reachability      string
}

func (d ParentAnnouncementEmailDelivery) Queued() bool { return d.OutboxID != nil }

type ParentAnnouncementEmailDeliveryStatus struct {
	DeliveryID        int64
	GuardianProfileID *int64
	AccountID         *int64
	FirstName         string
	LastName          string
	RecipientEmail    *string
	Reachability      string
	EmailStatus       string
	LastError         *string
	SentAt            *time.Time
	Attempts          int
}

type CareCancellationInput struct {
	StudentIDs []int64
	Title      string
	Body       string
	CreatedBy  int64
}

type CareCancellationResult struct {
	AnnouncementID int64
	RecipientCount int
}

type CareCancellationReach struct {
	Enabled     bool
	DefaultOn   bool
	FamilyCount int
}

type ParentAnnouncementAttachmentPurger interface {
	QueueAttachmentCleanupForAnnouncement(context.Context, int64) error
	CountAttachments(context.Context, int64) (int, error)
}

type CareCancellationPublisher interface {
	PublishCareCancellation(context.Context, CareCancellationInput) (*CareCancellationResult, error)
	CareCancellationReachFor(context.Context, []int64) (*CareCancellationReach, error)
}

type ParentAnnouncementAttachmentSupport interface {
	AnnouncementExists(context.Context, int64) (bool, error)
	AnnouncementEditable(context.Context, int64) (bool, error)
	LockAnnouncementForAttachmentChange(context.Context, int64) (bool, bool, error)
	ResetAnnouncementEngagement(context.Context, int64) error
	SetAttachmentPurger(ParentAnnouncementAttachmentPurger)
}

type ParentAnnouncementCapability interface {
	ListParentAnnouncements(context.Context, bool) ([]ParentAnnouncement, error)
	GetParentAnnouncement(context.Context, int64) (*ParentAnnouncement, error)
	CreateParentAnnouncement(context.Context, int64, ParentAnnouncementInput) (*ParentAnnouncement, error)
	UpdateParentAnnouncement(context.Context, int64, ParentAnnouncementInput) (*ParentAnnouncement, error)
	DeleteParentAnnouncement(context.Context, int64) error
	PublishParentAnnouncement(context.Context, int64) (*ParentAnnouncement, error)
	UnpublishParentAnnouncement(context.Context, int64) (*ParentAnnouncement, error)
	ParentAnnouncementStats(context.Context, int64) (*ParentAnnouncementStats, error)
	ParentAnnouncementRecipients(context.Context, int64) ([]ParentAnnouncementRecipient, error)
	ParentAnnouncementPollResults(context.Context, int64) (*ParentAnnouncementPollResults, error)
	ParentAnnouncementPollChildren(context.Context, int64) ([]ParentAnnouncementPollChild, error)
	RemindParentAnnouncement(context.Context, int64) (int, error)
	ParentAnnouncementLetterStatus(context.Context, int64) (*ParentAnnouncementLetterStatus, error)
	ResendParentAnnouncementEmails(context.Context, int64) (int, error)
	CareCancellationPublisher
	ParentAnnouncementAttachmentSupport
}

func ValidateCareCancellationText(title, body string) (string, string, error) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if title == "" || len([]rune(title)) > maxTitleLen {
		return "", "", fmt.Errorf("%w: title is required and at most %d characters", ErrParentAnnouncementValidation, maxTitleLen)
	}
	if body == "" || len([]rune(body)) > 4000 {
		return "", "", fmt.Errorf("%w: body is required and at most %d characters", ErrParentAnnouncementValidation, 4000)
	}
	return title, body, nil
}
