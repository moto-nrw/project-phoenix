package platform

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Announcement type constants
const (
	TypeAnnouncement = "announcement"
	TypeRelease      = "release"
	TypeMaintenance  = "maintenance"
)

// Announcement severity constants
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
)

// Target role constants
const (
	RoleAdmin    = "admin"
	RoleUser     = "user"
	RoleGuardian = "guardian"
)

// tablePlatformAnnouncements is the schema-qualified table name
const tablePlatformAnnouncements = "platform.announcements"

// Announcement represents a platform announcement or release note
type Announcement struct {
	base.Model  `bun:"schema:platform,table:announcements"`
	Title       string     `bun:"title,notnull" json:"title"`
	Content     string     `bun:"content,notnull" json:"content"`
	Type        string     `bun:"type,notnull,default:'announcement'" json:"type"`
	Severity    string     `bun:"severity,notnull,default:'info'" json:"severity"`
	Version     *string    `bun:"version" json:"version,omitempty"`
	Active      bool       `bun:"active,notnull,default:true" json:"active"`
	PublishedAt *time.Time `bun:"published_at" json:"published_at,omitempty"`
	ExpiresAt   *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
	TargetRoles []string   `bun:"target_roles,array" json:"target_roles,omitempty"`
	CreatedBy   int64      `bun:"created_by,notnull" json:"created_by"`

	// Relations
	Creator *Operator `bun:"rel:belongs-to,join:created_by=id" json:"creator,omitempty"`
}

// BeforeAppendModel is intentionally removed to allow repository methods
// to set custom table expressions with aliases (e.g., for JOIN queries).
// The bun tag `bun:"schema:platform,table:announcements"` handles the
// default table name, and repositories use ModelTableExpr when needed.

// TableName returns the database table name
func (a *Announcement) TableName() string {
	return tablePlatformAnnouncements
}

// Validate ensures announcement data is valid
func (a *Announcement) Validate() error {
	a.Title = strings.TrimSpace(a.Title)
	a.Content = strings.TrimSpace(a.Content)

	if a.Title == "" {
		return errors.New("title is required")
	}
	if len(a.Title) > 200 {
		return errors.New("title must not exceed 200 characters")
	}
	if a.Content == "" {
		return errors.New("content is required")
	}
	if !IsValidAnnouncementType(a.Type) {
		return errors.New("invalid announcement type")
	}
	if !IsValidSeverity(a.Severity) {
		return errors.New("invalid severity")
	}
	if a.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	if a.Version != nil && len(*a.Version) > 50 {
		return errors.New("version must not exceed 50 characters")
	}
	return nil
}

// IsValidAnnouncementType checks if a type string is valid
func IsValidAnnouncementType(t string) bool {
	switch t {
	case TypeAnnouncement, TypeRelease, TypeMaintenance:
		return true
	default:
		return false
	}
}

// IsValidSeverity checks if a severity string is valid
func IsValidSeverity(s string) bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	default:
		return false
	}
}

// IsPublished returns true if the announcement has been published
func (a *Announcement) IsPublished() bool {
	return a.PublishedAt != nil && a.PublishedAt.Before(time.Now())
}

// IsExpired returns true if the announcement has expired
func (a *Announcement) IsExpired() bool {
	return a.ExpiresAt != nil && a.ExpiresAt.Before(time.Now())
}

// IsDraft returns true if the announcement is a draft (not published)
func (a *Announcement) IsDraft() bool {
	return a.PublishedAt == nil
}

// GetID returns the entity's ID
func (a *Announcement) GetID() any {
	return a.ID
}

// GetCreatedAt returns the creation timestamp
func (a *Announcement) GetCreatedAt() time.Time {
	return a.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (a *Announcement) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}

// AnnouncementStats contains view statistics for an announcement
type AnnouncementStats struct {
	AnnouncementID int64 `json:"announcement_id"`
	TargetCount    int   `json:"target_count"`    // Total users who can see it
	SeenCount      int   `json:"seen_count"`      // Users who saw it
	DismissedCount int   `json:"dismissed_count"` // Users who dismissed it
}

// AnnouncementViewDetail contains info about a single user view
type AnnouncementViewDetail struct {
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"`
	SeenAt    time.Time `json:"seen_at"`
	Dismissed bool      `json:"dismissed"`
}
