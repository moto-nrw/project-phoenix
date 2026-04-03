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
	FindByIDForUpdate(ctx context.Context, id int64) (*Operator, error)
	FindByEmail(ctx context.Context, email string) (*Operator, error)
	Update(ctx context.Context, operator *Operator) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]*Operator, error)

	// Auth operations
	UpdateLastLogin(ctx context.Context, id int64) error
}

// OrganizationRepository defines operations for managing organizations.
type OrganizationRepository interface {
	Create(ctx context.Context, organization *Organization) error
	FindByID(ctx context.Context, id int64) (*Organization, error)
	FindBySlug(ctx context.Context, slug string) (*Organization, error)
	List(ctx context.Context) ([]*Organization, error)
	Update(ctx context.Context, organization *Organization) error
	CountByIDs(ctx context.Context, ids []int64) (int, error)
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

	// Query unread announcements for a user scoped to the current session tenant/org
	GetUnreadForUser(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) ([]*Announcement, error)

	// Count unread announcements for a user scoped to the current session tenant/org
	CountUnread(ctx context.Context, userID int64, userRoles []string, tenantID int64, orgID int64) (int, error)

	// Check if user has seen announcement
	HasSeen(ctx context.Context, userID, announcementID int64) (bool, error)

	// Get view statistics for an announcement
	GetStats(ctx context.Context, announcementID int64) (*AnnouncementStats, error)

	// Get detailed view list for an announcement (who has seen/dismissed)
	GetViewDetails(ctx context.Context, announcementID int64) ([]*AnnouncementViewDetail, error)
}

// SchoolRepository defines operations for school (tenant) records.
type SchoolRepository interface {
	Create(ctx context.Context, school *School) error
	FindByID(ctx context.Context, id int64) (*School, error)
	FindByIDForShare(ctx context.Context, id int64) (*School, error)
	FindBySlug(ctx context.Context, slug string) (*School, error)
	FindByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (*School, error)
	FindBySubdomain(ctx context.Context, subdomain string) (*School, error)
	List(ctx context.Context) ([]*School, error)
	ListActive(ctx context.Context) ([]School, error)
	ListPublic(ctx context.Context) ([]School, error)
	FindActiveByAccountID(ctx context.Context, accountID int64) ([]School, error)
	Update(ctx context.Context, school *School) error
	SoftDelete(ctx context.Context, id int64) error
	Restore(ctx context.Context, id int64) error
	CountByIDs(ctx context.Context, ids []int64) (int, error)
}

// OperatorInvitationTokenRepository defines operations for operator invitation tokens
type OperatorInvitationTokenRepository interface {
	Create(ctx context.Context, token *OperatorInvitationToken) error
	FindByID(ctx context.Context, id int64) (*OperatorInvitationToken, error)
	FindValidByToken(ctx context.Context, tokenStr string) (*OperatorInvitationToken, error)
	ConsumeByToken(ctx context.Context, tokenStr string) (*OperatorInvitationToken, error)
	InvalidateByEmail(ctx context.Context, email string) error
	UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	ListPending(ctx context.Context) ([]*OperatorInvitationToken, error)
	InvalidateExpiredTokens(ctx context.Context) (int, error)
	DeleteStaleTokens(ctx context.Context) (int, error)
}

// OperatorEmailChangeTokenRepository defines operations for email change verification tokens
type OperatorEmailChangeTokenRepository interface {
	Create(ctx context.Context, token *OperatorEmailChangeToken) error
	ConsumeByToken(ctx context.Context, tokenStr string) (*OperatorEmailChangeToken, error)
	InvalidateByOperatorID(ctx context.Context, operatorID int64) error
	UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	CountRecentByOperatorID(ctx context.Context, operatorID int64, since time.Time) (int, error)
	InvalidateExpiredTokens(ctx context.Context) (int, error)
	DeleteStaleTokens(ctx context.Context) (int, error)
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
