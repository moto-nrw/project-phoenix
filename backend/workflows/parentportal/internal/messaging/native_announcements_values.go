package messaging

import (
	"time"
) // Complete native values retain source metadata, nullable fields and JSON presence.
type AnnouncementFeedItem struct {
	ID                      int64                        `json:"id"`
	TenantID                int64                        `json:"tenant_id"`
	Title                   string                       `json:"title"`
	Body                    string                       `json:"body"`
	Priority                string                       `json:"priority"`
	LinkURL                 *string                      `json:"link_url,omitempty"`
	RequiresAcknowledgement bool                         `json:"requires_acknowledgement"`
	PublishedAt             *time.Time                   `json:"published_at,omitempty"`
	ExpiresAt               *time.Time                   `json:"expires_at,omitempty"`
	ResponseType            string                       `json:"response_type"`
	DeliveryMode            string                       `json:"delivery_mode"`
	ResponseDeadline        *time.Time                   `json:"response_deadline,omitempty"`
	SystemKind              *string                      `json:"system_kind,omitempty"`
	SchoolName              string                       `json:"school_name"`
	ReadAt                  *time.Time                   `json:"read_at,omitempty"`
	AcknowledgedAt          *time.Time                   `json:"acknowledged_at,omitempty"`
	ReminderSentAt          *time.Time                   `json:"reminder_sent_at,omitempty"`
	ReminderText            *string                      `json:"reminder_text,omitempty"`
	Options                 []*ParentAnnouncementOption  `json:"options,omitempty"`
	Children                []*AnnouncementPollChild     `json:"children,omitempty"`
	Declaration             *AnnouncementFeedDeclaration `json:"-"`
	Poll                    bool                         `json:"-"`
}
type AnnouncementFeedScope struct {
	TenantIDs           []int64
	SystemOnlyTenantIDs []int64
}
type AnnouncementPollChild struct {
	AnnouncementID  int64   `json:"-"`
	StudentID       int64   `json:"-"`
	FirstName       string  `json:"first_name"`
	LastName        string  `json:"last_name"`
	SelectedOptions []int64 `json:"-"`
}
type ParentAnnouncement struct {
	ID                      int64                           `json:"id"`
	CreatedAt               time.Time                       `json:"created_at"`
	UpdatedAt               time.Time                       `json:"updated_at"`
	TenantID                int64                           `json:"tenant_id"`
	Title                   string                          `json:"title"`
	Body                    string                          `json:"body"`
	Priority                string                          `json:"priority"`
	LinkURL                 *string                         `json:"link_url,omitempty"`
	RequiresAcknowledgement bool                            `json:"requires_acknowledgement"`
	SendEmail               bool                            `json:"send_email"`
	PublishedAt             *time.Time                      `json:"published_at,omitempty"`
	ExpiresAt               *time.Time                      `json:"expires_at,omitempty"`
	Active                  bool                            `json:"active"`
	CreatedBy               int64                           `json:"created_by"`
	ResponseType            string                          `json:"response_type"`
	ResponseDeadline        *time.Time                      `json:"response_deadline,omitempty"`
	DeliveryMode            string                          `json:"delivery_mode"`
	EmailAudience           string                          `json:"email_audience"`
	SystemKind              *string                         `json:"system_kind,omitempty"`
	ReminderAt              *time.Time                      `json:"reminder_at,omitempty"`
	ReminderText            *string                         `json:"reminder_text,omitempty"`
	ReminderSentAt          *time.Time                      `json:"reminder_sent_at,omitempty"`
	Targets                 []*ParentAnnouncementTarget     `json:"targets,omitempty"`
	Options                 []*ParentAnnouncementOption     `json:"options,omitempty"`
	Declaration             AnnouncementDeclarationSettings `json:"-"`
	Poll                    bool                            `json:"-"`
	DeclarationMode         bool                            `json:"-"`
}
type ParentAnnouncementOption struct {
	TenantID       int64     `json:"tenant_id"`
	ID             int64     `json:"id"`
	AnnouncementID int64     `json:"announcement_id"`
	Label          string    `json:"label"`
	Position       int       `json:"position"`
	CreatedAt      time.Time `json:"created_at"`
}
type ParentAnnouncementTarget struct {
	TenantID       int64     `json:"tenant_id"`
	ID             int64     `json:"id"`
	AnnouncementID int64     `json:"announcement_id"`
	TargetType     string    `json:"target_type"`
	TargetRefID    *int64    `json:"target_ref_id,omitempty"`
	TargetRefText  *string   `json:"target_ref_text,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
