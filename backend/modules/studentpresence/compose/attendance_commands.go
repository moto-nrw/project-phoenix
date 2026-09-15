package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (e engine) RecordAttendance(ctx context.Context, value studentpresence.Attendance) (studentpresence.Attendance, error) {
	row, err := attendanceFromPublic(value)
	if err != nil {
		return studentpresence.Attendance{}, err
	}
	err = e.Service.RecordAttendance(ctx, row)
	return attendanceToPublic(row), err
}

func (e engine) ReviseAttendance(ctx context.Context, value studentpresence.Attendance) (studentpresence.Attendance, error) {
	row, err := attendanceFromPublic(value)
	if err != nil {
		return studentpresence.Attendance{}, err
	}
	err = e.Service.ReviseAttendance(ctx, row)
	return attendanceToPublic(row), err
}

func (e engine) EnsureAttendanceBatch(ctx context.Context, values []studentpresence.Attendance) ([]int64, error) {
	rows := make([]*ports.Attendance, 0, len(values))
	for _, value := range values {
		row, err := attendanceFromPublic(value)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return e.Service.EnsureAttendanceBatch(ctx, rows)
}
func (e engine) HasAttendance(ctx context.Context, filter studentpresence.AttendanceFilter) (bool, error) {
	for _, date := range []string{filter.FromDate, filter.UntilDate, filter.BeforeDate} {
		if date != "" {
			if _, err := timezone.ParseDate(date); err != nil {
				return false, err
			}
		}
	}
	return e.Service.HasAttendance(ctx, ports.AttendanceFilter(filter))
}
func (e engine) ListOpenAttendanceStudentIDs(ctx context.Context, date string) ([]int64, error) {
	if _, err := timezone.ParseDate(date); err != nil {
		return nil, err
	}
	return e.Service.ListOpenAttendanceStudentIDs(ctx, date)
}
func (e engine) CloseStaleAttendance(ctx context.Context, id int64, at, updatedAt time.Time) (int64, error) {
	return e.Service.CloseStaleAttendance(ctx, id, at, updatedAt)
}
