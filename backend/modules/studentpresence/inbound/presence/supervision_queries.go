package presence

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (rs *Resource) presenceSupervisionResponses(ctx context.Context, filter studentpresence.GroupSupervisionFilter, operation string) ([]SupervisorResponse, error) {
	rows, err := rs.Presence.QueryGroupSupervisions(ctx, filter)
	if err != nil {
		return nil, &presenceError{Op: operation, Err: studentpresence.ErrDatabaseOperation}
	}
	responses := make([]SupervisorResponse, 0, len(rows))
	for _, row := range rows {
		response, err := presenceSupervisionResponse(row, timezone.TodayDate())
		if err != nil {
			return nil, &presenceError{Op: operation, Err: studentpresence.ErrDatabaseOperation}
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func (rs *Resource) presenceSessionSupervisors(ctx context.Context, groupID int64, today timezone.Date) ([]SupervisorResponse, error) {
	const operation = "GetActiveGroupWithSupervisors"
	groups, err := rs.Presence.ListLiveGroups(ctx, []int64{groupID})
	if err != nil || len(groups) == 0 {
		return nil, &presenceError{Op: operation, Err: studentpresence.ErrGroupNotFound}
	}
	day := today.String()
	return rs.presenceSupervisionResponses(ctx, studentpresence.GroupSupervisionFilter{GroupIDs: []int64{groupID}, ActiveOn: &day}, operation)
}

func presenceSupervisionResponse(row studentpresence.GroupSupervision, today timezone.Date) (SupervisorResponse, error) {
	start, err := timezone.ParseDate(row.StartDate)
	if err != nil {
		return SupervisorResponse{}, err
	}
	isActive := !start.After(today)
	var endTime *time.Time
	if row.EndDate != nil {
		end, err := timezone.ParseDate(*row.EndDate)
		if err != nil {
			return SupervisorResponse{}, err
		}
		at := end.UTCMidnight()
		endTime = &at
		isActive = isActive && today.Before(end)
	}
	return SupervisorResponse{
		ID: row.ID, StaffID: row.StaffID, ActiveGroupID: row.GroupID,
		StartTime: start.UTCMidnight(), EndTime: endTime, IsActive: isActive,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (rs *Resource) presenceSupervisor(ctx context.Context, id int64) (SupervisorResponse, error) {
	const operation = "GetGroupSupervisor"
	rows, err := rs.presenceSupervisionResponses(ctx, studentpresence.GroupSupervisionFilter{IDs: []int64{id}}, operation)
	if err != nil || len(rows) == 0 {
		return SupervisorResponse{}, &presenceError{Op: operation, Err: studentpresence.ErrGroupSupervisorNotFound}
	}
	return rows[0], nil
}

// presenceSupervisionRow returns the persisted supervision record the update
// route revises. A missing record fails as not found.
func (rs *Resource) presenceSupervisionRow(ctx context.Context, id int64) (studentpresence.GroupSupervision, error) {
	const operation = "GetGroupSupervisor"
	rows, err := rs.Presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{IDs: []int64{id}})
	if err != nil || len(rows) == 0 {
		return studentpresence.GroupSupervision{}, &presenceError{Op: operation, Err: studentpresence.ErrGroupSupervisorNotFound}
	}
	return rows[0], nil
}

// supervisionRowResponse renders a persisted supervision row when the
// trailing re-read after a write fails.
func supervisionRowResponse(row studentpresence.GroupSupervision) SupervisorResponse {
	response, err := presenceSupervisionResponse(row, timezone.TodayDate())
	if err != nil {
		return SupervisorResponse{ID: row.ID, StaffID: row.StaffID, ActiveGroupID: row.GroupID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	}
	return response
}
