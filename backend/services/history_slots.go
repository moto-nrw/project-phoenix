package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

type historySlots struct {
	source *timetableCompose.AttendanceHistory
}

func NewHistorySlots(records timetableCompose.AttendanceHistoryRecords) active.HistorySlotReader {
	if records == nil {
		return nil
	}
	return historySlots{source: timetableCompose.NewAttendanceHistory(records)}
}

func (r historySlots) Slots(ctx context.Context, studentID int64, from, to timezone.Date) ([]*active.HistorySlot, error) {
	rows, err := r.source.Slots(ctx, studentID, from.String(), to.String())
	if rows == nil {
		return nil, err
	}
	result := make([]*active.HistorySlot, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		slot := &active.HistorySlot{}
		if instance := row.Instance; instance != nil {
			slot.Instance = &active.HistorySlotInstance{ID: instance.ID, Date: timezone.Date(instance.Date), Title: instance.Title, Status: instance.Status, StartTime: instance.StartTime, EndTime: instance.EndTime}
		}
		if attendance := row.Attendance; attendance != nil {
			slot.Attendance = &active.HistorySlotAttendance{Status: attendance.Status, Substatus: attendance.Substatus, Note: attendance.Note, CheckedInAt: attendance.CheckedInAt, CheckedOutAt: attendance.CheckedOutAt, IsUnplanned: attendance.IsUnplanned}
		}
		result[i] = slot
	}
	return result, err
}

func (r historySlots) HasPlannedSlots(ctx context.Context, from, to timezone.Date) (bool, error) {
	return r.source.HasPlannedSlots(ctx, from.String(), to.String())
}
