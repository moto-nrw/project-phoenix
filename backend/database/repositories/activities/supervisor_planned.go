// backend/database/repositories/activities/supervisor_planned.go
package activities

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// SupervisorPlannedRepository implements activities.SupervisorPlannedRepository interface
type SupervisorPlannedRepository struct {
	*base.Repository[*activities.SupervisorPlanned]
	db *bun.DB
}

// NewSupervisorPlannedRepository creates a new SupervisorPlannedRepository
func NewSupervisorPlannedRepository(db *bun.DB) activities.SupervisorPlannedRepository {
	return &SupervisorPlannedRepository{
		Repository: base.NewRepository[*activities.SupervisorPlanned](db, "activities.supervisors", "SupervisorPlanned"),
		db:         db,
	}
}

// FindByStaffID finds all supervisions for a specific staff member
func (r *SupervisorPlannedRepository) FindByStaffID(ctx context.Context, staffID int64) ([]*activities.SupervisorPlanned, error) {
	var supervisors []*activities.SupervisorPlanned
	err := r.db.NewSelect().
		Model(&supervisors).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		Relation("Group").
		Where("staff_id = ?", staffID).
		Order("is_primary DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff ID",
			Err: err,
		}
	}

	return supervisors, nil
}

// FindByGroupID finds all supervisors for a specific group
func (r *SupervisorPlannedRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activities.SupervisorPlanned, error) {
	var supervisors []*activities.SupervisorPlanned
	err := r.db.NewSelect().
		Model(&supervisors).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		Relation("Staff").
		Relation("Staff.Person").
		Where("group_id = ?", groupID).
		Order("is_primary DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	return supervisors, nil
}

// FindPrimaryByGroupID finds the primary supervisor for a specific group
func (r *SupervisorPlannedRepository) FindPrimaryByGroupID(ctx context.Context, groupID int64) (*activities.SupervisorPlanned, error) {
	supervisor := new(activities.SupervisorPlanned)
	err := r.db.NewSelect().
		Model(supervisor).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		Relation("Staff").
		Relation("Staff.Person").
		Where("group_id = ? AND is_primary = true", groupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find primary by group ID",
			Err: err,
		}
	}

	return supervisor, nil
}

// SetPrimary sets a supervisor as the primary supervisor for a group
func (r *SupervisorPlannedRepository) SetPrimary(ctx context.Context, id int64) error {
	// We rely on the database trigger to ensure only one primary supervisor per group
	_, err := r.db.NewUpdate().
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		Set("is_primary = true").
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set primary",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *SupervisorPlannedRepository) Create(ctx context.Context, supervisor *activities.SupervisorPlanned) error {
	if supervisor == nil {
		return fmt.Errorf("supervisor cannot be nil")
	}

	// Validate supervisor
	if err := supervisor.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, supervisor)
}

// Update overrides the base Update method to handle validation
func (r *SupervisorPlannedRepository) Update(ctx context.Context, supervisor *activities.SupervisorPlanned) error {
	if supervisor == nil {
		return fmt.Errorf("supervisor cannot be nil")
	}

	// Validate supervisor
	if err := supervisor.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(supervisor).
		Where("id = ?", supervisor.ID).
		ModelTableExpr("activities.supervisors")

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(supervisor).
			Where("id = ?", supervisor.ID).
			ModelTableExpr("activities.supervisors")
	}

	// Execute the query
	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *SupervisorPlannedRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.SupervisorPlanned, error) {
	var supervisors []*activities.SupervisorPlanned
	query := r.db.NewSelect().Model(&supervisors)

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

	return supervisors, nil
}
