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
	repo := base.NewRepository[*active.GroupSupervisor](db, "active.group_supervisors", "GroupSupervisor")
	repo.TenantScoped = true
	return &GroupSupervisorRepository{
		Repository: repo,
		db:         db,
	}
}

// FindActiveByStaffID finds all active supervisions for a specific staff member
func (r *GroupSupervisorRepository) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	var supervisions []*active.GroupSupervisor
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&supervisions).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Where("staff_id = ? AND (end_date IS NULL OR end_date > NOW())", staffID)

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
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
	query := base.GetDB(ctx, r.db).NewSelect().
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

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&supervisions).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		// Use explicit JOINs for schema-qualified tables (Relation() doesn't handle cross-schema properly)
		ColumnExpr(`"group_supervisor".*`).
		ColumnExpr(`"staff"."id" AS "staff__id", "staff"."person_id" AS "staff__person_id", "staff"."staff_notes" AS "staff__staff_notes"`).
		ColumnExpr(`"person"."id" AS "staff__person__id", "person"."first_name" AS "staff__person__first_name", "person"."last_name" AS "staff__person__last_name"`).
		Join(`LEFT JOIN users.staff AS "staff" ON "staff"."id" = "group_supervisor"."staff_id"`).
		Join(`LEFT JOIN users.persons AS "person" ON "person"."id" = "staff"."person_id"`).
		Where(`"group_supervisor".group_id IN (?)`, bun.List(activeGroupIDs))

	if activeOnly {
		query = query.Where(`"group_supervisor".end_date IS NULL`)
	}

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
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
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.GroupSupervisor)(nil)).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Set("end_date = ?", time.Now()).
		Where(`"group_supervisor".id = ? AND "group_supervisor".end_date IS NULL`, id)

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	_, err := query.Exec(ctx)
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

// CreateBulk inserts multiple supervisors in a single query.
// All supervisors are validated and have their tenant IDs set from context before insertion.
func (r *GroupSupervisorRepository) CreateBulk(ctx context.Context, supervisors []*active.GroupSupervisor) error {
	if len(supervisors) == 0 {
		return nil
	}

	// Validate all supervisors and set tenant IDs
	for _, sup := range supervisors {
		if sup == nil {
			return fmt.Errorf("group supervisor cannot be nil")
		}
		if err := sup.Validate(); err != nil {
			return err
		}
		base.EnsureTenantID(ctx, sup)
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(&supervisors).
		ModelTableExpr(`active.group_supervisors`).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create bulk",
			Err: err,
		}
	}

	return nil
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

	// Perform the update with proper table expression
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(supervision).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		WherePK()

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update group_supervisor")
}

// applyActiveOnlyFilter handles the special active_only filter for group supervisors.
// Returns the modified query with the appropriate WHERE clause applied.
func (r *GroupSupervisorRepository) applyActiveOnlyFilter(query *bun.SelectQuery, filter *modelBase.Filter) *bun.SelectQuery {
	activeOnly, ok := filter.Get("active_only")
	if !ok {
		return query
	}

	// Remove from filter so ApplyToQuery doesn't try to use it as a column
	filter.Remove("active_only")

	isActive, isBool := activeOnly.(bool)
	if !isBool {
		return query
	}

	if isActive {
		return query.Where(`"group_supervisor".end_date IS NULL OR "group_supervisor".end_date > NOW()`)
	}
	// active=false returns only inactive (ended) supervisors
	return query.Where(`"group_supervisor".end_date IS NOT NULL AND "group_supervisor".end_date <= NOW()`)
}

// List overrides the base List method to accept the new QueryOptions type
func (r *GroupSupervisorRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.GroupSupervisor, error) {
	var supervisions []*active.GroupSupervisor
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&supervisions).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`)

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	if options != nil {
		if options.Filter != nil {
			query = r.applyActiveOnlyFilter(query, options.Filter)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(supervision).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Relation("Staff").
		Where("id = ?", id)

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(supervision).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Relation("ActiveGroup").
		Where("id = ?", id)

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with active group",
			Err: err,
		}
	}

	return supervision, nil
}

// EndAllActiveByStaffID ends all active supervisions for a staff member.
// Sets end_date = CURRENT_DATE for all supervisions where end_date IS NULL.
// Returns the number of supervisions that were ended.
func (r *GroupSupervisorRepository) EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error) {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.GroupSupervisor)(nil)).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Set("end_date = CURRENT_DATE").
		Where(`"group_supervisor".staff_id = ? AND "group_supervisor".end_date IS NULL`, staffID)

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end all active by staff ID",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end all active by staff ID (rows affected)",
			Err: err,
		}
	}

	return int(rowsAffected), nil
}

// EndSupervisionsByActiveGroupIDs ends all active supervisions for multiple group IDs in a single query.
// Returns the number of supervisions ended.
func (r *GroupSupervisorRepository) EndSupervisionsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) (int64, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*active.GroupSupervisor)(nil)).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Set("end_date = now()").
		Where(`"group_supervisor".group_id IN (?)`, bun.List(activeGroupIDs)).
		Where(`"group_supervisor".end_date IS NULL`)

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end supervisions by active group IDs",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "end supervisions by active group IDs (rows affected)",
			Err: err,
		}
	}

	return rowsAffected, nil
}

// GetStaffIDsWithSupervisionToday returns staff IDs who had any supervision activity today.
// This is used to determine "Anwesend" status - staff who were physically present via PyrePortal.
// A staff member is considered present today if:
// - Their supervision started today, OR
// - Their supervision ended today, OR
// - Their supervision spans today (started before and still ongoing or ends after today)
func (r *GroupSupervisorRepository) GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error) {
	var staffIDs []int64
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*active.GroupSupervisor)(nil)).
		ModelTableExpr(`active.group_supervisors AS "group_supervisor"`).
		Column("staff_id").
		Distinct().
		Where(`(
			"group_supervisor"."start_date" = CURRENT_DATE
			OR "group_supervisor"."end_date" = CURRENT_DATE
			OR (
				"group_supervisor"."start_date" < CURRENT_DATE
				AND ("group_supervisor"."end_date" IS NULL OR "group_supervisor"."end_date" > CURRENT_DATE)
			)
		)`)

	if where, val, ok := base.TenantWhere(ctx, "group_supervisor"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx, &staffIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get staff IDs with supervision today",
			Err: err,
		}
	}

	return staffIDs, nil
}
