package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (s *Service) LatestClosedMonth(ctx context.Context, staffID int64, year, month int) (*workforce.StaffMonthBalanceSnapshot, error) {
	var result *domain.StaffMonthBalanceSnapshot
	var err error
	err = s.run(ctx, "latest_closed_month", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.LatestClosedMonth(ctx, staffID, year, month)
		stats.Add(queryStats)
		return err
	})
	value, storedErr := result, err
	if value == nil {
		return nil, storedErr
	}
	storedResult := workforce.StaffMonthBalanceSnapshot(*value)
	return &storedResult, storedErr
}

func (s *Service) ClosedMonthSnapshotsForStaff(ctx context.Context, staffIDs []int64) ([]*workforce.StaffMonthBalanceSnapshot, error) {
	var result []*domain.StaffMonthBalanceSnapshot
	var err error
	err = s.run(ctx, "closed_month_snapshots_for_staff", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ClosedMonthSnapshotsForStaff(ctx, staffIDs)
		stats.Add(queryStats)
		return err
	})
	return monthSnapshotsToPublic(result), err
}

func (s *Service) ClosedMonthSnapshots(ctx context.Context, year, month int) ([]*workforce.StaffMonthBalanceSnapshot, error) {
	var result []*domain.StaffMonthBalanceSnapshot
	var err error
	err = s.run(ctx, "closed_month_snapshots", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ClosedMonthSnapshots(ctx, year, month)
		stats.Add(queryStats)
		return err
	})
	return monthSnapshotsToPublic(result), err
}

func (s *Service) RecordClosedMonth(ctx context.Context, value workforce.StaffMonthBalanceSnapshot) (workforce.StaffMonthBalanceSnapshot, error) {
	storedValue := domain.StaffMonthBalanceSnapshot(value)
	var result domain.StaffMonthBalanceSnapshot
	var err error
	if err := storedValue.Validate(); err != nil {
		return workforce.StaffMonthBalanceSnapshot(result), err
	}
	err = s.run(ctx, "record_closed_month", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.RecordClosedMonth(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return workforce.StaffMonthBalanceSnapshot(result), err
}

func (s *Service) ReopenMonthSnapshot(ctx context.Context, id, actorID int64, at time.Time, reason string) (int64, error) {
	var result int64
	var err error
	err = s.run(ctx, "reopen_month_snapshot", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.ReopenMonthSnapshot(txCtx, id, actorID, at, reason)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}
