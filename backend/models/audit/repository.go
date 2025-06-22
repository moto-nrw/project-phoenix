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
