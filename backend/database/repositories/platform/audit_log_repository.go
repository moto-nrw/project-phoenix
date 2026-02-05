package platform

import (
	"context"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

// Table and query constants
const (
	tablePlatformOperatorAuditLog      = "platform.operator_audit_log"
	tablePlatformOperatorAuditLogAlias = `platform.operator_audit_log AS "log"`
)

// OperatorAuditLogRepository implements platform.OperatorAuditLogRepository interface
type OperatorAuditLogRepository struct {
	db *bun.DB
}

// NewOperatorAuditLogRepository creates a new OperatorAuditLogRepository
func NewOperatorAuditLogRepository(db *bun.DB) platform.OperatorAuditLogRepository {
	return &OperatorAuditLogRepository{db: db}
}

// Create inserts a new audit log entry
func (r *OperatorAuditLogRepository) Create(ctx context.Context, entry *platform.OperatorAuditLog) error {
	_, err := r.db.NewInsert().
		Model(entry).
		ModelTableExpr(tablePlatformOperatorAuditLog).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create audit log entry",
			Err: err,
		}
	}

	return nil
}

// FindByOperatorID retrieves audit logs by operator ID
func (r *OperatorAuditLogRepository) FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*platform.OperatorAuditLog, error) {
	var entries []*platform.OperatorAuditLog
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(tablePlatformOperatorAuditLogAlias).
		Where(`"log".operator_id = ?`, operatorID).
		Order(`"log".created_at DESC`)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find audit logs by operator id",
			Err: err,
		}
	}

	return entries, nil
}

// FindByResourceType retrieves audit logs by resource type
func (r *OperatorAuditLogRepository) FindByResourceType(ctx context.Context, resourceType string, limit int) ([]*platform.OperatorAuditLog, error) {
	var entries []*platform.OperatorAuditLog
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(tablePlatformOperatorAuditLogAlias).
		Where(`"log".resource_type = ?`, resourceType).
		Order(`"log".created_at DESC`)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find audit logs by resource type",
			Err: err,
		}
	}

	return entries, nil
}

// FindByDateRange retrieves audit logs within a date range
func (r *OperatorAuditLogRepository) FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*platform.OperatorAuditLog, error) {
	var entries []*platform.OperatorAuditLog
	query := r.db.NewSelect().
		Model(&entries).
		ModelTableExpr(tablePlatformOperatorAuditLogAlias).
		Where(`"log".created_at >= ?`, start).
		Where(`"log".created_at <= ?`, end).
		Order(`"log".created_at DESC`)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find audit logs by date range",
			Err: err,
		}
	}

	return entries, nil
}
