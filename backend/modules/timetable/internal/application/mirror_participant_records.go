package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	models "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func mirrorParticipant(row *models.InstanceStudent) *timetable.ScheduledParticipant {
	if row == nil {
		return nil
	}
	value := ScheduledParticipantOf(row)
	return &value
}
func mirrorParticipants(rows []*models.InstanceStudent) []*timetable.ScheduledParticipant {
	if rows == nil {
		return nil
	}
	out := make([]*timetable.ScheduledParticipant, len(rows))
	for i, row := range rows {
		out[i] = mirrorParticipant(row)
	}
	return out
}
func mirrorLegacyKeys(keys []timetable.InstanceStudentKey) []models.InstanceStudentKey {
	if keys == nil {
		return nil
	}
	out := make([]models.InstanceStudentKey, len(keys))
	for i, key := range keys {
		out[i] = models.InstanceStudentKey{InstanceID: key.InstanceID, StudentID: key.StudentID}
	}
	return out
}
func mirrorParticipantRow(value *timetable.ScheduledParticipant) *models.InstanceStudent {
	if value == nil {
		return nil
	}
	row := &models.InstanceStudent{InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID, Status: value.Status, Substatus: value.Substatus, Note: value.Note, CheckedInAt: value.CheckedInAt, CheckedOutAt: value.CheckedOutAt, IsUnplanned: value.IsUnplanned, NotScheduled: value.NotScheduled, ManualStatusAt: value.ManualStatusAt, StudentStatusDayID: value.StudentStatusDayID, PickupExceptionID: value.PickupExceptionID}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}
func NativeMirrorParticipants(rows ports.ParticipantRows) *ports.MirrorParticipantRecords {
	if noParticipantRows(rows) {
		return nil
	}
	return &ports.MirrorParticipantRecords{MirrorParticipants: &ports.MirrorParticipants{FindByInstanceAndStudent: func(ctx context.Context, instanceID, studentID int64) (*timetable.ScheduledParticipant, error) {
		value, err := participantOf(ctx, rows.Reads, instanceID, studentID)
		return mirrorParticipant(value), err
	}, FindCurrentCandidates: func(ctx context.Context, studentID int64, date calendar.Date, at time.Time) ([]*timetable.ScheduledParticipant, error) {
		value, err := currentCandidates(ctx, rows.Reads, studentID, date, at)
		return mirrorParticipants(value), err
	}, FindCurrentCandidatesByStudentIDs: func(ctx context.Context, studentIDs []int64, date calendar.Date, at time.Time) ([]*timetable.ScheduledParticipant, error) {
		value, err := currentCandidatesOfStudents(ctx, rows.Reads, studentIDs, date, at)
		return mirrorParticipants(value), err
	}, FindByStudentAndDateRange: func(ctx context.Context, studentID int64, from, to calendar.Date) ([]*timetable.ScheduledParticipant, error) {
		value, err := participantsOfStudentBetween(ctx, rows.Reads, studentID, from, to)
		return mirrorParticipants(value), err
	}, FindByStudentIDsAndDate: func(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*timetable.ScheduledParticipant, error) {
		value, err := participantsOfStudentsOn(ctx, rows.Reads, studentIDs, date)
		return mirrorParticipants(value), err
	}, CreateUnplannedPresentIfAbsent: func(ctx context.Context, instanceID, studentID int64, at time.Time) (*timetable.ScheduledParticipant, error) {
		value, err := createUnplannedPresentIfAbsent(ctx, rows, instanceID, studentID, at)
		return mirrorParticipant(value), err
	}, UpdateAttendanceFromCheckin: func(ctx context.Context, instanceID, studentID int64, at time.Time) (bool, error) {
		return checkInParticipant(ctx, rows, instanceID, studentID, at)
	}, UpdateAttendanceFromCheckinBatch: func(ctx context.Context, keys []timetable.InstanceStudentKey, at time.Time) error {
		return checkInParticipants(ctx, rows, mirrorLegacyKeys(keys), at)
	}, UpdateAttendanceCheckout: func(ctx context.Context, instanceID, studentID int64, at time.Time) error {
		return checkOutParticipant(ctx, rows, instanceID, studentID, at)
	}, UpdateAttendanceCheckoutBatch: func(ctx context.Context, keys []timetable.InstanceStudentKey, at time.Time) error {
		return checkOutParticipants(ctx, rows, mirrorLegacyKeys(keys), at)
	}, ReconcileAttendanceInterval: func(ctx context.Context, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (bool, error) {
		return reconcileAttendanceInterval(ctx, rows, instanceID, studentID, previousCheckIn, previousCheckOut, updatedCheckIn, updatedCheckOut)
	}}, Create: func(ctx context.Context, value *timetable.ScheduledParticipant) error {
		row := mirrorParticipantRow(value)
		err := createLegacyParticipant(ctx, rows, row)
		if value != nil && row != nil {
			*value = *mirrorParticipant(row)
		}
		return err
	}, FindByID: func(ctx context.Context, id any) (*timetable.ScheduledParticipant, error) {
		recordID, ok := LegacyGroupID(id)
		if !ok {
			return nil, WrapDatabaseError("find by id", fmt.Errorf("invalid instance student id %T", id))
		}
		row, err := findLegacyParticipant(ctx, rows.Reads, recordID)
		return mirrorParticipant(row), err
	}, FindInstancesWithAttendanceByStudentAndDateRange: func(ctx context.Context, studentID int64, from, to calendar.Date) ([]*timetable.StudentWeekEntry, error) {
		entries, err := scheduledInstancesForStudent(ctx, rows.Reads, studentID, from, to)
		return mirrorWeekEntries(entries), err
	}}
}

func mirrorWeekEntries(rows []*models.ScheduledInstanceRow) []*timetable.StudentWeekEntry {
	if rows == nil {
		return nil
	}
	out := make([]*timetable.StudentWeekEntry, len(rows))
	for i, row := range rows {
		if row != nil {
			value := &timetable.StudentWeekEntry{}
			if row.Instance != nil {
				value.Instance = ScheduledInstanceOf(row.Instance)
			}
			if row.Attendance != nil {
				value.Attendance = ScheduledParticipantOf(row.Attendance)
			}
			out[i] = value
		}
	}
	return out
}
