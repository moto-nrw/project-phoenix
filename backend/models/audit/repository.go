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
}

// AuthEventRepository defines operations for managing authentication event audit records
type AuthEventRepository interface {
	Create(ctx context.Context, event *AuthEvent) error
	FindByID(ctx context.Context, id interface{}) (*AuthEvent, error)
	FindByAccountID(ctx context.Context, accountID int64, limit int) ([]*AuthEvent, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*AuthEvent, error)
	ListPendingAccountWideWipeAccountIDs(ctx context.Context, since time.Time) ([]int64, error)
}

// DataAccessLogRepository is an insert-only repository for audit.data_access_log.
// Access-log rows are append-only and are never updated or deleted by the application.
type DataAccessLogRepository interface {
	Create(ctx context.Context, entry *DataAccessLog) error
	// ExistsSince reports whether the actor already has a row of resourceType,
	// accessed at or after since, whose metadata matches every given
	// key/value (text comparison). Lets polling readers collapse identical
	// re-reads into one evidential row instead of flooding the append-only
	// log with duplicates.
	ExistsSince(ctx context.Context, actorAccountID int64, resourceType string, metadata map[string]string, since time.Time) (bool, error)
}

type UnregisteredTagScanRepository interface {
	Create(ctx context.Context, scan *UnregisteredTagScan) error
	FindByID(ctx context.Context, id int64) (*UnregisteredTagScan, error)
	ListForOperator(ctx context.Context, filter UnregisteredTagScanFilter) ([]*UnregisteredTagScan, error)
	Resolve(ctx context.Context, id, operatorID int64, note *string) (*UnregisteredTagScan, error)
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int, error)
}

// DataImportRepository defines operations for managing data import audit records
type DataImportRepository interface {
	Create(ctx context.Context, dataImport *DataImport) error
	FindByID(ctx context.Context, id int64) (*DataImport, error)
	List(ctx context.Context, filters map[string]interface{}) ([]*DataImport, error)
}
