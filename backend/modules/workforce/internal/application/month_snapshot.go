package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (s *Service) LatestClosedMonth(ctx context.Context, staffID int64, year, month int) (result *domain.StaffMonthBalanceSnapshot, err error) {
	err = s.run("latest_closed_month", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.LatestClosedMonth(ctx, staffID, year, month)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ClosedMonthSnapshotsForStaff(ctx context.Context, staffIDs []int64) (result []*domain.StaffMonthBalanceSnapshot, err error) {
	err = s.run("closed_month_snapshots_for_staff", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ClosedMonthSnapshotsForStaff(ctx, staffIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ClosedMonthSnapshots(ctx context.Context, year, month int) (result []*domain.StaffMonthBalanceSnapshot, err error) {
	err = s.run("closed_month_snapshots", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ClosedMonthSnapshots(ctx, year, month)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) RecordClosedMonth(ctx context.Context, value domain.StaffMonthBalanceSnapshot) (result domain.StaffMonthBalanceSnapshot, err error) {
	if err := value.Validate(); err != nil {
		return result, err
	}
	err = s.run("record_closed_month", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.RecordClosedMonth(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) ReopenMonthSnapshot(ctx context.Context, id, actorID int64, at time.Time, reason string) (result int64, err error) {
	err = s.run("reopen_month_snapshot", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.ReopenMonthSnapshot(txCtx, id, actorID, at, reason)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}
