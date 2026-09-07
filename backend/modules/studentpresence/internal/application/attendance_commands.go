package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) RecordAttendance(ctx context.Context, row *ports.Attendance) error {
	return s.runWrite(ctx, "record_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if row == nil || row.StudentID <= 0 || row.Date == "" || row.CheckInTime.IsZero() || row.DeviceID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance")
		}
		return s.store.RecordAttendance(txCtx, row)
	})
}

func (s *Service) ReviseAttendance(ctx context.Context, row *ports.Attendance) error {
	return s.runWrite(ctx, "revise_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if row == nil || row.StudentID <= 0 || row.Date == "" || row.CheckInTime.IsZero() || row.DeviceID <= 0 || row.ID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance")
		}
		return s.store.ReviseAttendance(txCtx, row)
	})
}

func (s *Service) DeleteAttendance(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance ID")
		}
		return s.store.DeleteAttendance(txCtx, id)
	})
}
func (s *Service) EnsureAttendanceBatch(ctx context.Context, rows []*ports.Attendance) (result []int64, err error) {
	err = s.runWrite(ctx, "ensure_attendance_batch", func(txCtx context.Context) (ports.Stats, error) {
		for _, row := range rows {
			if row == nil || row.StudentID <= 0 || row.Date == "" || row.CheckInTime.IsZero() || row.DeviceID <= 0 {
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

func (s *Service) HasAttendance(ctx context.Context, filter ports.AttendanceFilter) (result bool, err error) {
	err = s.run("has_attendance", func() (ports.Stats, error) {
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
	err = s.run("count_attendance_by_staff", func() (ports.Stats, error) {
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
	err = s.run("list_open_attendance_student_ids", func() (ports.Stats, error) {
		if date == "" {
			return ports.Stats{}, errors.New("student presence: invalid attendance query")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListOpenAttendanceStudentIDs(ctx, date)
		return stats, err
	})
	return result, err
}
