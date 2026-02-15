package audit

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SQL clause constants
const (
	orderByDeletedAtDesc    = "deleted_at DESC"
	whereDeletionTypeEquals = "deletion_type = ?"
)

// DataDeletionRepository implements audit.DataDeletionRepository interface
type DataDeletionRepository struct {
	*base.Repository[*audit.DataDeletion]
	db *bun.DB
}

// NewDataDeletionRepository creates a new DataDeletionRepository
func NewDataDeletionRepository(db *bun.DB) audit.DataDeletionRepository {
	repo := base.NewRepository[*audit.DataDeletion](db, "audit.data_deletions", "DataDeletion")
	repo.TenantScoped = true
	return &DataDeletionRepository{
		Repository: repo,
		db:         db,
	}
}

// Create overrides base Create to handle validation
func (r *DataDeletionRepository) Create(ctx context.Context, deletion *audit.DataDeletion) error {
	if deletion == nil {
		return &modelBase.DatabaseError{
			Op:  "create",
			Err: errors.New("deletion cannot be nil"),
		}
	}

	// Validate the deletion record
	if err := deletion.Validate(); err != nil {
		return &modelBase.DatabaseError{
			Op:  "validate",
			Err: err,
		}
	}

	// Use the base Create method
	return r.Repository.Create(ctx, deletion)
}

// FindByStudentID finds all deletion records for a specific student
func (r *DataDeletionRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*audit.DataDeletion, error) {
	var deletions []*audit.DataDeletion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&deletions).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where("student_id = ?", studentID)

	if where, val, ok := base.TenantWhere(ctx, "data_deletion"); ok {
		query = query.Where(where, val)
	}

	err := query.Order(orderByDeletedAtDesc).Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: err,
		}
	}

	return deletions, nil
}

// FindByDateRange finds all deletion records within a date range
func (r *DataDeletionRepository) FindByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*audit.DataDeletion, error) {
	var deletions []*audit.DataDeletion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&deletions).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where("deleted_at >= ?", startDate).
		Where("deleted_at <= ?", endDate)

	if where, val, ok := base.TenantWhere(ctx, "data_deletion"); ok {
		query = query.Where(where, val)
	}

	err := query.Order(orderByDeletedAtDesc).Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by date range",
			Err: err,
		}
	}

	return deletions, nil
}

// FindByType finds all deletion records of a specific type
func (r *DataDeletionRepository) FindByType(ctx context.Context, deletionType string) ([]*audit.DataDeletion, error) {
	var deletions []*audit.DataDeletion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&deletions).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where(whereDeletionTypeEquals, deletionType)

	if where, val, ok := base.TenantWhere(ctx, "data_deletion"); ok {
		query = query.Where(where, val)
	}

	err := query.Order(orderByDeletedAtDesc).Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by type",
			Err: err,
		}
	}

	return deletions, nil
}

// List overrides the base List method to apply proper filtering
func (r *DataDeletionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*audit.DataDeletion, error) {
	var deletions []*audit.DataDeletion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&deletions).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`)

	if where, val, ok := base.TenantWhere(ctx, "data_deletion"); ok {
		query = query.Where(where, val)
	}

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
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return deletions, nil
}

// GetDeletionStats returns aggregated statistics about data deletions
func (r *DataDeletionRepository) GetDeletionStats(ctx context.Context, startDate time.Time) (map[string]interface{}, error) {
	type deletionStats struct {
		TotalDeletions      int64 `bun:"total_deletions"`
		TotalRecordsDeleted int64 `bun:"total_records_deleted"`
		UniqueStudents      int64 `bun:"unique_students"`
	}

	tenantID := tenant.FromContext(ctx)

	var stats deletionStats
	statsQuery := `
		SELECT
			COUNT(*) as total_deletions,
			COALESCE(SUM(records_deleted), 0) as total_records_deleted,
			COUNT(DISTINCT student_id) as unique_students
		FROM audit.data_deletions
		WHERE deleted_at >= ?`

	var err error
	if tenantID > 0 {
		statsQuery += ` AND tenant_id = ?`
		err = base.GetDB(ctx, r.db).NewRaw(statsQuery, startDate, tenantID).Scan(ctx, &stats)
	} else {
		err = base.GetDB(ctx, r.db).NewRaw(statsQuery, startDate).Scan(ctx, &stats)
	}

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get deletion stats",
			Err: err,
		}
	}

	// Get breakdown by type
	type typeStats struct {
		DeletionType   string `bun:"deletion_type"`
		Count          int64  `bun:"count"`
		RecordsDeleted int64  `bun:"records_deleted"`
	}

	var typeBreakdown []typeStats
	typeQuery := `
		SELECT
			deletion_type,
			COUNT(*) as count,
			COALESCE(SUM(records_deleted), 0) as records_deleted
		FROM audit.data_deletions
		WHERE deleted_at >= ?`

	if tenantID > 0 {
		typeQuery += ` AND tenant_id = ?
		GROUP BY deletion_type`
		err = base.GetDB(ctx, r.db).NewRaw(typeQuery, startDate, tenantID).Scan(ctx, &typeBreakdown)
	} else {
		typeQuery += `
		GROUP BY deletion_type`
		err = base.GetDB(ctx, r.db).NewRaw(typeQuery, startDate).Scan(ctx, &typeBreakdown)
	}

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get type breakdown",
			Err: err,
		}
	}

	// Build result map
	result := map[string]interface{}{
		"total_deletions":       stats.TotalDeletions,
		"total_records_deleted": stats.TotalRecordsDeleted,
		"unique_students":       stats.UniqueStudents,
		"by_type":               typeBreakdown,
	}

	return result, nil
}

// CountByType counts deletion records of a specific type since a given date
func (r *DataDeletionRepository) CountByType(ctx context.Context, deletionType string, since time.Time) (int64, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*audit.DataDeletion)(nil)).
		ModelTableExpr(`audit.data_deletions AS "data_deletion"`).
		Where(whereDeletionTypeEquals, deletionType).
		Where("deleted_at >= ?", since)

	if where, val, ok := base.TenantWhere(ctx, "data_deletion"); ok {
		query = query.Where(where, val)
	}

	count, err := query.Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count by type",
			Err: err,
		}
	}

	return int64(count), nil
}
