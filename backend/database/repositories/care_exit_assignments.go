package repositories

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/carelifecycle"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type careExitAssignments struct {
	capability timetable.InstanceStudentCapability
}

func (d careExitAssignments) ListOpenStudentAssignments(ctx context.Context, ids []int64) ([]int64, error) {
	return d.capability.ListOpenStudentAssignments(ctx, ids)
}

func (d careExitAssignments) LatestStudentAssignmentAttendanceDate(ctx context.Context, id int64) (*string, error) {
	return d.capability.LatestStudentAssignmentAttendanceDate(ctx, id)
}

func (d careExitAssignments) CloseOpenStudentAssignments(ctx context.Context, ids []int64, at time.Time) (int64, error) {
	return d.capability.CloseOpenStudentAssignments(ctx, ids, at)
}

func (d careExitAssignments) LockOpenStudentAssignments(ctx context.Context, ids []int64) error {
	return d.capability.LockOpenStudentAssignments(ctx, ids)
}

func (d careExitAssignments) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, ids, pickups []int64, removals []carelifecycle.CareExitRemoval) error {
	return d.capability.ReconnectCareExitAssignmentPickupExceptions(ctx, ids, pickups, publicCareExitAssignments(removals))
}

func publicCareExitAssignments(removals []carelifecycle.CareExitRemoval) []timetable.InstanceStudent {
	result := make([]timetable.InstanceStudent, 0, len(removals))
	for _, row := range removals {
		if row.Kind != carelifecycle.CareExitRemovalRoster {
			continue
		}
		assignment := timetable.InstanceStudent{TenantID: row.TenantID, StudentID: row.StudentID,
			RoomID: row.RoomID, Substatus: row.Substatus, Note: row.Note, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID}
		if row.InstanceID != nil {
			assignment.InstanceID = *row.InstanceID
		}
		if row.Status != nil {
			assignment.Status = *row.Status
		}
		if row.IsUnplanned != nil {
			assignment.IsUnplanned = *row.IsUnplanned
		}
		if row.NotScheduled != nil {
			assignment.NotScheduled = *row.NotScheduled
		}
		result = append(result, assignment)
	}
	return result
}
