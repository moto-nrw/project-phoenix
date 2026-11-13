//go:build testhooks

package parentstore

import (
	"time"

	"github.com/moto-ogs/project-phoenix/modules/communication"
)

// The announcement construction values live in this already tagged store
// package, not in the Communication public package, so that package compiles
// identically with and without testhooks (#5985).

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
	Declaration             communication.ParentDeclarationSettings
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
