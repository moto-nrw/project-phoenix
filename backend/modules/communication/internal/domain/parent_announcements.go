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
	// Scheduled one-off reminder (#3162): moment, optional wording, sent mark.
	ReminderAt     *time.Time
	ReminderText   *string
	ReminderSentAt *time.Time
	Declaration    DeclarationSettings
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Targets        []*ParentAnnouncementTarget
	Options        []*ParentAnnouncementOption
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

// ErrParentAnnouncementPublished reports a draft-only write that found the
// announcement already published: the row is no longer editable.
var ErrParentAnnouncementPublished = errors.New("users: parent announcement is published (not editable)")

func AnnouncementResponseIsPoll(responseType string) bool {
	return responseType == "single_choice" || responseType == "multi_choice"
}

func AnnouncementIsDeclaration(deliveryMode string) bool { return deliveryMode == "declaration" }
