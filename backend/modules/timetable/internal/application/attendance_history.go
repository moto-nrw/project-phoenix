package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type AttendanceHistory struct {
	reads ports.LegacyParticipantReads
}

func NewAttendanceHistory(reads ports.LegacyParticipantReads) *AttendanceHistory {
	return &AttendanceHistory{reads: reads}
}

func (r *AttendanceHistory) Slots(ctx context.Context, studentID int64, from, to string) ([]*ports.AttendanceHistorySlot, error) {
	rows, err := scheduledInstancesForStudent(ctx, r.reads, studentID, calendar.Date(from), calendar.Date(to))
	if rows == nil {
		return nil, err
	}
	result := make([]*ports.AttendanceHistorySlot, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		slot := &ports.AttendanceHistorySlot{}
		if instance := row.Instance; instance != nil {
			slot.Instance = &ports.AttendanceHistoryInstance{
				ID: instance.ID, Date: instance.Date.String(), Title: instance.Title, Status: instance.Status,
				StartTime: instance.StartTime, EndTime: instance.EndTime,
			}
		}
		if attendance := row.Attendance; attendance != nil {
			slot.Attendance = &ports.AttendanceHistoryPresence{
				Status: attendance.Status, Substatus: attendance.Substatus, Note: attendance.Note,
				CheckedInAt: attendance.CheckedInAt, CheckedOutAt: attendance.CheckedOutAt, IsUnplanned: attendance.IsUnplanned,
			}
		}
		result[i] = slot
	}
	return result, err
}

func (r *AttendanceHistory) HasPlannedSlots(ctx context.Context, from, to string) (bool, error) {
	result, err := r.reads.HasPlannedSlotsInRange(ctx, calendar.Date(from).String(), calendar.Date(to).String())
	if err != nil {
		return false, WrapDatabaseError("check planned slots in range", err)
	}
	return result, nil
}
