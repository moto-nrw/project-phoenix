package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (e engine) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("timetable: assignment locks require a transaction")
	}
	return mapError(e.service.LockInstanceStudentAssignments(ctx, instanceID))
}

func (e engine) ListPlannedInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) ([]timetable.PlannedInstanceStudent, error) {
	values, err := e.service.ListPlannedInstanceStudents(ctx, domain.InstanceStudentFilter(filter))
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.PlannedInstanceStudent, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.PlannedInstanceStudent(value))
	}
	return result, nil
}

func (e engine) EnsureInstanceStudent(ctx context.Context, instanceID, studentID int64) (timetable.InstanceStudent, bool, error) {
	value, inserted, err := e.service.EnsureInstanceStudent(ctx, instanceID, studentID)
	return instanceStudentToPublic(value), inserted, mapError(err)
}

func (e engine) ArchivePlannedInstanceStudents(ctx context.Context, transitionID int64, entries []timetable.RosterArchiveEntry) (int, error) {
	values := make([]domain.RosterArchiveEntry, 0, len(entries))
	for _, entry := range entries {
		values = append(values, domain.RosterArchiveEntry{ParticipantID: entry.ParticipantID, Attendance: domain.ArchivedAttendance(entry.Attendance)})
	}
	rows, err := e.service.ArchivePlannedInstanceStudents(ctx, transitionID, values)
	return rows, mapError(err)
}

func (e engine) RestoreArchivedInstanceStudents(ctx context.Context, transitionID int64, studentIDs []int64, from string) ([]timetable.RestoredInstanceStudent, error) {
	values, err := e.service.RestoreArchivedInstanceStudents(ctx, transitionID, studentIDs, from)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]timetable.RestoredInstanceStudent, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.RestoredInstanceStudent{
			ID: value.ID, InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID,
			Date: value.Date, StartTime: value.StartTime, Attendance: timetable.ArchivedAttendance(value.Attendance),
		})
	}
	return result, nil
}
