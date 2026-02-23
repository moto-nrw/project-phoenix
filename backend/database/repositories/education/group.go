// backend/database/repositories/education/group.go
package education

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// GroupRepository implements education.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*education.Group]
	db *bun.DB
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *bun.DB) education.GroupRepository {
	return &GroupRepository{
		Repository: base.NewRepository[*education.Group](db, "education.groups", "Group"),
		db:         db,
	}
}

// FindByName retrieves a group by its name
func (r *GroupRepository) FindByName(ctx context.Context, name string) (*education.Group, error) {
	group := new(education.Group)
	err := r.db.NewSelect().
		Model(group).
		ModelTableExpr(`education.groups AS "group"`).
		Where("LOWER(name) = LOWER(?)", name).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return group, nil
}

// FindByRoom retrieves groups by their room ID
func (r *GroupRepository) FindByRoom(ctx context.Context, roomID int64) ([]*education.Group, error) {
	var groups []*education.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Where("room_id = ?", roomID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by room",
			Err: err,
		}
	}

	return groups, nil
}

// FindByIDs retrieves multiple groups by their IDs in a single query
func (r *GroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*education.Group), nil
	}

	var groups []*education.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.List(ids)).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by IDs",
			Err: err,
		}
	}

	// Convert to map for O(1) lookups
	result := make(map[int64]*education.Group, len(groups))
	for _, group := range groups {
		result[group.ID] = group
	}

	return result, nil
}

// FindByTeacher retrieves groups by their teacher ID (via group_teacher table)
func (r *GroupRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*education.Group, error) {
	var groups []*education.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`education.groups AS "group"`).
		Join("JOIN education.group_teacher gt ON gt.group_id = \"group\".id").
		Where("gt.teacher_id = ?", teacherID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by teacher",
			Err: err,
		}
	}

	return groups, nil
}

// FindWithRoom retrieves a group with its associated room
func (r *GroupRepository) FindWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	group := new(education.Group)

	// Perform manual join to avoid schema issues with Relation()
	type Result struct {
		*education.Group `bun:",extend"`
		Room             *facilities.Room `bun:"rel:belongs-to,join:room_id=id"`
	}

	result := new(Result)
	err := r.db.NewSelect().
		Model(result).
		ModelTableExpr(`education.groups AS "group"`).
		ColumnExpr(`"group".*`).
		ColumnExpr(`"room".id AS "room__id", "room".created_at AS "room__created_at", "room".updated_at AS "room__updated_at"`).
		ColumnExpr(`"room".name AS "room__name", "room".building AS "room__building", "room".floor AS "room__floor"`).
		ColumnExpr(`"room".capacity AS "room__capacity", "room".category AS "room__category", "room".color AS "room__color"`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`).
		Where(`"group".id = ?`, groupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with room",
			Err: err,
		}
	}

	// Map result to group
	group = result.Group
	if result.Room != nil && result.Room.ID != 0 {
		group.Room = result.Room
	}

	return group, nil
}

// Create overrides the base Create method to handle validation
func (r *GroupRepository) Create(ctx context.Context, group *education.Group) error {
	if group == nil {
		return fmt.Errorf("group cannot be nil")
	}

	// Validate group
	if err := group.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, group)
}

// Update overrides the base Update method to handle validation
func (r *GroupRepository) Update(ctx context.Context, group *education.Group) error {
	if group == nil {
		return fmt.Errorf("group cannot be nil")
	}

	// Validate group
	if err := group.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, group)
}

// List retrieves groups matching the provided query options
func (r *GroupRepository) List(ctx context.Context, filters map[string]interface{}) ([]*education.Group, error) {
	options := modelBase.NewQueryOptions()
	options.Filter = buildGroupFilter(filters)
	return r.ListWithOptions(ctx, options)
}

// buildGroupFilter converts legacy filter map to QueryOptions filter
func buildGroupFilter(filters map[string]interface{}) *modelBase.Filter {
	filter := modelBase.NewFilter()
	for field, value := range filters {
		if value == nil {
			continue
		}
		applyGroupFilterField(filter, field, value)
	}
	return filter
}

// applyGroupFilterField applies a single filter field
func applyGroupFilterField(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "name_like":
		if strValue, ok := value.(string); ok {
			filter.ILike("name", "%"+strValue+"%")
		}
	case "has_room":
		if boolValue, ok := value.(bool); ok {
			if boolValue {
				filter.IsNotNull("room_id")
			} else {
				filter.IsNull("room_id")
			}
		}
	default:
		filter.Equal(field, value)
	}
}

// groupWithRoom is used to scan LEFT JOIN results for group + room data
type groupWithRoom struct {
	Group *education.Group `bun:"group"`
	Room  *facilities.Room `bun:"room"`
}

// ListWithOptions provides a type-safe way to list groups with query options
func (r *GroupRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*education.Group, error) {
	var results []groupWithRoom
	query := r.db.NewSelect().
		Model(&results).
		ModelTableExpr(`education.groups AS "group"`).
		ColumnExpr(`"group".id AS "group__id", "group".created_at AS "group__created_at", "group".updated_at AS "group__updated_at"`).
		ColumnExpr(`"group".name AS "group__name", "group".room_id AS "group__room_id"`).
		ColumnExpr(`"room".id AS "room__id", "room".created_at AS "room__created_at", "room".updated_at AS "room__updated_at"`).
		ColumnExpr(`"room".name AS "room__name", "room".building AS "room__building", "room".floor AS "room__floor"`).
		ColumnExpr(`"room".capacity AS "room__capacity", "room".category AS "room__category", "room".color AS "room__color"`).
		Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`)

	// Apply query options (ensure table alias for JOINed queries to avoid ambiguous columns)
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("group")
		}
		if options.Sorting != nil {
			for i, f := range options.Sorting.Fields {
				if !strings.Contains(f.Field, ".") {
					options.Sorting.Fields[i].Field = `group.` + f.Field
				}
			}
		}
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	// Map results back to []*education.Group with room attached
	groups := make([]*education.Group, len(results))
	for i, result := range results {
		groups[i] = result.Group
		if result.Room != nil && result.Room.ID != 0 {
			groups[i].Room = result.Room
		}
	}

	return groups, nil
}

// CountWithOptions counts groups matching the query options (without pagination)
func (r *GroupRepository) CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error) {
	query := r.db.NewSelect().
		Model((*education.Group)(nil)).
		ModelTableExpr(`education.groups AS "group"`).
		Column("group.id")

	// Apply only filters (not pagination) for counting
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("group")
			query = options.Filter.ApplyToQuery(query)
		}
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count with options",
			Err: err,
		}
	}

	return count, nil
}
