// backend/database/repositories/active/group_batch.go
package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	"github.com/uptrace/bun"
)

// FindByIDs finds active groups by their IDs in a single query
func (r *GroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*active.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*active.Group), nil
	}

	uniqueIDs := deduplicateIDs(ids)

	groups, err := r.queryGroupsByIDs(ctx, uniqueIDs)
	if err != nil {
		return nil, err
	}

	if err := r.loadRoomsForGroups(ctx, groups); err != nil {
		return nil, err
	}

	return groupsToMap(groups), nil
}

// deduplicateIDs removes duplicate IDs from the slice
func deduplicateIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}

// queryGroupsByIDs fetches groups by their IDs
func (r *GroupRepository) queryGroupsByIDs(ctx context.Context, ids []int64) ([]*active.Group, error) {
	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.In(ids)).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find groups by IDs", Err: err}
	}
	return groups, nil
}

// loadRoomsForGroups batch loads rooms for the given groups
func (r *GroupRepository) loadRoomsForGroups(ctx context.Context, groups []*active.Group) error {
	roomIDs := collectRoomIDs(groups)
	if len(roomIDs) == 0 {
		return nil
	}

	rooms, err := r.queryRoomsByIDs(ctx, roomIDs, "find group rooms by IDs")
	if err != nil {
		return err
	}

	assignRoomsToGroups(groups, rooms)
	return nil
}

// collectRoomIDs extracts unique room IDs from groups
func collectRoomIDs(groups []*active.Group) []int64 {
	seen := make(map[int64]struct{})
	ids := make([]int64, 0)
	for _, g := range groups {
		if g.RoomID > 0 {
			if _, exists := seen[g.RoomID]; !exists {
				seen[g.RoomID] = struct{}{}
				ids = append(ids, g.RoomID)
			}
		}
	}
	return ids
}

// queryRoomsByIDs fetches rooms by their IDs
func (r *GroupRepository) queryRoomsByIDs(ctx context.Context, ids []int64, op string) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	if err := r.db.NewSelect().
		Model(&rooms).
		ModelTableExpr(`facilities.rooms AS "room"`).
		Where(`"room".id IN (?)`, bun.In(ids)).
		Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: op, Err: err}
	}
	return rooms, nil
}

// assignRoomsToGroups assigns rooms to groups based on room ID
func assignRoomsToGroups(groups []*active.Group, rooms []*facilities.Room) {
	roomMap := make(map[int64]*facilities.Room, len(rooms))
	for _, room := range rooms {
		roomMap[room.ID] = room
	}
	for _, g := range groups {
		if room, ok := roomMap[g.RoomID]; ok {
			g.Room = room
		}
	}
}

// groupsToMap converts a slice of groups to a map keyed by ID
func groupsToMap(groups []*active.Group) map[int64]*active.Group {
	result := make(map[int64]*active.Group, len(groups))
	for _, g := range groups {
		result[g.ID] = g
	}
	return result
}

// FindUnclaimed finds all active groups that have no supervisors assigned
// This is used to allow teachers to claim Schulhof via the frontend
// Only returns groups in rooms named "Schulhof" - this is the only room that supports deviceless claiming
func (r *GroupRepository) FindUnclaimed(ctx context.Context) ([]*active.Group, error) {
	groups, err := r.queryUnclaimedGroups(ctx)
	if err != nil {
		return nil, err
	}

	if err := r.loadUnclaimedGroupRelations(ctx, groups); err != nil {
		return nil, err
	}

	return groups, nil
}

// queryUnclaimedGroups fetches unclaimed groups from the database
func (r *GroupRepository) queryUnclaimedGroups(ctx context.Context) ([]*active.Group, error) {
	var groups []*active.Group
	err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`active.groups AS "group"`).
		Join(`LEFT JOIN active.group_supervisors AS "sup" ON "sup"."group_id" = "group"."id" AND ("sup"."end_date" IS NULL OR "sup"."end_date" > CURRENT_DATE)`).
		Join(`INNER JOIN facilities.rooms AS "room" ON "room"."id" = "group"."room_id"`).
		Where(`"group"."end_time" IS NULL`).
		Where(`"sup"."id" IS NULL`).
		Where(`"room"."name" = ?`, "Schulhof").
		Order("start_time DESC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find unclaimed groups", Err: err}
	}
	return groups, nil
}

// loadUnclaimedGroupRelations batch loads rooms and activity groups
func (r *GroupRepository) loadUnclaimedGroupRelations(ctx context.Context, groups []*active.Group) error {
	roomIDs, groupIDs := collectRelationIDs(groups)

	if err := r.loadAndAssignRooms(ctx, groups, roomIDs); err != nil {
		return err
	}

	if err := r.loadAndAssignActivityGroups(ctx, groups, groupIDs); err != nil {
		return err
	}

	return nil
}

// collectRelationIDs extracts unique room and group IDs
func collectRelationIDs(groups []*active.Group) (roomIDs, groupIDs []int64) {
	roomSeen := make(map[int64]bool)
	groupSeen := make(map[int64]bool)

	for _, g := range groups {
		if g.RoomID > 0 && !roomSeen[g.RoomID] {
			roomIDs = append(roomIDs, g.RoomID)
			roomSeen[g.RoomID] = true
		}
		if g.GroupID > 0 && !groupSeen[g.GroupID] {
			groupIDs = append(groupIDs, g.GroupID)
			groupSeen[g.GroupID] = true
		}
	}
	return roomIDs, groupIDs
}

// loadAndAssignRooms loads rooms and assigns them to groups
func (r *GroupRepository) loadAndAssignRooms(ctx context.Context, groups []*active.Group, roomIDs []int64) error {
	if len(roomIDs) == 0 {
		return nil
	}

	rooms, err := r.queryRoomsByIDs(ctx, roomIDs, "batch load rooms for unclaimed groups")
	if err != nil {
		return err
	}

	assignRoomsToGroups(groups, rooms)
	return nil
}

// loadAndAssignActivityGroups loads activity groups and assigns them
func (r *GroupRepository) loadAndAssignActivityGroups(ctx context.Context, groups []*active.Group, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}

	activityGroups, err := r.queryActivityGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return err
	}

	assignActivityGroupsToGroups(groups, activityGroups)
	return nil
}

// queryActivityGroupsByIDs fetches activity groups by their IDs
func (r *GroupRepository) queryActivityGroupsByIDs(ctx context.Context, ids []int64) ([]*activities.Group, error) {
	var groups []*activities.Group
	if err := r.db.NewSelect().
		Model(&groups).
		ModelTableExpr(`activities.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.In(ids)).
		Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "batch load activity groups for unclaimed groups", Err: err}
	}
	return groups, nil
}

// assignActivityGroupsToGroups assigns activity groups to active groups
func assignActivityGroupsToGroups(groups []*active.Group, activityGroups []*activities.Group) {
	agMap := make(map[int64]*activities.Group, len(activityGroups))
	for _, ag := range activityGroups {
		agMap[ag.ID] = ag
	}
	for _, g := range groups {
		if ag, ok := agMap[g.GroupID]; ok {
			g.ActualGroup = ag
		}
	}
}
