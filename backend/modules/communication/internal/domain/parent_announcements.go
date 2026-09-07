package domain

import (
	"errors"
	"time"
)

// ParentAnnouncement is a school-authored broadcast to guardians. It is a
// draft while PublishedAt is nil and visible to its audience once published.
type ParentAnnouncement struct {
	ID                      int64
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
	CreatedAt               time.Time
	UpdatedAt               time.Time
	Targets                 []*ParentAnnouncementTarget
	Options                 []*ParentAnnouncementOption
}

// ParentAnnouncementTarget is one audience selector of an announcement.
type ParentAnnouncementTarget struct {
	ID             int64
	TenantID       int64
	AnnouncementID int64
	TargetType     string
	TargetRefID    *int64
	TargetRefText  *string
	CreatedAt      time.Time
}

// ParentAnnouncementOption is one answer option of a poll.
type ParentAnnouncementOption struct {
	ID             int64
	TenantID       int64
	AnnouncementID int64
	Label          string
	Position       int
	CreatedAt      time.Time
}

// ParentAnnouncementFeedScope names the schools a guardian's feed draws from:
// schools with parent news on contribute every live announcement, schools with
// only the cancellation notice on contribute system-authored rows.
type ParentAnnouncementFeedScope struct {
	TenantIDs           []int64
	SystemOnlyTenantIDs []int64
}

// IsEmpty reports whether no school contributes anything.
func (s ParentAnnouncementFeedScope) IsEmpty() bool {
	return len(s.TenantIDs) == 0 && len(s.SystemOnlyTenantIDs) == 0
}

// ParentAnnouncementFeedItem is one row of the guardian-facing feed with the
// reading account's read/acknowledgement state.
type ParentAnnouncementFeedItem struct {
	ID                      int64
	TenantID                int64
	Title                   string
	Body                    string
	Priority                string
	LinkURL                 *string
	RequiresAcknowledgement bool
	PublishedAt             *time.Time
	ExpiresAt               *time.Time
	ResponseType            string
	ResponseDeadline        *time.Time
	DeliveryMode            string
	SystemKind              *string
	ReadAt                  *time.Time
	AcknowledgedAt          *time.Time
}

// ParentAnnouncementPollChild is one child a guardian may answer a poll for,
// with the option ids currently selected for that child.
type ParentAnnouncementPollChild struct {
	AnnouncementID  int64
	StudentID       int64
	FirstName       string
	LastName        string
	SelectedOptions []int64
}

// ParentAnnouncementPollOptionResult is one option's tally.
type ParentAnnouncementPollOptionResult struct {
	OptionID int64
	Label    string
	Position int
	Count    int
}

// ParentAnnouncementPollResults is the staff-facing evaluation of a poll.
type ParentAnnouncementPollResults struct {
	ChildCount       int
	TargetChildCount int
	AnsweredCount    int
	Options          []*ParentAnnouncementPollOptionResult
}

// ParentAnnouncementPollChildStatus is one reached child in the staff-facing
// answer list.
type ParentAnnouncementPollChildStatus struct {
	StudentID    int64
	FirstName    string
	LastName     string
	SchoolClass  string
	AnswerLabels []string
	RespondedAt  *time.Time
	CanAnswer    bool
}

// ParentAnnouncementReminderRecipient is one guardian to remind, with the
// address and locale the mail needs.
type ParentAnnouncementReminderRecipient struct {
	AccountID    int64
	Email        string
	FirstName    string
	LastName     string
	PortalLocale string
}

// ParentAnnouncementRecipient is one guardian to e-mail on publication.
type ParentAnnouncementRecipient struct {
	AccountID int64
	Email     string
	FirstName string
	LastName  string
}

// ParentAnnouncementDeliveryRecipient is one guardian linked to a reached
// child, resolved without the portal-access filter so the recipient matrix can
// show who receives nothing and why.
type ParentAnnouncementDeliveryRecipient struct {
	GuardianProfileID int64
	AccountID         *int64
	FirstName         string
	LastName          string
	Email             string
	HasPortalAccess   bool
	PortalLocale      string
}

// ParentAnnouncementLetterChildStatus is one reached child of an Elternbrief
// with its derived fulfilment state.
type ParentAnnouncementLetterChildStatus struct {
	StudentID      int64
	FirstName      string
	LastName       string
	SchoolClass    string
	CanConfirm     bool
	AcknowledgedAt *time.Time
	AckFirstName   string
	AckLastName    string
}

// ParentAnnouncementRecipientStatus is one guardian account in the live
// audience with that account's read/acknowledgement state.
type ParentAnnouncementRecipientStatus struct {
	AccountID      int64
	FirstName      string
	LastName       string
	PortalLocale   string
	ReadAt         *time.Time
	AcknowledgedAt *time.Time
}

// ParentAnnouncementStats is the staff-facing reach/engagement summary.
type ParentAnnouncementStats struct {
	TargetCount       int
	ReadCount         int
	AcknowledgedCount int
}

// PendingAnnouncementApplicant identifies an account or pre-account e-mail
// with an undecided enrollment application. Enrollment owns those rows; the
// audience projection receives them as values and never joins its tables.
//
// The json tags are load-bearing: the projection binds the slice as a jsonb
// record set, so the tag names are the SQL column names.
type PendingAnnouncementApplicant struct {
	TenantID          int64  `json:"tenant_id"`
	GuardianFirstName string `json:"guardian_first_name"`
	GuardianLastName  string `json:"guardian_last_name"`
	GuardianAccountID *int64 `json:"guardian_account_id"`
	GuardianEmail     string `json:"guardian_email"`
}

// ErrParentAnnouncementPublished reports a draft-only write that found the
// announcement already published: the row is no longer editable.
var ErrParentAnnouncementPublished = errors.New("communication: parent announcement is published (not editable)")
