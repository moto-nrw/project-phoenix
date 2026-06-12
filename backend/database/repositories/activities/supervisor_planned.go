// backend/database/repositories/activities/supervisor_planned.go
package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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
		ColumnExpr(`"supervisor".tenant_id AS "supervisor__tenant_id"`).
		ColumnExpr(`"supervisor".staff_id AS "supervisor__staff_id"`).
		ColumnExpr(`"supervisor".group_id AS "supervisor__group_id"`).
		ColumnExpr(`"supervisor".is_primary AS "supervisor__is_primary"`).
		// Validity window + period scope: callers feed these rows back into
		// full-row Update (e.g. SetPrimarySupervisor), so omitting them here
		// would NULL valid_from on write (zero timezone.Date binds as NULL).
		ColumnExpr(`"supervisor".valid_from AS "supervisor__valid_from"`).
		ColumnExpr(`"supervisor".valid_until AS "supervisor__valid_until"`).
		ColumnExpr(`"supervisor".calendar_period_id AS "supervisor__calendar_period_id"`).
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
func NewSupervisorPlannedRepository(db *bun.DB) activities.SupervisorPlannedRepository {
	repo := base.NewRepository[*activities.SupervisorPlanned](db, "activities.supervisors", "supervisor_planned")
	repo.TenantScoped = true
	return &SupervisorPlannedRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByID overrides the base repository method to fix the alias issue
func (r *SupervisorPlannedRepository) FindByID(ctx context.Context, id interface{}) (*activities.SupervisorPlanned, error) {
	var supervisor activities.SupervisorPlanned

	// Use the same alias as base repository: "supervisor_planned"
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&supervisor).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where(`"supervisor_planned".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "supervisor_planned"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return &supervisor, nil
}

// CapActiveByGroup ends every still-active supervision (valid_until IS NULL)
// of the given group at validUntil (exclusive). Returns the number of rows
// changed. Custom method (backend-conventions Rule 2): multi-row bulk update
// for the template split (WP-B3).
//
// Two statements, non-primary rows first: the BEFORE UPDATE trigger
// ensure_single_primary_supervisor reacts to ANY update of a row with
// is_primary=TRUE by updating the group's other rows. Folding primary and
// non-primary rows into one bulk UPDATE therefore fails with SQLSTATE 27000
// ("tuple to be updated was already modified by an operation triggered by
// the current command") as soon as the group has more than one active row —
// the period-scoped roster case (migration 1.15.52). Capping non-primary
// rows first leaves the primary row's trigger side effect targeting rows
// modified by a PREVIOUS command, which PostgreSQL permits.
func (r *SupervisorPlannedRepository) CapActiveByGroup(ctx context.Context, groupID int64, validUntil timezone.Date) (int64, error) {
	var total int64
	for _, isPrimary := range []bool{false, true} {
		query := base.GetDB(ctx, r.db).NewUpdate().
			Model((*activities.SupervisorPlanned)(nil)).
			ModelTableExpr(tableExprSupervisorPlanned).
			Set("valid_until = ?", validUntil).
			Where(`"supervisor_planned".group_id = ?`, groupID).
			Where(`"supervisor_planned".valid_until IS NULL`).
			Where(`"supervisor_planned".is_primary = ?`, isPrimary)

		if where, val, ok := base.TenantWhere(ctx, "supervisor_planned"); ok {
			query = query.Where(where, val)
		}

		res, err := query.Exec(ctx)
		if err != nil {
			return total, &modelBase.DatabaseError{
				Op:  "cap active supervisors by group",
				Err: err,
			}
		}
		rows, _ := res.RowsAffected() // nil-driver-safe: fall through with 0
		total += rows
	}
	return total, nil
}

// FindByStaffID finds all supervisions for a specific staff member
func (r *SupervisorPlannedRepository) FindByStaffID(ctx context.Context, staffID int64) ([]*activities.SupervisorPlanned, error) {
	// First get the supervisors
	supervisors := make([]*activities.SupervisorPlanned, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&supervisors).
		ModelTableExpr(tableExprSupervisorPlanned).
		Where("staff_id = ?", staffID)

	if where, val, ok := base.TenantWhere(ctx, "supervisor_planned"); ok {
		query = query.Where(where, val)
	}

	err := query.
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
			groupErr := base.GetDB(ctx, r.db).NewSelect().
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

	query := applySupervisorColumnMapping(base.GetDB(ctx, r.db).NewSelect().Model(&results)).
		Where(`"supervisor".group_id = ?`, groupID).
		Order("supervisor.is_primary DESC")

	if where, val, ok := base.TenantWhere(ctx, "supervisor"); ok {
		query = query.Where(where, val)
	}

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

	query := applySupervisorColumnMapping(base.GetDB(ctx, r.db).NewSelect().Model(&results)).
		Where(`"supervisor".group_id IN (?)`, bun.List(groupIDs)).
		Order("supervisor.is_primary DESC")

	if where, val, ok := base.TenantWhere(ctx, "supervisor"); ok {
		query = query.Where(where, val)
	}

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

	query := applySupervisorColumnMapping(base.GetDB(ctx, r.db).NewSelect().Model(&result)).
		Where(`"supervisor".group_id = ? AND "supervisor".is_primary = true`, groupID)

	if where, val, ok := base.TenantWhere(ctx, "supervisor"); ok {
		query = query.Where(where, val)
	}

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
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		Set("is_primary = true").
		Where(whereSupervisorIDEquals, id)

	if where, val, ok := base.TenantWhere(ctx, "supervisor"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set primary",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "set primary")
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

	// Configure the query with schema-qualified table - GetDB handles transaction extraction from context
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(supervisor).
		ModelTableExpr(tableSupervisorPlanned).
		Where(whereSupervisorIDEquals, supervisor.ID)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	// Execute the query
	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update supervisor")
}

// Delete overrides the base Delete method to handle transactions
func (r *SupervisorPlannedRepository) Delete(ctx context.Context, id interface{}) error {
	// Use the same alias as base repository: "supervisor_planned"
	// GetDB handles transaction extraction from context
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where(`"supervisor_planned".id = ?`, id)

	if where, val, ok := base.TenantWhere(ctx, "supervisor_planned"); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: err,
		}
	}

	return nil
}

// DeleteByStaffID removes all planned supervisions for a staff member. Used by
// staff offboarding, where the staff row is only soft-deleted and the old
// ON DELETE CASCADE therefore no longer cleans up assignments.
func (r *SupervisorPlannedRepository) DeleteByStaffID(ctx context.Context, staffID int64) (int64, error) {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(tableExprSupervisorPlanned).
		Where(`"supervisor_planned".staff_id = ?`, staffID)

	if where, val, ok := base.TenantWhere(ctx, "supervisor_planned"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete by staff id",
			Err: err,
		}
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *SupervisorPlannedRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.SupervisorPlanned, error) {
	supervisors := make([]*activities.SupervisorPlanned, 0)
	query := base.GetDB(ctx, r.db).NewSelect().Model(&supervisors).ModelTableExpr(tableExprSupervisorPlanned)

	if where, val, ok := base.TenantWhere(ctx, "supervisor_planned"); ok {
		query = query.Where(where, val)
	}

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

// ListPlannedSupervisionBlockers returns the staff member's planned activity
// supervisions as caregiver-capability blocker rows. Custom raw-SQL method
// (backend-conventions Rule 2): join into the users blocker read model.
func (r *SupervisorPlannedRepository) ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]users.BlockerActivity, error) {
	var results []users.BlockerActivity
	err := base.GetDB(ctx, r.db).NewRaw(`
		SELECT s.id, s.group_id AS activity_id,
		       COALESCE(ag.name, 'Unbekannte Aktivität') AS activity_name,
		       COALESCE(s.is_primary, false) AS is_primary
		FROM activities.supervisors AS s
		LEFT JOIN activities.groups AS ag ON ag.id = s.group_id AND ag.tenant_id = s.tenant_id
		WHERE s.tenant_id = ?
		  AND s.staff_id = ?
		ORDER BY ag.name ASC
	`, tenantID, staffID).Scan(ctx, &results)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list planned supervision blockers",
			Err: err,
		}
	}
	return results, nil
}
