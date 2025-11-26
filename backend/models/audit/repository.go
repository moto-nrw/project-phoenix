package audit

import (
	"context"
	"time"
)

// DataDeletionRepository defines operations for managing data deletion audit records
type DataDeletionRepository interface {
	Create(ctx context.Context, deletion *DataDeletion) error
	FindByID(ctx context.Context, id interface{}) (*DataDeletion, error)
	FindByStudentID(ctx context.Context, studentID int64) ([]*DataDeletion, error)
	FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*DataDeletion, error)
	FindByType(ctx context.Context, deletionType string) ([]*DataDeletion, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*DataDeletion, error)

	// Statistics methods
	GetDeletionStats(ctx context.Context, startDate time.Time) (map[string]interface{}, error)
	CountByType(ctx context.Context, deletionType string, since time.Time) (int64, error)
}

// AuthEventRepository defines operations for managing authentication event audit records
type AuthEventRepository interface {
	Create(ctx context.Context, event *AuthEvent) error
	FindByID(ctx context.Context, id interface{}) (*AuthEvent, error)
	FindByAccountID(ctx context.Context, accountID int64, limit int) ([]*AuthEvent, error)
	FindByEventType(ctx context.Context, eventType string, since time.Time) ([]*AuthEvent, error)
	FindFailedAttempts(ctx context.Context, accountID int64, since time.Time) ([]*AuthEvent, error)
	CountFailedAttempts(ctx context.Context, accountID int64, since time.Time) (int, error)
	CleanupOldEvents(ctx context.Context, olderThan time.Duration) (int, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*AuthEvent, error)
}

// DataImportRepository defines operations for managing data import audit records
type DataImportRepository interface {
	Create(ctx context.Context, dataImport *DataImport) error
	FindByID(ctx context.Context, id int64) (*DataImport, error)
	FindByImportedBy(ctx context.Context, accountID int64, limit int) ([]*DataImport, error)
	FindByEntityType(ctx context.Context, entityType string, limit int) ([]*DataImport, error)
	FindRecent(ctx context.Context, limit int) ([]*DataImport, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*DataImport, error)
}
