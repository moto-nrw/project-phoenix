package application

import (
	"context"
	"time"

	models "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
func NativeMirrorParticipants(rows models.InstanceStudentRepository) *ports.MirrorParticipantRecords {
	if rows == nil {
		return nil
	}
	return &ports.MirrorParticipantRecords{MirrorParticipants: &ports.MirrorParticipants{FindByInstanceAndStudent: func(ctx context.Context, instanceID, studentID int64) (*timetable.ScheduledParticipant, error) {
		value, err := rows.FindByInstanceAndStudent(ctx, instanceID, studentID)
		return mirrorParticipant(value), err
	}, FindCurrentCandidates: func(ctx context.Context, studentID int64, date calendar.Date, at time.Time) ([]*timetable.ScheduledParticipant, error) {
		value, err := rows.FindCurrentCandidates(ctx, studentID, models.Date(date), at)
		return mirrorParticipants(value), err
	}, FindCurrentCandidatesByStudentIDs: func(ctx context.Context, studentIDs []int64, date calendar.Date, at time.Time) ([]*timetable.ScheduledParticipant, error) {
		value, err := rows.FindCurrentCandidatesByStudentIDs(ctx, studentIDs, models.Date(date), at)
		return mirrorParticipants(value), err
	}, FindByStudentAndDateRange: func(ctx context.Context, studentID int64, from, to calendar.Date) ([]*timetable.ScheduledParticipant, error) {
		value, err := rows.FindByStudentAndDateRange(ctx, studentID, models.Date(from), models.Date(to))
		return mirrorParticipants(value), err
	}, FindByStudentIDsAndDate: func(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*timetable.ScheduledParticipant, error) {
		value, err := rows.FindByStudentIDsAndDate(ctx, studentIDs, models.Date(date))
		return mirrorParticipants(value), err
	}, CreateUnplannedPresentIfAbsent: func(ctx context.Context, instanceID, studentID int64, at time.Time) (*timetable.ScheduledParticipant, error) {
		value, err := rows.CreateUnplannedPresentIfAbsent(ctx, instanceID, studentID, at)
		return mirrorParticipant(value), err
	}, UpdateAttendanceFromCheckin: rows.UpdateAttendanceFromCheckin, UpdateAttendanceFromCheckinBatch: func(ctx context.Context, keys []timetable.InstanceStudentKey, at time.Time) error {
		return rows.UpdateAttendanceFromCheckinBatch(ctx, mirrorLegacyKeys(keys), at)
	}, UpdateAttendanceCheckout: rows.UpdateAttendanceCheckout, UpdateAttendanceCheckoutBatch: func(ctx context.Context, keys []timetable.InstanceStudentKey, at time.Time) error {
		return rows.UpdateAttendanceCheckoutBatch(ctx, mirrorLegacyKeys(keys), at)
	}, ReconcileAttendanceInterval: rows.ReconcileAttendanceInterval}, Create: func(ctx context.Context, value *timetable.ScheduledParticipant) error {
		row := mirrorParticipantRow(value)
		err := rows.Create(ctx, row)
		if value != nil && row != nil {
			*value = *mirrorParticipant(row)
		}
		return err
	}, FindByID: func(ctx context.Context, id any) (*timetable.ScheduledParticipant, error) {
		row, err := rows.FindByID(ctx, id)
		return mirrorParticipant(row), err
	}, FindInstancesWithAttendanceByStudentAndDateRange: func(ctx context.Context, studentID int64, from, to calendar.Date) ([]*timetable.StudentWeekEntry, error) {
		rows, err := rows.FindInstancesWithAttendanceByStudentAndDateRange(ctx, studentID, models.Date(from), models.Date(to))
		return mirrorWeekEntries(rows), err
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
