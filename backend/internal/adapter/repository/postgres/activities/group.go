// backend/database/repositories/activities/group.go
package activities

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableActivitiesGroups          = "activities.groups"
	tableExprActivitiesGroupsAsGrp = `activities.groups AS "group"`
	orderByNameAsc                 = "name ASC"
	whereIDEquals                  = "id = ?"
)

// GroupRepository implements activities.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*activities.Group]
	db *bun.DB
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *bun.DB) activities.GroupRepository {
	return &GroupRepository{
		Repository: base.NewRepository[*activities.Group](db, tableActivitiesGroups, "Group"),
		db:         db,
	}
}

// FindByCategory finds all groups in a specific category
func (r *GroupRepository) FindByCategory(ctx context.Context, categoryID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where("category_id = ?", categoryID).
		Order(orderByNameAsc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by category",
			Err: err,
		}
	}

	return groups, nil
}

// FindOpenGroups finds all groups that are open for enrollment
func (r *GroupRepository) FindOpenGroups(ctx context.Context) ([]*activities.Group, error) {
	var groups []*activities.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where("is_open = ?", true).
		Order(orderByNameAsc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find open groups",
			Err: err,
		}
	}

	return groups, nil
}

// FindWithEnrollmentCounts returns groups with their current enrollment counts
func (r *GroupRepository) FindWithEnrollmentCounts(ctx context.Context) ([]*activities.Group, map[int64]int, error) {
	var groups []*activities.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Order(orderByNameAsc).
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find groups",
			Err: err,
		}
	}

	// If no groups, return early
	if len(groups) == 0 {
		return groups, make(map[int64]int), nil
	}

	// Get all group IDs
	var groupIDs []interface{}
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}

	// Get enrollment counts for each group
	type countResult struct {
		GroupID int64 `bun:"activity_group_id"`
		Count   int   `bun:"count"`
	}
	var counts []countResult
	err = r.db.NewSelect().
		ModelTableExpr("activities.student_enrollments").
		Column("activity_group_id").
		ColumnExpr("COUNT(*) AS count").
		Where("activity_group_id IN (?)", bun.In(groupIDs)).
		Group("activity_group_id").
		Scan(ctx, &counts)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "count enrollments",
			Err: err,
		}
	}

	// Convert to map
	countMap := make(map[int64]int)
	for _, count := range counts {
		countMap[count.GroupID] = count.Count
	}

	return groups, countMap, nil
}

// loadStaffWithPerson loads staff and person relations for a supervisor
func (r *GroupRepository) loadStaffWithPerson(ctx context.Context, sup *activities.SupervisorPlanned) {
	if sup.StaffID <= 0 {
		return
	}

	staff := new(users.Staff)
	staffErr := r.db.NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(whereIDEquals, sup.StaffID).
		Scan(ctx)

	if staffErr != nil {
		return
	}

	sup.Staff = staff
	if staff.PersonID <= 0 {
		return
	}

	person := new(users.Person)
	personErr := r.db.NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(whereIDEquals, staff.PersonID).
		Scan(ctx)

	if personErr == nil {
		staff.Person = person
	}
}

// FindWithSupervisors returns a group with its supervisors
func (r *GroupRepository) FindWithSupervisors(ctx context.Context, groupID int64) (*activities.Group, []*activities.SupervisorPlanned, error) {
	// First get the group
	group := new(activities.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(whereIDEquals, groupID).
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find group",
			Err: err,
		}
	}

	// Then get the supervisors
	var supervisors []*activities.SupervisorPlanned
	err = r.db.NewSelect().
		Model(&supervisors).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where("group_id = ?", groupID).
		Order("is_primary DESC").
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find supervisors",
			Err: err,
		}
	}

	// Load Staff and Person relations for each supervisor
	for _, sup := range supervisors {
		r.loadStaffWithPerson(ctx, sup)
	}

	return group, supervisors, nil
}

// FindWithSchedules returns a group with its scheduled times
func (r *GroupRepository) FindWithSchedules(ctx context.Context, groupID int64) (*activities.Group, []*activities.Schedule, error) {
	// First get the group
	group := new(activities.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(whereIDEquals, groupID).
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find group",
			Err: err,
		}
	}

	// Then get the schedules
	// Note: Timeframe relation is commented out in Schedule model, so we can't use Relation()
	// The caller should load Timeframe separately if needed
	schedules := make([]*activities.Schedule, 0)
	err = r.db.NewSelect().
		Model(&schedules).
		ModelTableExpr(tableExprActivitiesSchedulesAsSch).
		Where("activity_group_id = ?", groupID).
		Order("weekday ASC").
		Order("timeframe_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find schedules",
			Err: err,
		}
	}

	return group, schedules, nil
}

// FindByStaffSupervisor finds all activity groups where a staff member is a supervisor
func (r *GroupRepository) FindByStaffSupervisor(ctx context.Context, staffID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Join("JOIN activities.supervisors AS s ON s.group_id = \"group\".id").
		Where("s.staff_id = ?", staffID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff supervisor",
			Err: err,
		}
	}

	return groups, nil
}

// FindByStaffSupervisorToday finds all activity groups where a staff member is a supervisor
func (r *GroupRepository) FindByStaffSupervisorToday(ctx context.Context, staffID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Join(`JOIN activities.supervisors AS s ON s.group_id = "group".id`).
		Where("s.staff_id = ?", staffID).
		Where("is_open = ?", true).
		Order(orderByNameAsc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff supervisor",
			Err: err,
		}
	}

	return groups, nil
}

// Create overrides the base Create method to handle validation
func (r *GroupRepository) Create(ctx context.Context, group *activities.Group) error {
	if group == nil {
		return fmt.Errorf("group cannot be nil")
	}

	// Validate group
	if err := group.Validate(); err != nil {
		return err
	}

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, group)
}

// Update overrides the base Update method to handle validation
func (r *GroupRepository) Update(ctx context.Context, group *activities.Group) error {
	if group == nil {
		return fmt.Errorf("group cannot be nil")
	}

	// Validate group
	if err := group.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(group).
		Where(whereIDEquals, group.ID).
		ModelTableExpr(tableActivitiesGroups)

	// Extract transaction from context if it exists
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(group).
			Where(whereIDEquals, group.ID).
			ModelTableExpr(tableActivitiesGroups)
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
func (r *GroupRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.Group, error) {
	var groups []*activities.Group
	query := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		ColumnExpr(`"group".*`).
		ColumnExpr(`"category"."id" AS "category__id"`).
		ColumnExpr(`"category"."created_at" AS "category__created_at"`).
		ColumnExpr(`"category"."updated_at" AS "category__updated_at"`).
		ColumnExpr(`"category"."name" AS "category__name"`).
		ColumnExpr(`"category"."description" AS "category__description"`).
		ColumnExpr(`"category"."color" AS "category__color"`).
		Join(`LEFT JOIN activities.categories AS "category" ON "category"."id" = "group"."category_id"`)

	// Apply query options with table alias to avoid ambiguous column references
	// (both "group" and "category" have "id" columns)
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("group")
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

	return groups, nil
}
