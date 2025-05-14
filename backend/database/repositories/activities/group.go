// backend/database/repositories/activities/group.go
package activities

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// GroupRepository implements activities.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*activities.Group]
	db *bun.DB
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *bun.DB) activities.GroupRepository {
	return &GroupRepository{
		Repository: base.NewRepository[*activities.Group](db, "activities.groups", "Group"),
		db:         db,
	}
}

// FindByCategory finds all groups in a specific category
func (r *GroupRepository) FindByCategory(ctx context.Context, categoryID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
	err := r.db.NewSelect().
		Model(&groups).
		Where("category_id = ?", categoryID).
		Order("name ASC").
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
		Where("is_open = ?", true).
		Order("name ASC").
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
		Order("name ASC").
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

// FindWithSupervisors returns a group with its supervisors
func (r *GroupRepository) FindWithSupervisors(ctx context.Context, groupID int64) (*activities.Group, []*activities.SupervisorPlanned, error) {
	// First get the group
	group := new(activities.Group)
	err := r.db.NewSelect().
		Model(group).
		Where("id = ?", groupID).
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
		Relation("Staff").
		Relation("Staff.Person").
		Where("group_id = ?", groupID).
		Order("is_primary DESC").
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find supervisors",
			Err: err,
		}
	}

	return group, supervisors, nil
}

// FindWithSchedules returns a group with its scheduled times
func (r *GroupRepository) FindWithSchedules(ctx context.Context, groupID int64) (*activities.Group, []*activities.Schedule, error) {
	// First get the group
	group := new(activities.Group)
	err := r.db.NewSelect().
		Model(group).
		Where("id = ?", groupID).
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find group",
			Err: err,
		}
	}

	// Then get the schedules
	var schedules []*activities.Schedule
	err = r.db.NewSelect().
		Model(&schedules).
		Relation("Timeframe").
		Where("activity_group_id = ?", groupID).
		Order("weekday, timeframe_id").
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
		Join("JOIN activities.supervisors AS s ON s.group_id = activities.groups.id").
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
		Where("id = ?", group.ID).
		ModelTableExpr("activities.groups")

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(group).
			Where("id = ?", group.ID).
			ModelTableExpr("activities.groups")
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
	query := r.db.NewSelect().Model(&groups)

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

	return groups, nil
}
