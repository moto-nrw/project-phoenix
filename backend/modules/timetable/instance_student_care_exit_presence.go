package timetable

import (
	"context"
	"time"
)

func (m *Module) ListOpenStudentAssignments(ctx context.Context, studentIDs []int64) ([]int64, error) {
	if hasInvalidID(studentIDs) {
		return nil, m.reject("list_open_student_assignments", ErrInvalidInstanceStudent)
	}
	return m.engine.ListOpenStudentAssignments(ctx, studentIDs)
}

func (m *Module) LatestStudentAssignmentAttendanceDate(ctx context.Context, studentID int64) (*string, error) {
	if studentID <= 0 {
		return nil, m.reject("latest_student_assignment_attendance_date", ErrInvalidInstanceStudent)
	}
	return m.engine.LatestStudentAssignmentAttendanceDate(ctx, studentID)
}

func (m *Module) CloseOpenStudentAssignments(ctx context.Context, studentIDs []int64, at time.Time) (int64, error) {
	if hasInvalidID(studentIDs) || at.IsZero() {
		return 0, m.reject("close_open_student_assignments", ErrInvalidInstanceStudent)
	}
	return m.engine.CloseOpenStudentAssignments(ctx, studentIDs, at)
}
