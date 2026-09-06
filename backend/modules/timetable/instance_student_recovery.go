package timetable

import (
	"context"
	"time"
)

type CompletionAttendance struct {
	RowID              int64
	Status             string
	Substatus          *string
	Note               *string
	CheckedInAt        *time.Time
	CheckedOutAt       *time.Time
	NotScheduled       bool
	StudentStatusDayID *int64
	PickupExceptionID  *int64
}

func (m *Module) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) error {
	if instanceID <= 0 {
		return m.reject("lock_instance_student_assignments", ErrInvalidInstanceStudentQuery)
	}
	return m.engine.LockInstanceStudentAssignments(ctx, instanceID)
}

func (m *Module) RestoreInstanceStudentAttendance(ctx context.Context, instanceID int64, rows []CompletionAttendance) error {
	if instanceID <= 0 {
		return m.reject("restore_instance_student_attendance", ErrInvalidInstanceStudent)
	}
	for _, row := range rows {
		if row.RowID <= 0 || !validInstanceAttendanceStatus(row.Status) {
			return m.reject("restore_instance_student_attendance", ErrInvalidInstanceStudent)
		}
	}
	return m.engine.RestoreInstanceStudentAttendance(ctx, instanceID, rows)
}
