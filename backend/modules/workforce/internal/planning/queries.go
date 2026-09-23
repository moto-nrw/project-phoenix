package planning

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The read-only slices of the Dienstplan that consumers bind on their own,
// without the write facade: the self-service "Mein Tag" assignments and the
// week grid the printable plan is built from. Both speak the public string
// dates and wall clocks and report the same error kinds the planning facade
// does, so a caller classifies one set of sentinels (#3418).

type staffAssignmentQuery struct{ assignments StaffAssignmentService }

// NewStaffAssignmentQuery binds the public assignment read to the service.
func NewStaffAssignmentQuery(assignments StaffAssignmentService) workforce.StaffAssignmentQuery {
	if assignments == nil {
		panic("staff assignment query: the assignment service is required")
	}
	return staffAssignmentQuery{assignments: assignments}
}

func (q staffAssignmentQuery) ListStaffAssignments(ctx context.Context, staffID int64, from, to string) ([]workforce.StaffAssignment, error) {
	fromDate, err := planningDate(from, "from")
	if err != nil {
		return nil, err
	}
	toDate, err := planningDate(to, "to")
	if err != nil {
		return nil, err
	}
	assignments, err := q.assignments.ListAssignmentsForStaff(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapPlanningError(err)
	}
	return assignmentsToCapability(assignments), nil
}

func assignmentsToCapability(assignments []*StaffAssignment) []workforce.StaffAssignment {
	result := make([]workforce.StaffAssignment, 0, len(assignments))
	for _, assignment := range assignments {
		if assignment == nil {
			continue
		}
		result = append(result, workforce.StaffAssignment{
			InstanceID: assignment.InstanceID, Title: assignment.Title, GroupName: assignment.GroupName,
			RoomName: assignment.RoomName, Date: assignment.Date.String(),
			StartTime: clockString(assignment.StartTime), EndTime: clockString(assignment.EndTime),
			Status: assignment.Status, Cancelled: assignment.Cancelled, IsPrimary: assignment.IsPrimary,
			IsSubstitute: assignment.IsSubstitute, IsAbsent: assignment.IsAbsent,
			AbsenceReason: assignment.AbsenceReason, CancelReason: assignment.CancelReason,
			UnderstaffedAck: assignment.UnderstaffedAck,
		})
	}
	return result
}

type staffScheduleOverviewQuery struct{ overview StaffScheduleOverviewGetter }

// NewStaffScheduleOverviewQuery binds the public week-grid read to the
// overview service.
func NewStaffScheduleOverviewQuery(overview StaffScheduleOverviewGetter) workforce.StaffScheduleOverviewQuery {
	if overview == nil {
		panic("staff schedule overview query: the overview service is required")
	}
	return staffScheduleOverviewQuery{overview: overview}
}

func (q staffScheduleOverviewQuery) Overview(ctx context.Context, from, to string) (workforce.StaffScheduleOverview, error) {
	fromDate, err := planningDate(from, "from")
	if err != nil {
		return workforce.StaffScheduleOverview{}, err
	}
	toDate, err := planningDate(to, "to")
	if err != nil {
		return workforce.StaffScheduleOverview{}, err
	}
	overview, err := q.overview.GetOverview(ctx, fromDate, toDate)
	if err != nil {
		return workforce.StaffScheduleOverview{}, mapPlanningError(err)
	}
	return overviewToCapability(overview), nil
}
