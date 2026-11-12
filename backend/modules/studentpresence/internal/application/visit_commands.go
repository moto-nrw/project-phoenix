package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) CloseGroupVisits(ctx context.Context, ids []int64) (result int64, err error) {
	err = s.runWrite(ctx, "close_group_visits", func(txCtx context.Context) (ports.Stats, error) {
		stats, writeErr := s.store.CloseGroupVisits(txCtx, ids)
		result = stats.Rows
		return stats, writeErr
	})
	return result, err
}

func (s *Service) TransferOpenVisits(ctx context.Context, from, to int64) (result int64, err error) {
	err = s.runWrite(ctx, "transfer_open_visits", func(txCtx context.Context) (ports.Stats, error) {
		if from <= 0 || to <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit transfer")
		}
		stats, writeErr := s.store.TransferOpenVisits(txCtx, from, to)
		result = stats.Rows
		return stats, writeErr
	})
	return result, err
}

func (s *Service) TransferRecentDeviceVisits(ctx context.Context, to, deviceID int64) (result int64, err error) {
	err = s.runWrite(ctx, "transfer_recent_device_visits", func(txCtx context.Context) (ports.Stats, error) {
		if to <= 0 || deviceID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit transfer")
		}
		stats, writeErr := s.store.TransferRecentDeviceVisits(txCtx, to, deviceID)
		result = stats.Rows
		return stats, writeErr
	})
	return result, err
}

func (s *Service) DeleteExpiredVisits(ctx context.Context, students []studentpresence.VisitRetentionCount, now time.Time) (result map[int64]int64, err error) {
	expiries, err := visitExpiries(students, now)
	if err != nil || len(expiries) == 0 {
		return map[int64]int64{}, err
	}
	err = s.runWrite(ctx, "delete_expired_visits", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		var writeErr error
		result, stats, writeErr = s.store.DeleteExpiredVisits(txCtx, expiries)
		return stats, writeErr
	})
	return result, err
}

// visitExpiries maps each student's retention onto its deletion cutoff: the
// completed-day boundary when the attendance log is off, otherwise its
// retention window back from now. A student appears at most once.
func visitExpiries(students []studentpresence.VisitRetentionCount, now time.Time) ([]ports.VisitExpiry, error) {
	if now.IsZero() {
		return nil, errors.New("student presence: invalid visit retention request")
	}
	expiries := make([]ports.VisitExpiry, 0, len(students))
	seen := make(map[int64]bool, len(students))
	for _, student := range students {
		if student.StudentID <= 0 || seen[student.StudentID] {
			return nil, errors.New("student presence: invalid visit retention request")
		}
		seen[student.StudentID] = true
		if student.CompletedBefore != nil {
			expiries = append(expiries, ports.VisitExpiry{StudentID: student.StudentID, Cutoff: *student.CompletedBefore, ByExit: true})
			continue
		}
		if student.RetentionDays < 1 {
			return nil, errors.New("student presence: invalid visit retention request")
		}
		expiries = append(expiries, ports.VisitExpiry{StudentID: student.StudentID, Cutoff: now.AddDate(0, 0, -student.RetentionDays)})
	}
	return expiries, nil
}
