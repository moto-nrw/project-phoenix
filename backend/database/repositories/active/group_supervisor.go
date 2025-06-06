// backend/database/repositories/active/group_supervisor.go
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

// GroupSupervisorRepository implements active.GroupSupervisorRepository interface
type GroupSupervisorRepository struct {
	*base.Repository[*active.GroupSupervisor]
	db *bun.DB
}

// NewGroupSupervisorRepository creates a new GroupSupervisorRepository
func NewGroupSupervisorRepository(db *bun.DB) active.GroupSupervisorRepository {
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

// FindByActiveGroupID finds all supervisors for a specific active group
func (r *GroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupSupervisor, error) {
	var supervisions []*active.GroupSupervisor
	err := r.db.NewSelect().
		Model(&supervisions).
		Where("group_id = ?", activeGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group ID",
			Err: err,
		}
	}

	return supervisions, nil
}

// EndSupervision marks a supervision as ended at the current date
func (r *GroupSupervisorRepository) EndSupervision(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*active.GroupSupervisor)(nil)).
		Set("end_date = ?", time.Now()).
		Where("id = ? AND end_date IS NULL", id).
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
