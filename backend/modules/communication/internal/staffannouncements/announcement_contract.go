package announcement

import (
	"context"
	"errors"
	"time"

	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

// OutboxEnqueuer is the slice of the email outbox service this package needs:
// queue e-mail rows inside the current tenant transaction, and cancel not-yet-
// sent rows when an announcement is retracted before the worker drains them.
type OutboxEnqueuer interface {
	deliveryPorts.MailOutbox
}

// PushOutboxCanceller is the narrow durable-push operation needed when an
// announcement is retracted. Pending reminder intents otherwise carry a link
// to an announcement that staff has already made unavailable.
type PushOutboxCanceller interface {
	CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error)
}

// DeliveryRecorder is the slice of the delivery repository this package needs:
// write the per-recipient rows behind the staff matrix, and drop them when an
// announcement is retracted. Declared here (rather than importing the repository
// interface wholesale) for the same reason OutboxEnqueuer is — the service
// depends on the operations it uses, not on a data-access surface.
type DeliveryRecorder interface {
	ReplaceForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []EmailDelivery) error
	DeleteForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) (int64, error)
	ListForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]EmailDeliveryStatus, error)
	AttachOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error
	ClaimFailedDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error)
}

const (
	maxTitleLen   = 200
	maxBodyLen    = 4000
	maxLinkURLLen = 2000

	// Poll (Umfrage) option bounds. Two options is the minimum that asks
	// anything ("Ja"/"Nein"); 60 fits an Elternsprechtag-Terminabstimmung
	// (#3861). Mirrored in frontend MAX_POLL_OPTIONS.
	minPollOptions  = 2
	maxPollOptions  = 60
	maxPollLabelLen = 120

	// relatedEntityTypeAnnouncement links an outbox row back to its announcement.
	// Shared by the enqueue and cancel paths so they never drift apart.
	relatedEntityTypeAnnouncement = "parent_announcement"

	parentAnnouncementNotificationType = "parent_announcement"
)

// Sentinel errors mapped to HTTP status by the handler.
var (
	ErrNotFound                    = errors.New("announcement: not found")
	ErrValidation                  = errors.New("announcement: invalid input")
	ErrNewsDisabled                = errors.New("announcement: parent news is disabled for this school")
	ErrSystemAnnouncementImmutable = errors.New("announcement: system announcements cannot be changed")
	// ErrPublishedImmutable: a published announcement is frozen — parents may
	// already have read (or been e-mailed) the exact wording, so silent edits
	// are forbidden. Correcting means unpublish (retract) → edit → republish,
	// which is visible to parents and re-triggers the opted-in e-mail.
	ErrPublishedImmutable = errors.New("announcement: published announcements cannot be edited")
	// ErrNotPublished: reminders and resends need a live announcement.
	ErrNotPublished = errors.New("announcement: announcement is not published")
	// ErrNothingOutstanding: the announcement never asked for anything, so there
	// is nobody to remind.
	ErrNothingOutstanding = errors.New("announcement: nothing is outstanding for this announcement")
)

// TargetInput is one audience selector supplied by the staff author.
type TargetInput struct {
	TargetType string  `json:"target_type"`
	RefID      *int64  `json:"ref_id,omitempty"`
	RefText    *string `json:"ref_text,omitempty"`
}

// Input is the create/update payload. ExpiresAt is optional (nil = never).
// SendEmail opts the announcement into an additional e-mail to the targeted
// guardians when it is published. LinkURL is an optional http(s) link shown
// with the announcement in the parent portal.
type Input struct {
	Title                   string        `json:"title"`
	Body                    string        `json:"body"`
	Priority                string        `json:"priority"`
	LinkURL                 *string       `json:"link_url,omitempty"`
	RequiresAcknowledgement bool          `json:"requires_acknowledgement"`
	SendEmail               bool          `json:"send_email"`
	ExpiresAt               *time.Time    `json:"expires_at,omitempty"`
	Targets                 []TargetInput `json:"targets"`

	// ResponseType turns the announcement into a poll (Umfrage): "" / "none" is a
	// plain Mitteilung, single_choice/multi_choice require Options. Options are
	// the answer labels in display order; ResponseDeadline is the optional answer
	// cut-off (independent of ExpiresAt, which controls visibility).
	ResponseType     string     `json:"response_type"`
	ResponseDeadline *time.Time `json:"response_deadline,omitempty"`
	Options          []string   `json:"options,omitempty"`

	// DeliveryMode "letter" makes this an Elternbrief (#2384): e-mail and
	// acknowledgement become mandatory and the mail carries the full body.
	// EmailAudience decides who receives that mail — "portal_only" (default) is
	// the portal audience, "all_contacts" additionally reaches guardians with an
	// address but no portal access. Both default when empty.
	DeliveryMode  string `json:"delivery_mode,omitempty"`
	EmailAudience string `json:"email_audience,omitempty"`

	// Scheduled reminder (#3162): the instant the announcement goes out a second
	// time to its whole audience, and the optional short wording of that second
	// delivery. Both nil = no reminder.
	ReminderAt   *time.Time `json:"reminder_at,omitempty"`
	ReminderText *string    `json:"reminder_text,omitempty"`
	// Declaration configures an Erklärung (#3430); zero for other modes.
	Declaration AnnouncementDeclarationSettings `json:"-"`
}

// Service is the staff-facing parent-announcement contract.
type Service interface {
	List(ctx context.Context, includeInactive bool) ([]*ParentAnnouncement, error)
	Get(ctx context.Context, id int64) (*ParentAnnouncement, error)
	Create(ctx context.Context, createdBy int64, in Input) (*ParentAnnouncement, error)
	Update(ctx context.Context, id int64, in Input) (*ParentAnnouncement, error)
	Delete(ctx context.Context, id int64) error
	Publish(ctx context.Context, id int64) (*ParentAnnouncement, error)
	Unpublish(ctx context.Context, id int64) (*ParentAnnouncement, error)
	Stats(ctx context.Context, id int64) (*AnnouncementStats, error)
	Recipients(ctx context.Context, id int64) ([]*AnnouncementRecipientStatus, error)

	// --- poll (Umfrage) ---
	// PollResults returns the per-option tally plus reached/answered child counts.
	PollResults(ctx context.Context, id int64) (*AnnouncementPollResults, error)
	// PollChildren returns every reached child with the answer given for them.
	PollChildren(ctx context.Context, id int64) ([]*AnnouncementPollChildStatus, error)
	// RemindUnanswered notifies the guardians of children that have not answered
	// yet and reports how many were reached.
	RemindUnanswered(ctx context.Context, id int64) (int, error)

	// --- Elternbrief (#2384) ---
	// LetterStatus returns the recipient matrix (e-mail and moto status per
	// person) plus the per-child fulfilment behind it.
	LetterStatus(ctx context.Context, id int64) (*LetterStatus, error)
	// RemindOutstanding nudges whoever still owes a response — an unanswered
	// poll or an unconfirmed Elternbrief — and reports how many were reached.
	RemindOutstanding(ctx context.Context, id int64) (int, error)
	// ResendFailedEmails re-queues only the mails that ended up failed.
	ResendFailedEmails(ctx context.Context, id int64) (int, error)

	// --- scheduled reminder (#3162) ---
	// UpdateReminder moves, rewords or removes the scheduled reminder of a live
	// announcement as long as it has not been sent.
	UpdateReminder(ctx context.Context, id int64, in ReminderInput) (*ParentAnnouncement, error)
	// SendDueReminders delivers every reminder of the current tenant that fell
	// due in (notBefore, dueBefore] and reports how many announcements were
	// reminded. retryFrom is set when a temporary delivery suppression needs
	// the scheduler to retain part of its scan window. Runs inside the
	// scheduler's tenant transaction.
	SendDueReminders(ctx context.Context, notBefore, dueBefore time.Time) (sent int, retryFrom *time.Time, err error)

	// --- retention (#4045) ---
	// DeleteExpiredParentAnnouncements deletes the announcements of the
	// school that ended before the window of retentionDays and reports how
	// many went. Runs inside the nightly cleanup's tenant transaction.
	DeleteExpiredParentAnnouncements(ctx context.Context, retentionDays int) (int, error)

	// --- cancellation notice (#2601) ---
	CareCancellationPublisher

	// AttachmentSupport is the announcement side of the attachments (#2890).
	AttachmentSupport
	DeclarationSupport
}
