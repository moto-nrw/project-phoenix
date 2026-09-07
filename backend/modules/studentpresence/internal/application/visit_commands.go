package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
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

func (s *Service) DeleteCompletedVisitsBefore(ctx context.Context, studentID int64, cutoff time.Time) (result int64, err error) {
	err = s.runWrite(ctx, "delete_completed_visits_before", func(txCtx context.Context) (ports.Stats, error) {
		if studentID <= 0 || cutoff.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid visit retention request")
		}
		stats, writeErr := s.store.DeleteCompletedVisitsBefore(txCtx, studentID, cutoff)
		result = stats.Rows
		return stats, writeErr
	})
	return result, err
}
