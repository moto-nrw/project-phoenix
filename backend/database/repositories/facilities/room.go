// backend/database/repositories/facilities/room.go
package facilities

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableFacilitiesRooms           = "facilities.rooms"
	tableExprFacilitiesRoomsAsRoom = `facilities.rooms AS "room"`
)

// RoomRepository implements facilities.RoomRepository interface
type RoomRepository struct {
	*base.Repository[*facilities.Room]
	db *bun.DB
	// supervisorPersons maps supervising staff IDs to the persons behind
	// them. School Membership owns those rows, so the composition root
	// injects the lookup instead of this repository joining users.staff
	// (#2667). Without it the occupancy rows carry staff IDs only.
	supervisorPersons func(ctx context.Context, staffIDs []int64) (map[int64]int64, error)
}

// SetSupervisorPersonsResolver installs the staff-to-person lookup used to
// fill RoomOccupancyRow.SupervisorPersonIDs.
func (r *RoomRepository) SetSupervisorPersonsResolver(resolve func(ctx context.Context, staffIDs []int64) (map[int64]int64, error)) {
	r.supervisorPersons = resolve
}

// attachSupervisorPersons resolves the persons behind every row's supervising
// staff. The replaced array_agg(DISTINCT st.person_id) handed the persons
// over deduplicated and in ascending ID order; keep that so name rendering
// stays stable. The former INNER JOIN carried no soft-delete filter, so the
// resolver is expected to include offboarded staff.
func (r *RoomRepository) attachSupervisorPersons(ctx context.Context, rows []facilities.RoomOccupancyRow) error {
	if r.supervisorPersons == nil || len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.SupervisorStaffIDs...)
	}
	persons, err := r.supervisorPersons(ctx, ids)
	if err != nil {
		return fmt.Errorf("resolve supervising persons: %w", err)
	}
	for index := range rows {
		personIDs := make([]int64, 0, len(rows[index].SupervisorStaffIDs))
		seen := make(map[int64]struct{}, len(rows[index].SupervisorStaffIDs))
		for _, staffID := range rows[index].SupervisorStaffIDs {
			personID, found := persons[staffID]
			if !found {
				continue
			}
			if _, duplicate := seen[personID]; duplicate {
				continue
			}
			seen[personID] = struct{}{}
			personIDs = append(personIDs, personID)
		}
		slices.Sort(personIDs)
		rows[index].SupervisorPersonIDs = personIDs
	}
	return nil
}

// FindByIDForUpdate retrieves and locks a room for capacity-sensitive writes.
func (r *RoomRepository) FindByIDForUpdate(ctx context.Context, id int64) (*facilities.Room, error) {
	room := new(facilities.Room)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(room).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
		Where(`"room".id = ?`, id).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "room")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find room for update", Err: base.TranslateNotFound(err)}
	}
	return room, nil
}

// NewRoomRepository creates a new RoomRepository
func NewRoomRepository(db *bun.DB) facilities.RoomRepository {
	repo := base.NewRepository[*facilities.Room](db, tableFacilitiesRooms, "Room")
	repo.TenantScoped = true
	return &RoomRepository{
		Repository: repo,
		db:         db,
	}
}

// Update overrides the base Update method for schema consistency
func (r *RoomRepository) Update(ctx context.Context, room *facilities.Room) error {
	if room == nil {
		return fmt.Errorf("room cannot be nil")
	}

	// Validation lives in the service layer (see Create above for the full
	// rationale). Repo writes trust the service to have validated already.

	// Use GetDB to detect transaction from context
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(room).
		Where("id = ?", room.ID).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom)

	query = base.WithTenantFilter(ctx, query, "room")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update room")
}

// FindByName retrieves a room by its name (case-insensitive).
func (r *RoomRepository) FindByName(ctx context.Context, name string) (*facilities.Room, error) {
	room := new(facilities.Room)
	query := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
		Where("LOWER(name) = LOWER(?)", name)

	query = base.WithTenantFilter(ctx, query, "room")

	err := query.Scan(ctx, room)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: base.TranslateNotFound(err),
		}
	}

	return room, nil
}

// FindByCategory retrieves rooms by category
func (r *RoomRepository) FindByCategory(ctx context.Context, category string) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rooms).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
		Where("LOWER(category) = LOWER(?)", category)

	query = base.WithTenantFilter(ctx, query, "room")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by category",
			Err: base.TranslateNotFound(err),
		}
	}

	return rooms, nil
}

// List retrieves rooms matching the provided filters
// Note: This implementation still uses the old map[string]interface{} filter system
// but should be migrated to QueryOptions in the future
func (r *RoomRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rooms).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom)

	query = base.WithTenantFilter(ctx, query, "room")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = applyRoomFilter(query, field, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: base.TranslateNotFound(err),
		}
	}

	return rooms, nil
}

// applyRoomFilter applies a single filter to the query based on field name
func applyRoomFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "name":
		return applyCaseInsensitiveExactMatch(query, "name", value)
	case "name_like":
		return applyCaseInsensitiveLikeMatch(query, "name", value)
	case "building":
		return applyCaseInsensitiveExactMatch(query, "building", value)
	case "building_like":
		return applyCaseInsensitiveLikeMatch(query, "building", value)
	case "category":
		return applyCaseInsensitiveExactMatch(query, "category", value)
	case "min_capacity":
		return query.Where("capacity IS NULL OR capacity <= 0 OR capacity >= ?", value)
	case "max_capacity":
		return query.Where("capacity <= ?", value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyCaseInsensitiveExactMatch applies case-insensitive exact match filter
func applyCaseInsensitiveExactMatch(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+column+") = LOWER(?)", strValue)
	}
	return query.Where(column+" = ?", value)
}

// applyCaseInsensitiveLikeMatch applies case-insensitive LIKE filter
func applyCaseInsensitiveLikeMatch(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+column+") LIKE LOWER(?)", "%"+strValue+"%")
	}
	return query
}

// FindWithCapacity retrieves rooms with at least the specified capacity
func (r *RoomRepository) FindWithCapacity(ctx context.Context, minCapacity int) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rooms).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
		Where("capacity IS NULL OR capacity <= 0 OR capacity >= ?", minCapacity)

	query = base.WithTenantFilter(ctx, query, "room")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with capacity",
			Err: base.TranslateNotFound(err),
		}
	}

	return rooms, nil
}

// SearchByText performs a text search across multiple room fields
func (r *RoomRepository) SearchByText(ctx context.Context, searchText string) ([]*facilities.Room, error) {
	if searchText == "" {
		return []*facilities.Room{}, nil
	}

	var rooms []*facilities.Room
	searchPattern := "%" + strings.ToLower(searchText) + "%"

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rooms).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
		Where("LOWER(name) LIKE ? OR LOWER(building) LIKE ? OR LOWER(category) LIKE ?",
			searchPattern, searchPattern, searchPattern)

	query = base.WithTenantFilter(ctx, query, "room")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "search by text",
			Err: base.TranslateNotFound(err),
		}
	}

	return rooms, nil
}

// occupancyColumns adds the shared occupancy projection used by
// FindWithOccupancy and ListWithOccupancy. Aggregates across ALL active
// groups in the room (not a single arbitrary one) so the list and detail
// surfaces stay consistent for multi-group rooms (#1374).
func occupancyColumns(q *bun.SelectQuery) *bun.SelectQuery {
	return q.
		ColumnExpr("r.id, r.name, r.building, r.floor, r.capacity, r.category, r.color, r.created_at, r.updated_at").
		ColumnExpr(`EXISTS (
			SELECT 1 FROM active.groups ag
			WHERE ag.room_id = r.id AND ag.end_time IS NULL
		) AS is_occupied`).
		ColumnExpr(`(
			SELECT string_agg(DISTINCT act_group.name, ', ' ORDER BY act_group.name)
			FROM active.groups ag
			INNER JOIN activities.groups act_group ON act_group.id = ag.group_id
			WHERE ag.room_id = r.id AND ag.end_time IS NULL
		) AS group_name`).
		ColumnExpr(`(
			SELECT string_agg(DISTINCT cat.name, ', ' ORDER BY cat.name)
			FROM active.groups ag
			INNER JOIN activities.groups act_group ON act_group.id = ag.group_id
			INNER JOIN activities.categories cat ON cat.id = act_group.category_id
			WHERE ag.room_id = r.id AND ag.end_time IS NULL
		) AS category_name`).
		ColumnExpr(`COALESCE((
			SELECT COUNT(DISTINCT v.student_id)
			FROM active.visits v
			INNER JOIN active.groups ag ON ag.id = v.active_group_id
			WHERE ag.room_id = r.id AND ag.end_time IS NULL AND v.exit_time IS NULL
		), 0)::int AS student_count`).
		// Only the supervising staff IDs are read here; the composition layer
		// turns them into SupervisorPersonIDs through School Membership (#2667).
		ColumnExpr(`COALESCE((
			SELECT array_agg(DISTINCT gs.staff_id)
			FROM active.group_supervisors gs
			INNER JOIN active.groups ag ON ag.id = gs.group_id
			WHERE ag.room_id = r.id AND ag.end_time IS NULL AND gs.end_date IS NULL
		), '{}'::bigint[]) AS supervisor_staff_ids`)
}

// FindWithOccupancy returns the room row plus its live occupancy aggregate.
// Custom method (backend-conventions Rule 2): correlated subqueries over
// active.groups / visits / supervisors that the generic shape cannot
// express. The not-found case surfaces as a wrapped sql.ErrNoRows.
func (r *RoomRepository) FindWithOccupancy(ctx context.Context, id int64) (*facilities.RoomOccupancyRow, error) {
	var result facilities.RoomOccupancyRow
	query := occupancyColumns(base.GetDB(ctx, r.db).NewSelect().
		TableExpr("facilities.rooms AS r")).
		Where("r.id = ?", id)

	query = base.WithTenantFilter(ctx, query, "r")

	if err := query.Scan(ctx, &result); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find room with occupancy",
			Err: base.TranslateNotFound(err),
		}
	}
	rows := []facilities.RoomOccupancyRow{result}
	if err := r.attachSupervisorPersons(ctx, rows); err != nil {
		return nil, err
	}
	return &rows[0], nil
}

// ListWithOccupancy returns every room (optionally filtered) plus its live
// occupancy aggregate, ordered by name. Same projection as
// FindWithOccupancy so the list and detail surfaces cannot drift.
func (r *RoomRepository) ListWithOccupancy(ctx context.Context, options *modelBase.QueryOptions) ([]facilities.RoomOccupancyRow, error) {
	query := occupancyColumns(base.GetDB(ctx, r.db).NewSelect().
		TableExpr("facilities.rooms AS r")).
		OrderExpr("r.name ASC")

	query = base.WithTenantFilter(ctx, query, "r")

	if options != nil && options.Filter != nil {
		options.Filter.WithTableAlias("r")
		query = base.ApplyFilter(query, options.Filter)
	}

	var results []facilities.RoomOccupancyRow
	if err := query.Scan(ctx, &results); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list rooms with occupancy",
			Err: base.TranslateNotFound(err),
		}
	}
	if err := r.attachSupervisorPersons(ctx, results); err != nil {
		return nil, err
	}
	return results, nil
}

// FindByIDs retrieves rooms by their IDs. Custom method (Rule 2): the
// generic List(filters) cannot express the empty-slice short-circuit that
// callers rely on to skip the round trip entirely.
func (r *RoomRepository) FindByIDs(ctx context.Context, ids []int64) ([]*facilities.Room, error) {
	rooms := make([]*facilities.Room, 0, len(ids))
	if len(ids) == 0 {
		return rooms, nil
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rooms).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Where(`"room".id IN (?)`, bun.List(ids))
	query = base.WithTenantFilter(ctx, query, "room")
	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return rooms, nil
}
