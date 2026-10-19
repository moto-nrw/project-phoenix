package api

import (
	"context"
	"fmt"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func FacilitiesGroupSupervisions(presence interface {
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}) func(context.Context, []int64) ([]facilitiesCompose.OpenGroupSupervisor, error) {
	return func(ctx context.Context, groupIDs []int64) ([]facilitiesCompose.OpenGroupSupervisor, error) {
		result := make([]facilitiesCompose.OpenGroupSupervisor, 0)
		if len(groupIDs) == 0 {
			return result, nil
		}
		today := timezone.TodayDate()
		day := today.String()
		rows, err := presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{GroupIDs: groupIDs, ActiveOn: &day})
		if err != nil {
			return nil, &studentpresence.OperationError{Op: "FindSupervisorsByActiveGroupIDs", Err: studentpresence.ErrDatabaseOperation}
		}
		for _, row := range rows {
			result = append(result, facilitiesCompose.OpenGroupSupervisor{ID: row.ID, GroupID: row.GroupID, StaffID: row.StaffID, Ended: row.EndedOn(today)})
		}
		return result, nil
	}
}

func FacilitiesGroupVisits(presence interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}) func(context.Context, int64) ([]facilitiesCompose.OpenGroupVisit, error) {
	return func(ctx context.Context, groupID int64) ([]facilitiesCompose.OpenGroupVisit, error) {
		visits, err := presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: []int64{groupID}})
		if err != nil {
			return nil, &studentpresence.OperationError{Op: "FindVisitsByActiveGroupID", Err: studentpresence.ErrDatabaseOperation}
		}
		result := make([]facilitiesCompose.OpenGroupVisit, 0, len(visits))
		for _, visit := range visits {
			result = append(result, facilitiesCompose.OpenGroupVisit{ExitTime: visit.ExitTime})
		}
		return result, nil
	}
}

func FacilitiesRoomSessions(presence interface {
	QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
}) func(context.Context, int64) ([]facilitiesCompose.OpenGroup, error) {
	return func(ctx context.Context, roomID int64) ([]facilitiesCompose.OpenGroup, error) {
		groups, err := presence.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{RoomID: &roomID, OpenOnly: true})
		if err != nil {
			return nil, &studentpresence.OperationError{Op: "FindActiveGroupsByRoomID", Err: fmt.Errorf("find by room: %w", err)}
		}
		today := timezone.TodayDate()
		result := make([]facilitiesCompose.OpenGroup, 0, len(groups))
		for _, group := range groups {
			result = append(result, facilitiesCompose.OpenGroup{
				ID: group.ID, StartTime: group.StartTime, EndTime: group.EndTime,
				IsToday: group.StartedOn(today),
			})
		}
		return result, nil
	}
}
