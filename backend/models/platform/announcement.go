package platform

import (
	"errors"
	"fmt"
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

// MaxTargetIDs is the upper bound for target_org_ids and target_tenant_ids arrays.
const MaxTargetIDs = 100

// Field length bounds for announcement validation.
const (
	maxTitleLen   = 200
	maxVersionLen = 50
)

// Announcement represents a platform announcement or release note
type Announcement struct {
	base.Model      `bun:"schema:platform,table:announcements"`
	Title           string     `bun:"title,notnull" json:"title"`
	Content         string     `bun:"content,notnull" json:"content"`
	Type            string     `bun:"type,notnull,default:'announcement'" json:"type"`
	Severity        string     `bun:"severity,notnull,default:'info'" json:"severity"`
	Version         *string    `bun:"version" json:"version,omitempty"`
	Active          bool       `bun:"active,notnull,default:true" json:"active"`
	PublishedAt     *time.Time `bun:"published_at" json:"published_at,omitempty"`
	ExpiresAt       *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
	TargetRoles     []string   `bun:"target_roles,array,nullzero,default:'{}'" json:"target_roles,omitempty"`
	TargetOrgIDs    []int64    `bun:"target_org_ids,array,nullzero,default:'{}'" json:"target_org_ids,omitempty"`
	TargetTenantIDs []int64    `bun:"target_tenant_ids,array,nullzero,default:'{}'" json:"target_tenant_ids,omitempty"`
	CreatedBy       int64      `bun:"created_by,notnull" json:"created_by"`

	// Relations
	Creator *Operator `bun:"rel:belongs-to,join:created_by=id" json:"creator,omitempty"`
}

// Validate ensures announcement data is valid
func (a *Announcement) Validate() error {
	a.Title = strings.TrimSpace(a.Title)
	a.Content = strings.TrimSpace(a.Content)

	if a.Title == "" {
		return errors.New("title is required")
	}
	if len(a.Title) > maxTitleLen {
		return fmt.Errorf("title must not exceed %d characters", maxTitleLen)
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
	if a.Version != nil && len(*a.Version) > maxVersionLen {
		return fmt.Errorf("version must not exceed %d characters", maxVersionLen)
	}

	// Normalize nil slices to empty arrays so BUN sends '{}' instead of NULL.
	if a.TargetRoles == nil {
		a.TargetRoles = []string{}
	}
	if a.TargetOrgIDs == nil {
		a.TargetOrgIDs = []int64{}
	}
	if a.TargetTenantIDs == nil {
		a.TargetTenantIDs = []int64{}
	}

	// Cap targeting array sizes to prevent bloated IN clauses and GIN index overhead.
	if len(a.TargetOrgIDs) > MaxTargetIDs {
		return fmt.Errorf("target_org_ids exceeds maximum of %d entries", MaxTargetIDs)
	}
	if len(a.TargetTenantIDs) > MaxTargetIDs {
		return fmt.Errorf("target_tenant_ids exceeds maximum of %d entries", MaxTargetIDs)
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

// IsDraft returns true if the announcement is a draft (not published)
func (a *Announcement) IsDraft() bool {
	return a.PublishedAt == nil
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
	UserID int64 `bun:"user_id" json:"user_id"`
	// AccountEmail is the fallback display name; the composition layer
	// replaces UserName with the person's name through the People Directory.
	AccountEmail string    `bun:"account_email" json:"-"`
	UserName     string    `bun:"user_name" json:"user_name"`
	SeenAt       time.Time `bun:"seen_at" json:"seen_at"`
	Dismissed    bool      `bun:"dismissed" json:"dismissed"`
}
