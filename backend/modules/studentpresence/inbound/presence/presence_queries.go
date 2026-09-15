package presence

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type PresenceQueries interface {
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
	QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	ListLiveGroups(context.Context, []int64) ([]studentpresence.LiveGroup, error)
	GetCombinedGroup(context.Context, int64) (studentpresence.CombinedGroup, error)
	ListCombinedGroups(context.Context, studentpresence.CombinedGroupFilter) ([]studentpresence.CombinedGroup, error)
	AddGroupToCombination(context.Context, int64, int64) error
	RemoveGroupFromCombination(context.Context, int64, int64) error
	ListGroupMappings(context.Context, studentpresence.GroupMappingFilter) ([]studentpresence.GroupMapping, error)
	FindVisit(context.Context, int64) (*studentpresence.Visit, error)
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}

func (rs *Resource) listPresenceLiveGroups(ctx context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error) {
	rows, err := rs.Presence.QueryLiveGroups(ctx, filter)
	if err != nil {
		return nil, &presenceError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return rows, nil
}

func (rs *Resource) presenceSessionVisits(ctx context.Context, groupID int64) ([]studentpresence.Visit, error) {
	const operation = "GetActiveGroupVisits"
	groups, err := rs.Presence.ListLiveGroups(ctx, []int64{groupID})
	if err != nil {
		return nil, &presenceError{Op: operation, Err: studentpresence.ErrDatabaseOperation}
	}
	if len(groups) == 0 {
		return nil, &presenceError{Op: operation, Err: studentpresence.ErrGroupNotFound}
	}
	visits, err := rs.Presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{groupID}})
	if err != nil {
		return nil, &presenceError{Op: operation, Err: studentpresence.ErrDatabaseOperation}
	}
	return visits, nil
}

func (rs *Resource) presenceRoomSessions(ctx context.Context, roomID int64) ([]studentpresence.LiveGroup, error) {
	rows, err := rs.Presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{RoomID: &roomID, OpenOnly: true})
	if err != nil {
		return nil, &presenceError{Op: "FindActiveGroupsByRoomID", Err: fmt.Errorf("find by room: %w", err)}
	}
	return rows, nil
}

func (rs *Resource) presenceActivitySessions(ctx context.Context, activityID int64) ([]studentpresence.LiveGroup, error) {
	rows, err := rs.Presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{ActivityGroupIDs: []int64{activityID}, OpenOnly: true})
	if err != nil {
		return nil, &presenceError{Op: "FindActiveGroupsByGroupID", Err: studentpresence.ErrDatabaseOperation}
	}
	return rows, nil
}

func (rs *Resource) presenceLiveGroup(ctx context.Context, id int64) (*studentpresence.LiveGroup, error) {
	rows, err := rs.Presence.ListLiveGroups(ctx, []int64{id})
	if err != nil {
		return nil, &presenceError{Op: "GetActiveGroup", Err: studentpresence.ErrDatabaseOperation}
	}
	if len(rows) == 0 {
		return nil, &presenceError{Op: "GetActiveGroup", Err: studentpresence.ErrGroupNotFound}
	}
	return &rows[0], nil
}

// sessionResponse renders one session with its room summary, the shape the
// single-session routes answer with.
func (rs *Resource) sessionResponse(ctx context.Context, id int64) (ActiveGroupResponse, error) {
	group, err := rs.presenceLiveGroup(ctx, id)
	if err != nil {
		return ActiveGroupResponse{}, err
	}
	response := newPresenceLiveGroupResponse(*group)
	if group.RoomID > 0 {
		if room, ok := rs.loadRoomsMap(ctx, []studentpresence.LiveGroup{*group})[group.RoomID]; ok {
			response.Room = newSessionRoomResponse(room)
		}
	}
	return response, nil
}

func (rs *Resource) presenceCombination(ctx context.Context, id int64) (studentpresence.CombinedGroup, error) {
	row, err := rs.Presence.GetCombinedGroup(ctx, id)
	if err != nil {
		return studentpresence.CombinedGroup{}, &presenceError{Op: "GetCombinedGroup", Err: studentpresence.ErrCombinedGroupNotFound}
	}
	return row, nil
}

// presenceCombinationGroups returns the sessions mapped into a combination.
// A missing combination fails as not found; mapped sessions that no longer
// exist are skipped.
func (rs *Resource) presenceCombinationGroups(ctx context.Context, id int64) (studentpresence.CombinedGroup, []studentpresence.LiveGroup, error) {
	const operation = "GetCombinedGroupWithGroups"
	combination, err := rs.Presence.GetCombinedGroup(ctx, id)
	if err != nil {
		return studentpresence.CombinedGroup{}, nil, &presenceError{Op: operation, Err: studentpresence.ErrCombinedGroupNotFound}
	}
	mappings, err := rs.Presence.ListGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &id})
	if err != nil {
		return studentpresence.CombinedGroup{}, nil, &presenceError{Op: operation, Err: studentpresence.ErrCombinedGroupNotFound}
	}
	ids := make([]int64, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping.ActiveGroupID > 0 {
			ids = append(ids, mapping.ActiveGroupID)
		}
	}
	rows, err := rs.Presence.ListLiveGroups(ctx, ids)
	if err != nil {
		return studentpresence.CombinedGroup{}, nil, &presenceError{Op: operation, Err: studentpresence.ErrCombinedGroupNotFound}
	}
	byID := make(map[int64]studentpresence.LiveGroup, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	groups := make([]studentpresence.LiveGroup, 0, len(mappings))
	for _, mapping := range mappings {
		if group, ok := byID[mapping.ActiveGroupID]; ok {
			groups = append(groups, group)
		}
	}
	return combination, groups, nil
}

func (rs *Resource) listPresenceCombinations(ctx context.Context, filter studentpresence.CombinedGroupFilter, operation string) ([]studentpresence.CombinedGroup, error) {
	rows, err := rs.Presence.ListCombinedGroups(ctx, filter)
	if err != nil {
		return nil, &presenceError{Op: operation, Err: studentpresence.ErrDatabaseOperation}
	}
	return rows, nil
}

func (rs *Resource) addPresenceGroupToCombination(ctx context.Context, combinedID, groupID int64) error {
	const operation = "AddGroupToCombination"
	mappings, err := rs.presenceGroupMappings(ctx, studentpresence.GroupMappingFilter{CombinedGroupID: &combinedID}, operation)
	if err != nil {
		return err
	}
	for _, mapping := range mappings {
		if mapping.ActiveGroupID == groupID {
			return &presenceError{Op: operation, Err: studentpresence.ErrGroupAlreadyInCombination}
		}
	}
	if err := rs.Presence.AddGroupToCombination(ctx, combinedID, groupID); err != nil {
		return &presenceError{Op: operation, Err: studentpresence.ErrDatabaseOperation}
	}
	return nil
}

func (rs *Resource) removePresenceGroupFromCombination(ctx context.Context, combinedID, groupID int64) error {
	if err := rs.Presence.RemoveGroupFromCombination(ctx, combinedID, groupID); err != nil {
		return &presenceError{Op: "RemoveGroupFromCombination", Err: studentpresence.ErrDatabaseOperation}
	}
	return nil
}

func (rs *Resource) presenceGroupMappings(ctx context.Context, filter studentpresence.GroupMappingFilter, operation string) ([]studentpresence.GroupMapping, error) {
	rows, err := rs.Presence.ListGroupMappings(ctx, filter)
	if err != nil {
		return nil, &presenceError{Op: operation, Err: studentpresence.ErrDatabaseOperation}
	}
	return rows, nil
}

func newPresenceGroupMappingResponse(mapping studentpresence.GroupMapping) GroupMappingResponse {
	return GroupMappingResponse{ID: mapping.ID, ActiveGroupID: mapping.ActiveGroupID, CombinedGroupID: mapping.ActiveCombinedGroupID}
}

func (rs *Resource) findPresenceVisit(ctx context.Context, id int64) (*studentpresence.Visit, error) {
	visit, err := rs.Presence.FindVisit(ctx, id)
	if err == nil && visit == nil {
		err = studentpresence.ErrVisitNotFound
	}
	return visit, presenceQueryError("GetVisit", err)
}

func (rs *Resource) currentPresenceVisit(ctx context.Context, studentID int64) (*studentpresence.Visit, error) {
	visits, err := rs.Presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{studentID}, OpenOnly: true, NewestFirst: true, Limit: 1})
	if err != nil {
		return nil, presenceQueryError("GetStudentCurrentVisit", err)
	}
	if len(visits) == 0 {
		return nil, presenceQueryError("GetStudentCurrentVisit", studentpresence.ErrVisitNotFound)
	}
	return &visits[0], nil
}

func presenceQueryError(operation string, err error) error {
	if err == nil {
		return nil
	}
	cause := studentpresence.ErrDatabaseOperation
	if errors.Is(err, studentpresence.ErrVisitNotFound) {
		cause = studentpresence.ErrVisitNotFound
	}
	return &presenceError{Op: operation, Err: cause}
}

func newPresenceVisitResponse(visit studentpresence.Visit) VisitResponse {
	return VisitResponse{
		ID: visit.ID, StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID,
		CheckInTime: visit.EntryTime, CheckOutTime: visit.ExitTime, IsActive: visit.ExitTime == nil,
		CreatedAt: visit.CreatedAt, UpdatedAt: visit.UpdatedAt,
	}
}
