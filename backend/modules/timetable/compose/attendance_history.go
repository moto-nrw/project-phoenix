package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
)

type AttendanceHistoryRecords interface {
	FindInstancesWithAttendanceByStudentAndDateRange(context.Context, int64, schedule.Date, schedule.Date) ([]*schedule.ScheduledInstanceRow, error)
	HasPlannedSlotsInRange(context.Context, schedule.Date, schedule.Date) (bool, error)
}

// AttendanceHistorySlot contains only the timetable facts used in student history.
type AttendanceHistorySlot struct {
	Instance   *AttendanceHistoryInstance
	Attendance *AttendanceHistoryPresence
}

type AttendanceHistoryInstance struct {
	ID        int64
	Date      string
	Title     string
	Status    string
	StartTime time.Time
	EndTime   time.Time
}

type AttendanceHistoryPresence struct {
	Status       string
	Substatus    *string
	Note         *string
	CheckedInAt  *time.Time
	CheckedOutAt *time.Time
	IsUnplanned  bool
}

type AttendanceHistory struct{ records AttendanceHistoryRecords }

func NewAttendanceHistory(records AttendanceHistoryRecords) *AttendanceHistory {
	return &AttendanceHistory{records: records}
}

func (r *AttendanceHistory) Slots(ctx context.Context, studentID int64, from, to string) ([]*AttendanceHistorySlot, error) {
	rows, err := r.records.FindInstancesWithAttendanceByStudentAndDateRange(ctx, studentID, schedule.Date(from), schedule.Date(to))
	if rows == nil {
		return nil, err
	}
	result := make([]*AttendanceHistorySlot, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		slot := &AttendanceHistorySlot{}
		if instance := row.Instance; instance != nil {
			slot.Instance = &AttendanceHistoryInstance{
				ID: instance.ID, Date: instance.Date.String(), Title: instance.Title, Status: instance.Status,
				StartTime: instance.StartTime, EndTime: instance.EndTime,
			}
		}
		if attendance := row.Attendance; attendance != nil {
			slot.Attendance = &AttendanceHistoryPresence{
				Status: attendance.Status, Substatus: attendance.Substatus, Note: attendance.Note,
				CheckedInAt: attendance.CheckedInAt, CheckedOutAt: attendance.CheckedOutAt, IsUnplanned: attendance.IsUnplanned,
			}
		}
		result[i] = slot
	}
	return result, err
}

func (r *AttendanceHistory) HasPlannedSlots(ctx context.Context, from, to string) (bool, error) {
	return r.records.HasPlannedSlotsInRange(ctx, schedule.Date(from), schedule.Date(to))
}
