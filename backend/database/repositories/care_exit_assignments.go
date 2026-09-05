package repositories

import (
	"context"
	"time"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type careExitAssignments struct {
	capability timetable.InstanceStudentCapability
}

func (d careExitAssignments) CountPlannedStudentAssignmentsAfter(ctx context.Context, ids []int64, after string, removals []usersRepo.CareExitRemoval) (map[int64]int, error) {
	return d.capability.CountPlannedStudentAssignmentsAfter(ctx, ids, after, publicCareExitAssignments(removals))
}

func (d careExitAssignments) RemovePlannedStudentAssignmentsAfter(ctx context.Context, ids []int64, after string) ([]usersRepo.CareExitRemoval, error) {
	rows, err := d.capability.RemovePlannedStudentAssignmentsAfter(ctx, ids, after)
	if err != nil {
		return nil, err
	}
	result := make([]usersRepo.CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		result = append(result, usersRepo.CareExitRemoval{
			TenantID: row.TenantID, StudentID: row.StudentID, Kind: usersRepo.CareExitRemovalRoster,
			InstanceID: &row.InstanceID, RoomID: row.RoomID, Status: &row.Status, Substatus: row.Substatus, Note: row.Note,
			IsUnplanned: &row.IsUnplanned, NotScheduled: &row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
			StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
		})
	}
	return result, nil
}

func (d careExitAssignments) RestoreCareExitStudentAssignments(ctx context.Context, ids, rooms, statuses, pickups []int64, removals []usersRepo.CareExitRemoval) (int64, error) {
	return d.capability.RestoreCareExitStudentAssignments(ctx, ids, rooms, statuses, pickups, publicCareExitAssignments(removals))
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

func (d careExitAssignments) LockPlannedStudentAssignmentsAfter(ctx context.Context, ids []int64, after string) error {
	return d.capability.LockPlannedStudentAssignmentsAfter(ctx, ids, after)
}

func (d careExitAssignments) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, ids, pickups []int64, removals []usersRepo.CareExitRemoval) error {
	return d.capability.ReconnectCareExitAssignmentPickupExceptions(ctx, ids, pickups, publicCareExitAssignments(removals))
}

func publicCareExitAssignments(removals []usersRepo.CareExitRemoval) []timetable.InstanceStudent {
	result := make([]timetable.InstanceStudent, 0, len(removals))
	for _, row := range removals {
		if row.Kind != usersRepo.CareExitRemovalRoster {
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
