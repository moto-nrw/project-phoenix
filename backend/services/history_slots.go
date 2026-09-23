package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

type historySlots struct {
	source *timetableCompose.AttendanceHistory
}

func NewHistorySlots(records timetableCompose.AttendanceHistoryRecords) presenceservice.HistorySlotReader {
	if records == nil {
		return nil
	}
	return historySlots{source: timetableCompose.NewAttendanceHistory(records)}
}

func (r historySlots) Slots(ctx context.Context, studentID int64, from, to timezone.Date) ([]*studentpresence.HistorySlot, error) {
	rows, err := r.source.Slots(ctx, studentID, from.String(), to.String())
	if rows == nil {
		return nil, err
	}
	result := make([]*studentpresence.HistorySlot, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		slot := &studentpresence.HistorySlot{}
		if instance := row.Instance; instance != nil {
			slot.Instance = &studentpresence.HistorySlotInstance{ID: instance.ID, Date: timezone.Date(instance.Date), Title: instance.Title, Status: instance.Status, StartTime: instance.StartTime, EndTime: instance.EndTime}
		}
		if attendance := row.Attendance; attendance != nil {
			slot.Attendance = &studentpresence.HistorySlotAttendance{Status: attendance.Status, Substatus: attendance.Substatus, Note: attendance.Note, CheckedInAt: attendance.CheckedInAt, CheckedOutAt: attendance.CheckedOutAt, IsUnplanned: attendance.IsUnplanned}
		}
		result[i] = slot
	}
	return result, err
}

func (r historySlots) HasPlannedSlots(ctx context.Context, from, to timezone.Date) (bool, error) {
	return r.source.HasPlannedSlots(ctx, from.String(), to.String())
}
