package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *Service) RecordAttendance(ctx context.Context, value studentpresence.Attendance) (studentpresence.Attendance, error) {
	row, err := attendanceFromPublic(value)
	if err != nil {
		return studentpresence.Attendance{}, err
	}
	err = s.runWrite(ctx, "record_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if row.StudentID <= 0 || row.Date == "" || row.CheckInTime.IsZero() || row.DeviceID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance")
		}
		return s.store.RecordAttendance(txCtx, row)
	})
	return attendanceToPublic(row), err
}

func (s *Service) ReviseAttendance(ctx context.Context, value studentpresence.Attendance) (studentpresence.Attendance, error) {
	row, err := attendanceFromPublic(value)
	if err != nil {
		return studentpresence.Attendance{}, err
	}
	err = s.runWrite(ctx, "revise_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if row.StudentID <= 0 || row.Date == "" || row.CheckInTime.IsZero() || row.DeviceID <= 0 || row.ID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance")
		}
		return s.store.ReviseAttendance(txCtx, row)
	})
	return attendanceToPublic(row), err
}

func (s *Service) DeleteAttendance(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance ID")
		}
		return s.store.DeleteAttendance(txCtx, id)
	})
}

// DeleteExpiredAttendance deletes the attendance dated before the first
// Berlin calendar day a window of retentionDays keeps, for the nightly
// retention (#3957). It joins the school's cleanup transaction and never
// opens its own, so the deletion commits with its deletion-log entry.
func (s *Service) DeleteExpiredAttendance(ctx context.Context, retentionDays int) (deleted int, err error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", studentpresence.ErrAttendanceRetentionWindow, retentionDays)
	}
	cutoff := calendar.TodayDate().AddDays(-retentionDays)
	err = s.run("delete_attendance_before", func() (ports.Stats, error) {
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		stats, err := s.store.DeleteAttendanceBefore(ctx, cutoff)
		deleted = int(stats.Rows)
		return stats, err
	})
	return deleted, err
}

func (s *Service) EnsureAttendanceBatch(ctx context.Context, values []studentpresence.Attendance) (result []int64, err error) {
	rows := make([]*ports.Attendance, 0, len(values))
	for _, value := range values {
		row, err := attendanceFromPublic(value)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	err = s.runWrite(ctx, "ensure_attendance_batch", func(txCtx context.Context) (ports.Stats, error) {
		for _, row := range rows {
			if row.StudentID <= 0 || row.Date == "" || row.CheckInTime.IsZero() || row.DeviceID <= 0 {
				return ports.Stats{}, errors.New("student presence: invalid attendance")
			}
		}
		var stats ports.Stats
		result, stats, err = s.store.EnsureAttendanceBatch(txCtx, rows)
		return stats, err
	})
	return result, err
}
func (s *Service) CloseStaleAttendance(ctx context.Context, id int64, at, updatedAt time.Time) (result int64, err error) {
	err = s.runWrite(ctx, "close_stale_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 || at.IsZero() || updatedAt.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid stale attendance close")
		}
		stats, writeErr := s.store.CloseStaleAttendance(txCtx, id, at, updatedAt)
		if writeErr == nil {
			result = stats.Rows
		}
		return stats, writeErr
	})
	return result, err
}
func (s *Service) LockStudentAttendance(ctx context.Context, id int64) error {
	return s.run("lock_student_attendance", func() (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockStudentAttendance(ctx, id)
	})
}

func (s *Service) HasAttendance(ctx context.Context, filter studentpresence.AttendanceFilter) (result bool, err error) {
	if err := validAttendanceFilterDates(filter); err != nil {
		return false, err
	}
	err = s.runWrite(ctx, "has_attendance", func(ctx context.Context) (ports.Stats, error) {
		if !validIDs(filter.IDs) || !validIDs(filter.StudentIDs) || filter.ForUpdate {
			return ports.Stats{}, errors.New("student presence: invalid attendance query")
		}
		var stats ports.Stats
		result, stats, err = s.store.HasAttendance(ctx, filter)
		return stats, err
	})
	return result, err
}

func (s *Service) CountAttendanceByStaff(ctx context.Context, id int64) (result int, err error) {
	err = s.runWrite(ctx, "count_attendance_by_staff", func(ctx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance query")
		}
		var stats ports.Stats
		result, stats, err = s.store.CountAttendanceByStaff(ctx, id)
		return stats, err
	})
	return result, err
}

func (s *Service) ListOpenAttendanceStudentIDs(ctx context.Context, date string) (result []int64, err error) {
	if _, err := calendar.ParseDate(date); err != nil {
		return nil, err
	}
	err = s.runWrite(ctx, "list_open_attendance_student_ids", func(ctx context.Context) (ports.Stats, error) {
		if date == "" {
			return ports.Stats{}, errors.New("student presence: invalid attendance query")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListOpenAttendanceStudentIDs(ctx, date)
		return stats, err
	})
	return result, err
}
