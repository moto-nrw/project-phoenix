package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (e engine) LatestClosedMonth(ctx context.Context, staffID int64, year, month int) (*workforce.StaffMonthBalanceSnapshot, error) {
	value, err := e.service.LatestClosedMonth(ctx, staffID, year, month)
	if value == nil {
		return nil, err
	}
	result := workforce.StaffMonthBalanceSnapshot(*value)
	return &result, err
}

func (e engine) ClosedMonthSnapshots(ctx context.Context, year, month int) ([]*workforce.StaffMonthBalanceSnapshot, error) {
	values, err := e.service.ClosedMonthSnapshots(ctx, year, month)
	return monthSnapshotsToPublic(values), err
}

func (e engine) ClosedMonthSnapshotsForStaff(ctx context.Context, staffIDs []int64) ([]*workforce.StaffMonthBalanceSnapshot, error) {
	values, err := e.service.ClosedMonthSnapshotsForStaff(ctx, staffIDs)
	return monthSnapshotsToPublic(values), err
}

func monthSnapshotsToPublic(values []*domain.StaffMonthBalanceSnapshot) []*workforce.StaffMonthBalanceSnapshot {
	if values == nil {
		return nil
	}
	result := make([]*workforce.StaffMonthBalanceSnapshot, 0, len(values))
	for _, value := range values {
		converted := workforce.StaffMonthBalanceSnapshot(*value)
		result = append(result, &converted)
	}
	return result
}

func (e engine) RecordClosedMonth(ctx context.Context, value workforce.StaffMonthBalanceSnapshot) (workforce.StaffMonthBalanceSnapshot, error) {
	result, err := e.service.RecordClosedMonth(ctx, domain.StaffMonthBalanceSnapshot(value))
	return workforce.StaffMonthBalanceSnapshot(result), err
}

func (e engine) ReopenMonthSnapshot(ctx context.Context, id, actorID int64, at time.Time, reason string) (int64, error) {
	return e.service.ReopenMonthSnapshot(ctx, id, actorID, at, reason)
}
