package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The attendance record keeps its calendar date typed; the public value
// carries it as YYYY-MM-DD.
func attendanceToPublic(row *ports.Attendance) studentpresence.Attendance {
	if row == nil {
		return studentpresence.Attendance{}
	}
	return studentpresence.Attendance{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, Date: row.Date.String(), CheckInTime: row.CheckInTime,
		CheckOutTime: row.CheckOutTime, CheckedInBy: row.CheckedInBy, CheckedOutBy: row.CheckedOutBy,
		DeviceID: row.DeviceID, CheckedOutDeviceID: row.CheckedOutDeviceID, YardSince: row.YardSince,
		CheckOutNote: row.CheckOutNote,
	}
}

func attendanceFromPublic(value studentpresence.Attendance) (*ports.Attendance, error) {
	date, err := calendar.ParseDate(value.Date)
	if err != nil {
		return nil, err
	}
	row := &ports.Attendance{StudentID: value.StudentID, Date: date, CheckInTime: value.CheckInTime,
		CheckOutTime: value.CheckOutTime, CheckedInBy: value.CheckedInBy, CheckedOutBy: value.CheckedOutBy,
		DeviceID: value.DeviceID, CheckedOutDeviceID: value.CheckedOutDeviceID, YardSince: value.YardSince,
		CheckOutNote: value.CheckOutNote}
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

func validAttendanceFilterDates(filter studentpresence.AttendanceFilter) error {
	for _, date := range []string{filter.FromDate, filter.UntilDate, filter.BeforeDate} {
		if date != "" {
			if _, err := calendar.ParseDate(date); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) FindAttendance(ctx context.Context, id int64) (*studentpresence.Attendance, error) {
	var row *ports.Attendance
	err := s.runWrite(ctx, "find_attendance", func(ctx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance ID")
		}
		var stats ports.Stats
		var err error
		row, stats, err = s.store.FindAttendance(ctx, id)
		return stats, err
	})
	if errors.Is(err, transactionruntime.ErrNoRows) {
		return nil, studentpresence.ErrAttendanceNotFound
	}
	if err != nil {
		return nil, err
	}
	result := attendanceToPublic(row)
	return &result, nil
}

func (s *Service) ListAttendance(ctx context.Context, filter studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	if err := validAttendanceFilterDates(filter); err != nil {
		return nil, err
	}
	var rows []*ports.Attendance
	err := s.runQuery(ctx, filter.ForUpdate, "list_attendance", func(ctx context.Context) (ports.Stats, error) {
		if !validIDs(filter.IDs) || !validIDs(filter.StudentIDs) || filter.Limit < 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance filter")
		}
		if filter.ForUpdate {
			if err := s.tx.Require(ctx); err != nil {
				return ports.Stats{}, err
			}
		}
		var stats ports.Stats
		var err error
		rows, stats, err = s.store.ListAttendance(ctx, filter)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return attendanceRowsToPublic(rows), nil
}

func (s *Service) runWrite(ctx context.Context, operation string, fn func(context.Context) (ports.Stats, error)) error {
	return s.run(operation, func() (ports.Stats, error) {
		var stats ports.Stats
		err := s.tx.Run(ctx, func(txCtx context.Context) error {
			var err error
			stats, err = fn(txCtx)
			return err
		})
		return stats, err
	})
}

func (s *Service) EnsureAttendance(ctx context.Context, input studentpresence.Attendance) (studentpresence.Attendance, bool, error) {
	row, err := attendanceFromPublic(input)
	if err != nil {
		return studentpresence.Attendance{}, false, err
	}
	var inserted bool
	err = s.runWrite(ctx, "ensure_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if row.StudentID <= 0 || row.Date == "" || row.CheckInTime.IsZero() || row.DeviceID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance")
		}
		var stats ports.Stats
		var err error
		inserted, stats, err = s.store.EnsureAttendance(txCtx, row)
		return stats, err
	})
	return attendanceToPublic(row), inserted, err
}

func (s *Service) CloseAttendance(ctx context.Context, checkout studentpresence.AttendanceCheckout) ([]studentpresence.Attendance, error) {
	if _, err := calendar.ParseDate(checkout.Date); err != nil {
		return nil, err
	}
	var rows []*ports.Attendance
	err := s.runWrite(ctx, "close_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if !validIDs(checkout.StudentIDs) || checkout.Date == "" || checkout.At.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid attendance checkout")
		}
		var stats ports.Stats
		var err error
		rows, stats, err = s.store.CloseAttendance(txCtx, checkout)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return attendanceRowsToPublic(rows), nil
}

func (s *Service) ListAttendanceDays(ctx context.Context, from, to string) ([]studentpresence.AttendanceDay, error) {
	start, err := calendar.ParseDate(from)
	if err != nil {
		return nil, err
	}
	end, err := calendar.ParseDate(to)
	if err != nil {
		return nil, err
	}
	var rows []ports.AttendanceDay
	err = s.runWrite(ctx, "list_attendance_days", func(ctx context.Context) (ports.Stats, error) {
		if start.IsZero() || end.IsZero() || end.Before(start) {
			return ports.Stats{}, errors.New("student presence: invalid attendance day range")
		}
		var stats ports.Stats
		var err error
		rows, stats, err = s.store.ListAttendanceDays(ctx, start, end)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.AttendanceDay, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.AttendanceDay{StudentID: row.StudentID, Date: row.Date.String()})
	}
	return result, nil
}

// ListAttendanceSpans reads the Ist of the Soll-und-Ist comparison (#4774).
func (s *Service) ListAttendanceSpans(ctx context.Context, from, to calendar.Date, studentIDs []int64) ([]ports.AttendanceSpan, error) {
	var spans []ports.AttendanceSpan
	err := s.runWrite(ctx, "list_attendance_spans", func(ctx context.Context) (ports.Stats, error) {
		if from.IsZero() || to.IsZero() || to.Before(from) {
			return ports.Stats{}, errors.New("student presence: invalid attendance span range")
		}
		var stats ports.Stats
		var err error
		spans, stats, err = s.store.ListAttendanceSpans(ctx, from, to, studentIDs)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return spans, nil
}

func (s *Service) RoomUtilization(ctx context.Context, windows []studentpresence.StudentVisitWindow) (result []studentpresence.RoomUtilization, err error) {
	err = s.runWrite(ctx, "room_utilization", func(ctx context.Context) (ports.Stats, error) {
		ends := make(map[int64]time.Time, len(windows))
		for _, window := range windows {
			if window.StudentID <= 0 || window.StartAt.IsZero() || !window.EndAt.After(window.StartAt) {
				return ports.Stats{}, errors.New("student presence: invalid visit window")
			}
			if window.StartAt.Before(ends[window.StudentID]) {
				return ports.Stats{}, errors.New("student presence: overlapping or unordered visit windows")
			}
			ends[window.StudentID] = window.EndAt
		}
		var stats ports.Stats
		result, stats, err = s.store.RoomUtilization(ctx, windows)
		return stats, err
	})
	return listResult(result, err)
}
