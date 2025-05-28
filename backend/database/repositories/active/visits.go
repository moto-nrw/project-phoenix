// backend/database/repositories/active/visit.go
package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// VisitRepository implements active.VisitRepository interface
type VisitRepository struct {
	*base.Repository[*active.Visit]
	db *bun.DB
}

// NewVisitRepository creates a new VisitRepository
func NewVisitRepository(db *bun.DB) active.VisitRepository {
	return &VisitRepository{
		Repository: base.NewRepository[*active.Visit](db, "active.visits", "Visit"),
		db:         db,
	}
}

// FindActiveByStudentID finds all active visits for a specific student
func (r *VisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".student_id = ? AND "visit".exit_time IS NULL`, studentID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by student ID",
			Err: err,
		}
	}

	return visits, nil
}

// FindByActiveGroupID finds all visits for a specific active group
func (r *VisitRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".active_group_id = ?`, activeGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group ID",
			Err: err,
		}
	}

	return visits, nil
}

// FindByTimeRange finds all visits active during a specific time range
func (r *VisitRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".entry_time <= ? AND ("visit".exit_time IS NULL OR "visit".exit_time >= ?)`, end, start).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: err,
		}
	}

	return visits, nil
}

// EndVisit marks a visit as ended at the current time
func (r *VisitRepository) EndVisit(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*active.Visit)(nil)).
		ModelTableExpr(`active.visits AS "visit"`).
		Set(`"visit".exit_time = ?`, time.Now()).
		Where(`"visit".id = ? AND "visit".exit_time IS NULL`, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end visit",
			Err: err,
		}
	}

	return nil
}

// Create overrides base Create to handle validation
func (r *VisitRepository) Create(ctx context.Context, visit *active.Visit) error {
	if visit == nil {
		return fmt.Errorf("visit cannot be nil")
	}

	// Validate visit
	if err := visit.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, visit)
}

// List overrides the base List method to accept the new QueryOptions type
func (r *VisitRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.Visit, error) {
	var visits []*active.Visit
	query := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return visits, nil
}

// FindWithStudent retrieves visits with student details
func (r *VisitRepository) FindWithStudent(ctx context.Context, id int64) (*active.Visit, error) {
	visit := new(active.Visit)
	err := r.db.NewSelect().
		Model(visit).
		ModelTableExpr(`active.visits AS "visit"`).
		Relation("Student").
		Where(`"visit".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with student",
			Err: err,
		}
	}

	return visit, nil
}

// FindWithActiveGroup retrieves visits with active group details
func (r *VisitRepository) FindWithActiveGroup(ctx context.Context, id int64) (*active.Visit, error) {
	visit := new(active.Visit)
	err := r.db.NewSelect().
		Model(visit).
		ModelTableExpr(`active.visits AS "visit"`).
		Relation("ActiveGroup").
		Where(`"visit".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with active group",
			Err: err,
		}
	}

	return visit, nil
}
