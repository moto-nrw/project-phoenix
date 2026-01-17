package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	activePort "github.com/moto-nrw/project-phoenix/internal/core/port/active"
	"github.com/uptrace/bun"
)

// GroupSupervisorRepository implements active.GroupSupervisorRepository interface
type GroupSupervisorRepository struct {
	*base.Repository[*active.GroupSupervisor]
	db *bun.DB
}

// NewGroupSupervisorRepository creates a new GroupSupervisorRepository
func NewGroupSupervisorRepository(db *bun.DB) activePort.GroupSupervisorRepository {
	return &GroupSupervisorRepository{
		Repository: base.NewRepository[*active.GroupSupervisor](db, "active.group_supervisors", "GroupSupervisor"),
		db:         db,
	}
}

// FindActiveByStaffID finds all active supervisions for a specific staff member
func (r *GroupSupervisorRepository) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	var supervisions []*active.GroupSupervisor
	err := r.db.NewSelect().
		Model(&supervisions).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where("staff_id = ? AND (end_date IS NULL OR end_date >= CURRENT_DATE)", staffID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by staff ID",
			Err: err,
		}
	}

	return supervisions, nil
}

// FindByActiveGroupID finds supervisors for a specific active group
// If activeOnly is true, only returns supervisors with end_date IS NULL (currently active)
// Includes Staff.Person relation for staff name display
func (r *GroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	var supervisions []*active.GroupSupervisor
	query := r.db.NewSelect().
		Model(&supervisions).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		// Use explicit JOINs for schema-qualified tables (Relation() doesn't handle cross-schema properly)
		ColumnExpr(`"group_supervisor".*`).
		ColumnExpr(`"staff"."id" AS "staff__id", "staff"."person_id" AS "staff__person_id", "staff"."staff_notes" AS "staff__staff_notes"`).
		ColumnExpr(`"person"."id" AS "staff__person__id", "person"."first_name" AS "staff__person__first_name", "person"."last_name" AS "staff__person__last_name"`).
		Join(`LEFT JOIN users.staff AS "staff" ON "staff"."id" = "group_supervisor"."staff_id"`).
		Join(`LEFT JOIN users.persons AS "person" ON "person"."id" = "staff"."person_id"`).
		Where(`"group_supervisor".group_id = ?`, activeGroupID)

	if activeOnly {
		query = query.Where(`"group_supervisor".end_date IS NULL`)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group ID",
			Err: err,
		}
	}

	return supervisions, nil
}

// FindByActiveGroupIDs finds supervisors for multiple active groups in a single query
// If activeOnly is true, only returns supervisors with end_date IS NULL (currently active)
// Includes Staff.Person relation for staff name display
func (r *GroupSupervisorRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	if len(activeGroupIDs) == 0 {
		return []*active.GroupSupervisor{}, nil
	}

	var supervisions []*active.GroupSupervisor
	query := r.db.NewSelect().
		Model(&supervisions).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		// Use explicit JOINs for schema-qualified tables (Relation() doesn't handle cross-schema properly)
		ColumnExpr(`"group_supervisor".*`).
		ColumnExpr(`"staff"."id" AS "staff__id", "staff"."person_id" AS "staff__person_id", "staff"."staff_notes" AS "staff__staff_notes"`).
		ColumnExpr(`"person"."id" AS "staff__person__id", "person"."first_name" AS "staff__person__first_name", "person"."last_name" AS "staff__person__last_name"`).
		Join(`LEFT JOIN users.staff AS "staff" ON "staff"."id" = "group_supervisor"."staff_id"`).
		Join(`LEFT JOIN users.persons AS "person" ON "person"."id" = "staff"."person_id"`).
		Where(`"group_supervisor".group_id IN (?)`, bun.In(activeGroupIDs))

	if activeOnly {
		query = query.Where(`"group_supervisor".end_date IS NULL`)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group IDs",
			Err: err,
		}
	}

	return supervisions, nil
}

// EndSupervision marks a supervision as ended at the current date
func (r *GroupSupervisorRepository) EndSupervision(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*active.GroupSupervisor)(nil)).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Set("end_date = ?", time.Now()).
		Where(`"group_supervisor".id = ? AND "group_supervisor".end_date IS NULL`, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end supervision",
			Err: err,
		}
	}

	return nil
}

// Create overrides base Create to handle validation
func (r *GroupSupervisorRepository) Create(ctx context.Context, supervision *active.GroupSupervisor) error {
	if supervision == nil {
		return fmt.Errorf("group supervisor cannot be nil")
	}

	// Validate supervision
	if err := supervision.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, supervision)
}

// Update overrides base Update to handle schema-qualified tables
func (r *GroupSupervisorRepository) Update(ctx context.Context, supervision *active.GroupSupervisor) error {
	if supervision == nil {
		return fmt.Errorf("group supervisor cannot be nil")
	}

	// Validate supervision
	if err := supervision.Validate(); err != nil {
		return err
	}

	// Check if we have a transaction in context
	var db bun.IDB = r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	// Perform the update with proper table expression
	_, err := db.NewUpdate().
		Model(supervision).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		WherePK().
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *GroupSupervisorRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.GroupSupervisor, error) {
	var supervisions []*active.GroupSupervisor
	query := r.db.NewSelect().
		Model(&supervisions).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`)

	// Apply query options with table alias
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("group_supervisor")
		}
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return supervisions, nil
}

// FindWithStaff retrieves supervisions with staff details
func (r *GroupSupervisorRepository) FindWithStaff(ctx context.Context, id int64) (*active.GroupSupervisor, error) {
	supervision := new(active.GroupSupervisor)
	err := r.db.NewSelect().
		Model(supervision).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Relation("Staff").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff",
			Err: err,
		}
	}

	return supervision, nil
}

// FindWithActiveGroup retrieves supervisions with active group details
func (r *GroupSupervisorRepository) FindWithActiveGroup(ctx context.Context, id int64) (*active.GroupSupervisor, error) {
	supervision := new(active.GroupSupervisor)
	err := r.db.NewSelect().
		Model(supervision).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Relation("ActiveGroup").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with active group",
			Err: err,
		}
	}

	return supervision, nil
}
