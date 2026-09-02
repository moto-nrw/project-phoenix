// backend/database/repositories/activities/supervisor_planned.go
package activities

import (
	"context"
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
	setSupervisorValidUntil    = "valid_until = ?"
)

// supervisorResult holds the result of a supervisor query with the joined
// staff row. Staff.Person is attached by the composition layer through the
// People Directory.
type supervisorResult struct {
	Supervisor *activities.SupervisorPlanned `bun:"supervisor"`
	Staff      *users.Staff                  `bun:"staff"`
}

// applySupervisorColumnMapping adds all required column expressions for supervisor queries
// with the staff join. This eliminates duplication across FindByGroupID and
// FindByGroupIDs.
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
		// would overwrite valid_from with an invalid empty calendar date.
		ColumnExpr(`"supervisor".valid_from AS "supervisor__valid_from"`).
		ColumnExpr(`"supervisor".valid_until AS "supervisor__valid_until"`).
		ColumnExpr(`"supervisor".calendar_period_id AS "supervisor__calendar_period_id"`).
		// Weekday scope (#2129) — same reason as the validity window above:
		// omitting it here would widen a Monday-only assignment to the whole
		// series on the next full-row Update.
		ColumnExpr(`"supervisor".weekday AS "supervisor__weekday"`).
		// Staff columns
		ColumnExpr(`"staff".id AS "staff__id"`).
		ColumnExpr(`"staff".created_at AS "staff__created_at"`).
		ColumnExpr(`"staff".updated_at AS "staff__updated_at"`).
		ColumnExpr(`"staff".person_id AS "staff__person_id"`).
		ColumnExpr(`"staff".staff_notes AS "staff__staff_notes"`).
		// Joins
		Join(`LEFT JOIN users.staff AS "staff" ON "staff".id = "supervisor".staff_id`)
}

// mapSupervisorResults converts supervisorResult slice to SupervisorPlanned slice with relations.
func mapSupervisorResults(results []supervisorResult) []*activities.SupervisorPlanned {
	supervisors := make([]*activities.SupervisorPlanned, len(results))
	for i, result := range results {
		supervisors[i] = result.Supervisor
		supervisors[i].Staff = result.Staff
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

// CapActiveByGroup truncates open supervisions. Future-only open rows are
// deleted instead of creating valid_until < valid_from; begun open rows are
// capped. Bounded rows remain untouched.
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
	deleteQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(tableExprSupervisorPlanned).
		Where(`"supervisor_planned".group_id = ?`, groupID).
		Where(`"supervisor_planned".valid_from >= ?`, validUntil).
		Where(`"supervisor_planned".valid_until IS NULL`)
	deleteQuery = base.WithTenantFilter(ctx, deleteQuery, "supervisor_planned")
	deletedResult, err := deleteQuery.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete future supervisors by group", Err: base.TranslateNotFound(err)}
	}
	total, _ := deletedResult.RowsAffected()

	for _, isPrimary := range []bool{false, true} {
		query := base.GetDB(ctx, r.db).NewUpdate().
			Model((*activities.SupervisorPlanned)(nil)).
			ModelTableExpr(tableExprSupervisorPlanned).
			Set(setSupervisorValidUntil, validUntil).
			Where(`"supervisor_planned".group_id = ?`, groupID).
			Where(`"supervisor_planned".valid_from < ?`, validUntil).
			Where(`"supervisor_planned".valid_until IS NULL`).
			Where(`"supervisor_planned".is_primary = ?`, isPrimary)

		query = base.WithTenantFilter(ctx, query, "supervisor_planned")

		res, err := query.Exec(ctx)
		if err != nil {
			return total, &modelBase.DatabaseError{
				Op:  "cap active supervisors by group",
				Err: base.TranslateNotFound(err),
			}
		}
		rows, _ := res.RowsAffected() // nil-driver-safe: fall through with 0
		total += rows
	}
	return total, nil
}

// SetValidUntilByID closes one supervision using a narrow update. This avoids
// writing relation-backed or otherwise partial read models back to the roster
// table when only the validity boundary changes.
func (r *SupervisorPlannedRepository) SetValidUntilByID(ctx context.Context, id int64, validUntil timezone.Date) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(tableExprSupervisorPlanned).
		Set(setSupervisorValidUntil, validUntil).
		Where(`"supervisor_planned".id = ?`, id)
	query = base.WithTenantFilter(ctx, query, "supervisor_planned")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "set supervisor valid_until", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(result, 1, "set supervisor valid_until")
}

// FindByStaffID finds all supervisions for a specific staff member
func (r *SupervisorPlannedRepository) FindByStaffID(ctx context.Context, staffID int64) ([]*activities.SupervisorPlanned, error) {
	// First get the supervisors
	supervisors := make([]*activities.SupervisorPlanned, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&supervisors).
		ModelTableExpr(tableExprSupervisorPlanned).
		Where("staff_id = ?", staffID)

	query = base.WithTenantFilter(ctx, query, "supervisor_planned")

	err := query.
		Order("is_primary DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff ID",
			Err: base.TranslateNotFound(err),
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

	query = base.WithTenantFilter(ctx, query, "supervisor")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: base.TranslateNotFound(err),
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

	query = base.WithTenantFilter(ctx, query, "supervisor")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group IDs",
			Err: base.TranslateNotFound(err),
		}
	}

	return mapSupervisorResults(results), nil
}

// SetPrimary sets a supervisor as the primary supervisor for a group
func (r *SupervisorPlannedRepository) SetPrimary(ctx context.Context, id int64) error {
	// We rely on the database trigger to ensure only one primary supervisor per group
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*activities.SupervisorPlanned)(nil)).
		ModelTableExpr(`activities.supervisors AS "supervisor"`).
		Set("is_primary = true").
		Where(whereSupervisorIDEquals, id)

	query = base.WithTenantFilter(ctx, query, "supervisor")

	result, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set primary",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "set primary")
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
			Err: base.TranslateNotFound(err),
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

	query = base.WithTenantFilter(ctx, query, "supervisor_planned")

	_, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: base.TranslateNotFound(err),
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

	query = base.WithTenantFilter(ctx, query, "supervisor_planned")

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete by staff id",
			Err: base.TranslateNotFound(err),
		}
	}
	rows, _ := result.RowsAffected()
	return rows, nil
}

// List overrides the base List method to accept the new QueryOptions type
func (r *SupervisorPlannedRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.SupervisorPlanned, error) {
	supervisors, err := r.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}
	if supervisors == nil {
		supervisors = make([]*activities.SupervisorPlanned, 0)
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
			Err: base.TranslateNotFound(err),
		}
	}
	return results, nil
}

// CloseOpenByGroupAndPeriod closes the open planned supervisions of a group
// for the given calendar period (issue #584: moved verbatim from
// api/timetable).
func (r *SupervisorPlannedRepository) CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error {
	tenantID := tenant.FromContext(ctx)
	update := base.GetDB(ctx, r.db).NewUpdate().
		Table("activities.supervisors").
		Set(setSupervisorValidUntil, validFrom).
		Where("tenant_id = ?", tenantID).
		Where("group_id = ?", groupID).
		Where("valid_until IS NULL")
	if calendarPeriodID == nil {
		update = update.Where("calendar_period_id IS NULL")
	} else {
		update = update.Where("calendar_period_id = ?", *calendarPeriodID)
	}
	_, err := update.Exec(ctx)
	return err
}
