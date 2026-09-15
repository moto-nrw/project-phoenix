package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) FindAttendance(ctx context.Context, id int64) (result *ports.Attendance, err error) {
	err = s.run("find_attendance", func() (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.FindAttendance(ctx, id)
		return stats, err
	})
	return result, err
}

func (s *Service) ListAttendance(ctx context.Context, filter ports.AttendanceFilter) (result []*ports.Attendance, err error) {
	err = s.run("list_attendance", func() (ports.Stats, error) {
		if !validIDs(filter.IDs) || !validIDs(filter.StudentIDs) || filter.Limit < 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance filter")
		}
		if filter.ForUpdate {
			if err := s.tx.Require(ctx); err != nil {
				return ports.Stats{}, err
			}
		}
		var stats ports.Stats
		result, stats, err = s.store.ListAttendance(ctx, filter)
		return stats, err
	})
	return result, err
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

func (s *Service) EnsureAttendance(ctx context.Context, row *ports.Attendance) (inserted bool, err error) {
	err = s.runWrite(ctx, "ensure_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if row == nil || row.StudentID <= 0 || row.Date == "" || row.CheckInTime.IsZero() || row.DeviceID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid attendance")
		}
		var stats ports.Stats
		inserted, stats, err = s.store.EnsureAttendance(txCtx, row)
		return stats, err
	})
	return inserted, err
}

func (s *Service) CloseAttendance(ctx context.Context, checkout ports.AttendanceCheckout) (result []*ports.Attendance, err error) {
	err = s.runWrite(ctx, "close_attendance", func(txCtx context.Context) (ports.Stats, error) {
		if !validIDs(checkout.StudentIDs) || checkout.Date == "" || checkout.At.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid attendance checkout")
		}
		var stats ports.Stats
		result, stats, err = s.store.CloseAttendance(txCtx, checkout)
		return stats, err
	})
	return result, err
}

func (s *Service) ListAttendanceDays(ctx context.Context, from, to ports.Date) (result []ports.AttendanceDay, err error) {
	err = s.run("list_attendance_days", func() (ports.Stats, error) {
		if from.IsZero() || to.IsZero() || to.Before(from) {
			return ports.Stats{}, errors.New("student presence: invalid attendance day range")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListAttendanceDays(ctx, from, to)
		return stats, err
	})
	return result, err
}

func (s *Service) RoomUtilization(ctx context.Context, windows []ports.StudentVisitWindow) (result []ports.RoomUtilization, err error) {
	err = s.run("room_utilization", func() (ports.Stats, error) {
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
	return result, err
}
