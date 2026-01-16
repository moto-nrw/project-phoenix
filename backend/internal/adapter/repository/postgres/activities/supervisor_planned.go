// backend/database/repositories/activities/supervisor_planned.go
package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	activitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/activities"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableSupervisorPlanned     = "activities.supervisors"
	tableExprSupervisorPlanned = `activities.supervisors AS "supervisor_planned"`
	whereSupervisorIDEquals    = "id = ?"
)

// supervisorResult holds the result of a supervisor query with joined staff and person.
type supervisorResult struct {
	Supervisor *activities.SupervisorPlanned `bun:"supervisor"`
	Staff      *users.Staff                  `bun:"staff"`
	Person     *users.Person                 `bun:"person"`
}

// applySupervisorColumnMapping adds all required column expressions for supervisor queries
// with staff and person joins. This eliminates duplication across FindByGroupID,
// FindByGroupIDs, and FindPrimaryByGroupID.
func applySupervisorColumnMapping(q *bun.SelectQuery) *bun.SelectQuery {
	return q.
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		// Supervisor columns
		ColumnExpr(`"supervisor".id AS "supervisor__id"`).
		ColumnExpr(`"supervisor".created_at AS "supervisor__created_at"`).
		ColumnExpr(`"supervisor".updated_at AS "supervisor__updated_at"`).
		ColumnExpr(`"supervisor".staff_id AS "supervisor__staff_id"`).
		ColumnExpr(`"supervisor".group_id AS "supervisor__group_id"`).
		ColumnExpr(`"supervisor".is_primary AS "supervisor__is_primary"`).
		// Staff columns
		ColumnExpr(`"staff".id AS "staff__id"`).
		ColumnExpr(`"staff".created_at AS "staff__created_at"`).
		ColumnExpr(`"staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id"`).
		ColumnExpr(`"staff".staff_notes AS "staff__staff_notes"`).
		// Person columns
		ColumnExpr(`"person".id AS "person__id"`).
		ColumnExpr(`"person".created_at AS "person__created_at"`).
		ColumnExpr(`"person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".first_name AS "person__first_name"`).
		ColumnExpr(`"person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id"`).
		ColumnExpr(`"person".account_id AS "person__account_id"`).
		// Joins
		Join(`LEFT JOIN users.staff AS "staff" ON "staff".id = "supervisor".staff_id`).
		Join(`LEFT JOIN users.persons AS "person" ON "person".id = "staff".person_id`)
}

// mapSupervisorResults converts supervisorResult slice to SupervisorPlanned slice with relations.
func mapSupervisorResults(results []supervisorResult) []*activities.SupervisorPlanned {
	supervisors := make([]*activities.SupervisorPlanned, len(results))
	for i, result := range results {
		supervisors[i] = result.Supervisor
		supervisors[i].Staff = result.Staff
		if result.Staff != nil {
			result.Staff.Person = result.Person
		}
	}
	return supervisors
}

// SupervisorPlannedRepository implements activities.SupervisorPlannedRepository interface
type SupervisorPlannedRepository struct {
	*base.Repository[*activities.SupervisorPlanned]
	db *bun.DB
}

// NewSupervisorPlannedRepository creates a new SupervisorPlannedRepository
func NewSupervisorPlannedRepository(db *bun.DB) activitiesPort.SupervisorPlannedRepository {
	return &SupervisorPlannedRepository{
		Repository: base.NewRepository[*activities.SupervisorPlanned](db, "activities.supervisors", "supervisor_planned"),
		db:         db,
	}
}

// FindByID overrides the base repository method to fix the alias issue
func (r *SupervisorPlannedRepository) FindByID(ctx context.Context, id interface{}) (*activities.SupervisorPlanned, error) {
	var supervisor activities.SupervisorPlanned

	var query *bun.SelectQuery

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		query = (*tx).NewSelect()
	} else {
		query = r.db.NewSelect()
	}

	// Use the same alias as base repository: "supervisor_planned"
	err := query.
		Model(&supervisor).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where(`"supervisor_planned".id = ?`, id).
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
	// First get the supervisors
	supervisors := make([]*activities.SupervisorPlanned, 0)
	err := r.db.NewSelect().
		Model(&supervisors).
		ModelTableExpr(tableExprSupervisorPlanned).
		Where("staff_id = ?", staffID).
		Order("is_primary DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff ID",
			Err: err,
		}
	}

	// Load Group relation for each supervisor
	for _, sup := range supervisors {
		if sup.GroupID > 0 {
			group := new(activities.Group)
			groupErr := r.db.NewSelect().
				Model(group).
				ModelTableExpr(`activities.groups AS "group"`).
				Where(whereSupervisorIDEquals, sup.GroupID).
				Scan(ctx)
			if groupErr == nil {
				sup.Group = group
			}
		}
	}

	return supervisors, nil
}

// FindByGroupID finds all supervisors for a specific group
func (r *SupervisorPlannedRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activities.SupervisorPlanned, error) {
	var results []supervisorResult

	query := applySupervisorColumnMapping(r.db.NewSelect().Model(&results)).
		Where(`"supervisor".group_id = ?`, groupID).
		Order("supervisor.is_primary DESC")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: err,
		}
	}

	return mapSupervisorResults(results), nil
}

// FindByGroupIDs finds all supervisors for multiple groups in a single query
func (r *SupervisorPlannedRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activities.SupervisorPlanned, error) {
	if len(groupIDs) == 0 {
		return []*activities.SupervisorPlanned{}, nil
	}

	var results []supervisorResult

	query := applySupervisorColumnMapping(r.db.NewSelect().Model(&results)).
		Where(`"supervisor".group_id IN (?)`, bun.In(groupIDs)).
		Order("supervisor.is_primary DESC")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group IDs",
			Err: err,
		}
	}

	return mapSupervisorResults(results), nil
}

// FindPrimaryByGroupID finds the primary supervisor for a specific group
func (r *SupervisorPlannedRepository) FindPrimaryByGroupID(ctx context.Context, groupID int64) (*activities.SupervisorPlanned, error) {
	var result supervisorResult

	query := applySupervisorColumnMapping(r.db.NewSelect().Model(&result)).
		Where(`"supervisor".group_id = ? AND "supervisor".is_primary = true`, groupID)

	if err := query.Scan(ctx); err != nil {
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
		Where(whereSupervisorIDEquals, id).
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

	var query *bun.UpdateQuery

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		query = (*tx).NewUpdate()
	} else {
		query = r.db.NewUpdate()
	}

	// Configure the query with schema-qualified table
	query = query.
		Model(supervisor).
		ModelTableExpr(tableSupervisorPlanned).
		Where(whereSupervisorIDEquals, supervisor.ID)

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
	var query *bun.DeleteQuery

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		query = (*tx).NewDelete()
	} else {
		query = r.db.NewDelete()
	}

	// Use the same alias as base repository: "supervisor_planned"
	_, err := query.
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where(`"supervisor_planned".id = ?`, id).
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
	supervisors := make([]*activities.SupervisorPlanned, 0)
	query := r.db.NewSelect().Model(&supervisors).ModelTableExpr(tableExprSupervisorPlanned)

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
