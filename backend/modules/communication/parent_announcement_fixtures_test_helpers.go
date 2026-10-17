//go:build testhooks

package communication

import "time"

// ParentAnnouncementFixture is the complete announcement construction value
// used by workflow integration suites. The root binds it to the real store.
type ParentAnnouncementFixture struct {
	ID                      int64
	TenantID                int64
	CreatedAt               time.Time
	UpdatedAt               time.Time
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
	Targets                 []*ParentAnnouncementTargetFixture
	Options                 []*ParentAnnouncementOptionFixture
	Declaration             ParentDeclarationSettings
}

// ParentAnnouncementTargetFixture is one persisted audience target.
type ParentAnnouncementTargetFixture struct {
	ID             int64
	TenantID       int64
	AnnouncementID int64
	TargetType     string
	TargetRefID    *int64
	TargetRefText  *string
	CreatedAt      time.Time
}

// ParentAnnouncementOptionFixture is one persisted poll option.
type ParentAnnouncementOptionFixture struct {
	ID             int64
	TenantID       int64
	AnnouncementID int64
	Label          string
	Position       int
	CreatedAt      time.Time
}
