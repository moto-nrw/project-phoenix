package compose

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type engine struct{ *application.Service }

func attendanceToPublic(row *ports.Attendance) studentpresence.Attendance {
	if row == nil {
		return studentpresence.Attendance{}
	}
	return studentpresence.Attendance{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, Date: row.Date.String(), CheckInTime: row.CheckInTime,
		CheckOutTime: row.CheckOutTime, CheckedInBy: row.CheckedInBy, CheckedOutBy: row.CheckedOutBy,
		DeviceID: row.DeviceID, CheckedOutDeviceID: row.CheckedOutDeviceID, YardSince: row.YardSince,
	}
}
func attendanceFromPublic(value studentpresence.Attendance) (*ports.Attendance, error) {
	date, err := timezone.ParseDate(value.Date)
	if err != nil {
		return nil, err
	}
	row := &ports.Attendance{StudentID: value.StudentID, Date: date, CheckInTime: value.CheckInTime,
		CheckOutTime: value.CheckOutTime, CheckedInBy: value.CheckedInBy, CheckedOutBy: value.CheckedOutBy,
		DeviceID: value.DeviceID, CheckedOutDeviceID: value.CheckedOutDeviceID, YardSince: value.YardSince}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row, nil
}
func attendanceRowsToPublic(rows []*ports.Attendance) []studentpresence.Attendance {
	result := make([]studentpresence.Attendance, 0, len(rows))
	for _, row := range rows {
		result = append(result, attendanceToPublic(row))
	}
	return result
}
func (e engine) FindAttendance(ctx context.Context, id int64) (*studentpresence.Attendance, error) {
	row, err := e.Service.FindAttendance(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, studentpresence.ErrAttendanceNotFound
	}
	if err != nil {
		return nil, err
	}
	result := attendanceToPublic(row)
	return &result, nil
}
func (e engine) ListAttendance(ctx context.Context, filter studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	for _, date := range []string{filter.FromDate, filter.UntilDate, filter.BeforeDate} {
		if date != "" {
			if _, err := timezone.ParseDate(date); err != nil {
				return nil, err
			}
		}
	}
	rows, err := e.Service.ListAttendance(ctx, ports.AttendanceFilter(filter))
	if err != nil {
		return nil, err
	}
	return attendanceRowsToPublic(rows), nil
}
func (e engine) EnsureAttendance(ctx context.Context, input studentpresence.Attendance) (studentpresence.Attendance, bool, error) {
	row, err := attendanceFromPublic(input)
	if err != nil {
		return studentpresence.Attendance{}, false, err
	}
	inserted, err := e.Service.EnsureAttendance(ctx, row)
	return attendanceToPublic(row), inserted, err
}
func (e engine) CloseAttendance(ctx context.Context, input studentpresence.AttendanceCheckout) ([]studentpresence.Attendance, error) {
	if _, err := timezone.ParseDate(input.Date); err != nil {
		return nil, err
	}
	rows, err := e.Service.CloseAttendance(ctx, ports.AttendanceCheckout(input))
	if err != nil {
		return nil, err
	}
	return attendanceRowsToPublic(rows), nil
}
