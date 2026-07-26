package audit

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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

	query = base.WithTenantFilter(ctx, query, "data_deletion")

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

	query = base.WithTenantFilter(ctx, query, "data_deletion")

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

	query = base.WithTenantFilter(ctx, query, "data_deletion")

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

	query = base.WithTenantFilter(ctx, query, "data_deletion")

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
