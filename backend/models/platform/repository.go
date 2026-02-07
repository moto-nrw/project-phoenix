package platform

import (
	"context"
	"time"
)

// OperatorRepository defines operations for managing operators
type OperatorRepository interface {
	// Core CRUD operations
	Create(ctx context.Context, operator *Operator) error
	FindByID(ctx context.Context, id int64) (*Operator, error)
	FindByEmail(ctx context.Context, email string) (*Operator, error)
	Update(ctx context.Context, operator *Operator) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]*Operator, error)

	// Auth operations
	UpdateLastLogin(ctx context.Context, id int64) error
}

// AnnouncementRepository defines operations for managing announcements
type AnnouncementRepository interface {
	// Core CRUD operations
	Create(ctx context.Context, announcement *Announcement) error
	FindByID(ctx context.Context, id int64) (*Announcement, error)
	Update(ctx context.Context, announcement *Announcement) error
	Delete(ctx context.Context, id int64) error

	// Listing operations
	List(ctx context.Context, includeInactive bool) ([]*Announcement, error)
	ListPublished(ctx context.Context) ([]*Announcement, error)

	// Publishing
	Publish(ctx context.Context, id int64) error
	Unpublish(ctx context.Context, id int64) error
}

// AnnouncementViewRepository defines operations for tracking announcement views
type AnnouncementViewRepository interface {
	// Mark as seen/dismissed
	MarkSeen(ctx context.Context, userID, announcementID int64) error
	MarkDismissed(ctx context.Context, userID, announcementID int64) error

	// Query unread announcements for a user (filtered by roles)
	GetUnreadForUser(ctx context.Context, userID int64, userRoles []string) ([]*Announcement, error)

	// Count unread announcements for a user (filtered by roles)
	CountUnread(ctx context.Context, userID int64, userRoles []string) (int, error)

	// Check if user has seen announcement
	HasSeen(ctx context.Context, userID, announcementID int64) (bool, error)

	// Get view statistics for an announcement
	GetStats(ctx context.Context, announcementID int64) (*AnnouncementStats, error)

	// Get detailed view list for an announcement (who has seen/dismissed)
	GetViewDetails(ctx context.Context, announcementID int64) ([]*AnnouncementViewDetail, error)
}

// OperatorAuditLogRepository defines operations for the audit log
type OperatorAuditLogRepository interface {
	// Create a new audit log entry
	Create(ctx context.Context, entry *OperatorAuditLog) error

	// Query audit logs
	FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*OperatorAuditLog, error)
	FindByResourceType(ctx context.Context, resourceType string, limit int) ([]*OperatorAuditLog, error)
	FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*OperatorAuditLog, error)
}
