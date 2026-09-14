package active

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
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
		return nil, &activeService.ActiveError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return rows, nil
}

func (rs *Resource) presenceSessionVisits(ctx context.Context, groupID int64) ([]studentpresence.Visit, error) {
	const operation = "GetActiveGroupVisits"
	groups, err := rs.Presence.ListLiveGroups(ctx, []int64{groupID})
	if err != nil {
		return nil, &activeService.ActiveError{Op: operation, Err: activeService.ErrDatabaseOperation}
	}
	if len(groups) == 0 {
		return nil, &activeService.ActiveError{Op: operation, Err: activeService.ErrActiveGroupNotFound}
	}
	visits, err := rs.Presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{groupID}})
	if err != nil {
		return nil, &activeService.ActiveError{Op: operation, Err: activeService.ErrDatabaseOperation}
	}
	return visits, nil
}

func (rs *Resource) presenceRoomSessions(ctx context.Context, roomID int64) ([]studentpresence.LiveGroup, error) {
	rows, err := rs.Presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{RoomID: &roomID, OpenOnly: true})
	if err != nil {
		return nil, &activeService.ActiveError{Op: "FindActiveGroupsByRoomID", Err: fmt.Errorf("find by room: %w", err)}
	}
	return rows, nil
}

func (rs *Resource) presenceActivitySessions(ctx context.Context, activityID int64) ([]studentpresence.LiveGroup, error) {
	rows, err := rs.Presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{ActivityGroupIDs: []int64{activityID}, OpenOnly: true})
	if err != nil {
		return nil, &activeService.ActiveError{Op: "FindActiveGroupsByGroupID", Err: activeService.ErrDatabaseOperation}
	}
	return rows, nil
}

func (rs *Resource) presenceLiveGroup(ctx context.Context, id int64) (*studentpresence.LiveGroup, error) {
	rows, err := rs.Presence.ListLiveGroups(ctx, []int64{id})
	if err != nil {
		return nil, &activeService.ActiveError{Op: "GetActiveGroup", Err: activeService.ErrDatabaseOperation}
	}
	if len(rows) == 0 {
		return nil, &activeService.ActiveError{Op: "GetActiveGroup", Err: activeService.ErrActiveGroupNotFound}
	}
	return &rows[0], nil
}

func (rs *Resource) presenceCombination(ctx context.Context, id int64) (studentpresence.CombinedGroup, error) {
	row, err := rs.Presence.GetCombinedGroup(ctx, id)
	if err != nil {
		return studentpresence.CombinedGroup{}, &activeService.ActiveError{Op: "GetCombinedGroup", Err: activeService.ErrCombinedGroupNotFound}
	}
	return row, nil
}

func (rs *Resource) listPresenceCombinations(ctx context.Context, filter studentpresence.CombinedGroupFilter, operation string) ([]studentpresence.CombinedGroup, error) {
	rows, err := rs.Presence.ListCombinedGroups(ctx, filter)
	if err != nil {
		return nil, &activeService.ActiveError{Op: operation, Err: activeService.ErrDatabaseOperation}
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
			return &activeService.ActiveError{Op: operation, Err: activeService.ErrGroupAlreadyInCombination}
		}
	}
	if err := rs.Presence.AddGroupToCombination(ctx, combinedID, groupID); err != nil {
		return &activeService.ActiveError{Op: operation, Err: activeService.ErrDatabaseOperation}
	}
	return nil
}

func (rs *Resource) removePresenceGroupFromCombination(ctx context.Context, combinedID, groupID int64) error {
	if err := rs.Presence.RemoveGroupFromCombination(ctx, combinedID, groupID); err != nil {
		return &activeService.ActiveError{Op: "RemoveGroupFromCombination", Err: activeService.ErrDatabaseOperation}
	}
	return nil
}

func (rs *Resource) presenceGroupMappings(ctx context.Context, filter studentpresence.GroupMappingFilter, operation string) ([]studentpresence.GroupMapping, error) {
	rows, err := rs.Presence.ListGroupMappings(ctx, filter)
	if err != nil {
		return nil, &activeService.ActiveError{Op: operation, Err: activeService.ErrDatabaseOperation}
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
	cause := activeService.ErrDatabaseOperation
	if errors.Is(err, studentpresence.ErrVisitNotFound) {
		cause = activeService.ErrVisitNotFound
	}
	return &activeService.ActiveError{Op: operation, Err: cause}
}

func newPresenceVisitResponse(visit studentpresence.Visit) VisitResponse {
	return VisitResponse{
		ID: visit.ID, StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID,
		CheckInTime: visit.EntryTime, CheckOutTime: visit.ExitTime, IsActive: visit.ExitTime == nil,
		CreatedAt: visit.CreatedAt, UpdatedAt: visit.UpdatedAt,
	}
}
