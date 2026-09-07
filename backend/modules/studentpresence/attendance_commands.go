package studentpresence

import (
	"context"
	"time"
)

type AttendanceHistoryCommand interface {
	RecordAttendance(context.Context, Attendance) (Attendance, error)
	ReviseAttendance(context.Context, Attendance) (Attendance, error)
	DeleteAttendance(context.Context, int64) error
	EnsureAttendanceBatch(context.Context, []Attendance) ([]int64, error)
	CloseStaleAttendance(context.Context, int64, time.Time, time.Time) (int64, error)
	LockStudentAttendance(context.Context, int64) error
}
type AttendanceSummaryQuery interface {
	HasAttendance(context.Context, AttendanceFilter) (bool, error)
	CountAttendanceByStaff(context.Context, int64) (int, error)
	ListOpenAttendanceStudentIDs(context.Context, string) ([]int64, error)
}

func (m *Module) RecordAttendance(ctx context.Context, value Attendance) (Attendance, error) {
	return m.engine.RecordAttendance(ctx, value)
}
func (m *Module) ReviseAttendance(ctx context.Context, value Attendance) (Attendance, error) {
	return m.engine.ReviseAttendance(ctx, value)
}
func (m *Module) DeleteAttendance(ctx context.Context, id int64) error {
	return m.engine.DeleteAttendance(ctx, id)
}
func (m *Module) EnsureAttendanceBatch(ctx context.Context, rows []Attendance) ([]int64, error) {
	return m.engine.EnsureAttendanceBatch(ctx, rows)
}
func (m *Module) CloseStaleAttendance(ctx context.Context, id int64, at, updatedAt time.Time) (int64, error) {
	return m.engine.CloseStaleAttendance(ctx, id, at, updatedAt)
}
func (m *Module) LockStudentAttendance(ctx context.Context, id int64) error {
	return m.engine.LockStudentAttendance(ctx, id)
}
func (m *Module) HasAttendance(ctx context.Context, filter AttendanceFilter) (bool, error) {
	return m.engine.HasAttendance(ctx, filter)
}
func (m *Module) CountAttendanceByStaff(ctx context.Context, id int64) (int, error) {
	return m.engine.CountAttendanceByStaff(ctx, id)
}
func (m *Module) ListOpenAttendanceStudentIDs(ctx context.Context, date string) ([]int64, error) {
	return m.engine.ListOpenAttendanceStudentIDs(ctx, date)
}
