package audit

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/uptrace/bun"
)

// SQL clause constants
const (
	orderByDeletedAtDesc    = "deleted_at DESC"
	whereDeletionTypeEquals = "deletion_type = ?"
)

// DataDeletionRepository implements audit.DataDeletionRepository interface
type DataDeletionRepository struct {
	runtime Runtime
}

// NewDataDeletionRepository creates a new DataDeletionRepository
func NewDataDeletionRepository(runtime Runtime) *DataDeletionRepository {
	return &DataDeletionRepository{runtime: requireRuntime(runtime)}
}

// Create overrides base Create to handle validation
func (r *DataDeletionRepository) Create(ctx context.Context, deletion *audit.DataDeletion) error {
	if deletion == nil {
		return errors.New("data deletion cannot be nil")
	}
	return NewAppender(r.runtime).Append(ctx, deletion)
}

func (r *DataDeletionRepository) FindByID(ctx context.Context, id interface{}) (*audit.DataDeletion, error) {
	var deletion audit.DataDeletion
	query := runtimeDB(ctx, r.runtime).NewSelect().Model(&deletion).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where(`"data_deletion".id = ?`, id)
	query = withDataDeletionTenant(ctx, r.runtime, query)
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("data deletion not found")
		}
		return nil, wrapDatabase("find data deletion", err)
	}
	return &deletion, nil
}

// FindByStudentID finds all deletion records for a specific student
func (r *DataDeletionRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*audit.DataDeletion, error) {
	var deletions []*audit.DataDeletion
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&deletions).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where("student_id = ?", studentID)

	query = withDataDeletionTenant(ctx, r.runtime, query)

	err := query.Order(orderByDeletedAtDesc).Scan(ctx)

	if err != nil {
		return nil, wrapDatabase("find data deletions by student ID", err)
	}

	return deletions, nil
}

// FindByDateRange finds all deletion records within a date range
func (r *DataDeletionRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*audit.DataDeletion, error) {
	var deletions []*audit.DataDeletion
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&deletions).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where("deleted_at >= ?", startDate).
		Where("deleted_at <= ?", endDate)

	query = withDataDeletionTenant(ctx, r.runtime, query)

	err := query.Order(orderByDeletedAtDesc).Scan(ctx)

	if err != nil {
		return nil, wrapDatabase("find data deletions by date range", err)
	}

	return deletions, nil
}

// FindByType finds all deletion records of a specific type
func (r *DataDeletionRepository) FindByType(ctx context.Context, deletionType string) ([]*audit.DataDeletion, error) {
	var deletions []*audit.DataDeletion
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&deletions).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where(whereDeletionTypeEquals, deletionType)

	query = withDataDeletionTenant(ctx, r.runtime, query)

	err := query.Order(orderByDeletedAtDesc).Scan(ctx)

	if err != nil {
		return nil, wrapDatabase("find data deletions by type", err)
	}

	return deletions, nil
}

// List overrides the base List method to apply proper filtering
func (r *DataDeletionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*audit.DataDeletion, error) {
	var deletions []*audit.DataDeletion
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&deletions).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`)

	query = withDataDeletionTenant(ctx, r.runtime, query)

	query = query.Order(orderByDeletedAtDesc)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "student_id":
				query = query.Where("student_id = ?", value)
			case "deletion_type":
				query = query.Where(whereDeletionTypeEquals, value)
			case "deleted_by":
				query = query.Where("deleted_by = ?", value)
			default:
				query = query.Where("? = ?", bun.Ident(field), value)
			}
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, wrapDatabase("list data deletions", err)
	}

	return deletions, nil
}

func (r *DataDeletionRepository) ListRecentRetentionSummaries(
	ctx context.Context,
	since time.Time,
	limit int,
) ([]audit.RecentDeletionSummary, error) {
	if limit <= 0 {
		return nil, errors.New("recent deletion summary limit must be positive")
	}
	var summaries []audit.RecentDeletionSummary
	query := runtimeDB(ctx, r.runtime).NewSelect().
		TableExpr(`audit.data_deletions AS "data_deletion"`).
		ColumnExpr(`TO_CHAR("data_deletion".deleted_at, 'YYYY-MM-DD') AS date`).
		ColumnExpr(`SUM("data_deletion".records_deleted) AS records_deleted`).
		ColumnExpr(`COUNT(DISTINCT "data_deletion".student_id) AS student_count`).
		Where(`"data_deletion".deletion_type = ?`, audit.DeletionTypeVisitRetention).
		Where(`"data_deletion".deleted_at >= ?`, since).
		GroupExpr(`TO_CHAR("data_deletion".deleted_at, 'YYYY-MM-DD')`).
		OrderExpr(`date DESC`).
		Limit(limit)
	query = withDataDeletionTenant(ctx, r.runtime, query)
	if err := query.Scan(ctx, &summaries); err != nil {
		return nil, wrapDatabase("list recent retention deletion summaries", err)
	}
	return summaries, nil
}

func withDataDeletionTenant(ctx context.Context, runtime Runtime, query *bun.SelectQuery) *bun.SelectQuery {
	if tenantID := runtimeTenantID(ctx, runtime); tenantID > 0 {
		return query.Where(`"data_deletion".tenant_id = ?`, tenantID)
	}
	return query
}
