package application

import (
	"context"
	"errors"

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
