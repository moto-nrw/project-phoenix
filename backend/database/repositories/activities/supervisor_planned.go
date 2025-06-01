// backend/database/repositories/activities/supervisor_planned.go
package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
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

// FindByID overrides the base repository method to fix the alias issue
func (r *SupervisorPlannedRepository) FindByID(ctx context.Context, id interface{}) (*activities.SupervisorPlanned, error) {
	var supervisor activities.SupervisorPlanned
	
	// Extract transaction from context if it exists
	db := r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = *tx
	}
	
	// Use the same alias as base repository: "supervisorplanned" (lowercase)
	err := db.NewSelect().
		Model(&supervisor).
		ModelTableExpr(`activities.supervisors AS "supervisorplanned"`).
		Where(`"supervisorplanned".id = ?`, id).
		Scan(ctx)
	
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}
	
	return &supervisor, nil
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
	type supervisorResult struct {
		Supervisor *activities.SupervisorPlanned `bun:"supervisor"`
		Staff      *users.Staff                  `bun:"staff"`
		Person     *users.Person                 `bun:"person"`
	}

	var results []supervisorResult

	// Use explicit joins with schema qualification
	err := r.db.NewSelect().
		Model(&results).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		// Explicit column mapping for each table
		ColumnExpr(`"supervisor".id AS "supervisor__id"`).
		ColumnExpr(`"supervisor".created_at AS "supervisor__created_at"`).
		ColumnExpr(`"supervisor".updated_at AS "supervisor__updated_at"`).
		ColumnExpr(`"supervisor".staff_id AS "supervisor__staff_id"`).
		ColumnExpr(`"supervisor".group_id AS "supervisor__group_id"`).
		ColumnExpr(`"supervisor".is_primary AS "supervisor__is_primary"`).
		ColumnExpr(`"staff".id AS "staff__id"`).
		ColumnExpr(`"staff".created_at AS "staff__created_at"`).
		ColumnExpr(`"staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id"`).
		ColumnExpr(`"staff".staff_notes AS "staff__staff_notes"`).
		ColumnExpr(`"person".id AS "person__id"`).
		ColumnExpr(`"person".created_at AS "person__created_at"`).
		ColumnExpr(`"person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name"`).
		ColumnExpr(`"person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id"`).
		ColumnExpr(`"person".account_id AS "person__account_id"`).
		// Properly schema-qualified joins
		Join(`LEFT JOIN users.staff AS "staff" ON "staff".id = "supervisor".staff_id`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		// Filter by group ID
		Where(`"supervisor".group_id = ?`, groupID).
		Order("supervisor.is_primary DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	// Convert results to SupervisorPlanned objects
	supervisors := make([]*activities.SupervisorPlanned, len(results))
	for i, result := range results {
		supervisors[i] = result.Supervisor
		supervisors[i].Staff = result.Staff
		if result.Staff != nil {
			result.Staff.Person = result.Person
		}
	}

	return supervisors, nil
}

// FindPrimaryByGroupID finds the primary supervisor for a specific group
func (r *SupervisorPlannedRepository) FindPrimaryByGroupID(ctx context.Context, groupID int64) (*activities.SupervisorPlanned, error) {
	type supervisorResult struct {
		Supervisor *activities.SupervisorPlanned `bun:"supervisor"`
		Staff      *users.Staff                  `bun:"staff"`
		Person     *users.Person                 `bun:"person"`
	}

	var result supervisorResult

	// Use explicit joins with schema qualification
	err := r.db.NewSelect().
		Model(&result).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		// Explicit column mapping for each table
		ColumnExpr(`"supervisor".id AS "supervisor__id"`).
		ColumnExpr(`"supervisor".created_at AS "supervisor__created_at"`).
		ColumnExpr(`"supervisor".updated_at AS "supervisor__updated_at"`).
		ColumnExpr(`"supervisor".staff_id AS "supervisor__staff_id"`).
		ColumnExpr(`"supervisor".group_id AS "supervisor__group_id"`).
		ColumnExpr(`"supervisor".is_primary AS "supervisor__is_primary"`).
		ColumnExpr(`"staff".id AS "staff__id"`).
		ColumnExpr(`"staff".created_at AS "staff__created_at"`).
		ColumnExpr(`"staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id"`).
		ColumnExpr(`"staff".staff_notes AS "staff__staff_notes"`).
		ColumnExpr(`"person".id AS "person__id"`).
		ColumnExpr(`"person".created_at AS "person__created_at"`).
		ColumnExpr(`"person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name"`).
		ColumnExpr(`"person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id"`).
		ColumnExpr(`"person".account_id AS "person__account_id"`).
		// Properly schema-qualified joins
		Join(`LEFT JOIN users.staff AS "staff" ON "staff".id = "supervisor".staff_id`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
		// Filter by group ID and primary status
		Where(`"supervisor".group_id = ? AND "supervisor".is_primary = true`, groupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find primary by group ID",
			Err: err,
		}
	}

	// Setup relations correctly
	if result.Supervisor != nil {
		result.Supervisor.Staff = result.Staff
		if result.Staff != nil {
			result.Staff.Person = result.Person
		}
		return result.Supervisor, nil
	}

	return nil, &modelBase.DatabaseError{
		Op:  "find primary by group ID",
		Err: errors.New("no primary supervisor found"),
	}
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

	// Extract transaction from context if it exists
	db := r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = *tx
	}

	// Get the query builder
	query := db.NewUpdate().
		Model(supervisor).
		Where("id = ?", supervisor.ID).
		ModelTableExpr("activities.supervisors")

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

// Delete overrides the base Delete method to handle transactions
func (r *SupervisorPlannedRepository) Delete(ctx context.Context, id interface{}) error {
	// Extract transaction from context if it exists
	db := r.db
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		db = *tx
	}

	// Use the same alias as base repository: "supervisorplanned" (lowercase)
	_, err := db.NewDelete().
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(`activities.supervisors AS "supervisorplanned"`).
		Where(`"supervisorplanned".id = ?`, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
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
