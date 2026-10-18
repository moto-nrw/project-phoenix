package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

type monthSnapshotCapability struct{ workforce.MonthSnapshots }

// MonthSnapshotCapability maps Workforce records onto the month consumers' port.
func MonthSnapshotCapability(snapshots workforce.MonthSnapshots) MonthSnapshots {
	if snapshots == nil {
		panic("month snapshots: Workforce is required")
	}
	return monthSnapshotCapability{snapshots}
}

func (c monthSnapshotCapability) LatestClosedMonth(ctx context.Context, staffID int64, year, month int) (*MonthSnapshot, error) {
	value, err := c.MonthSnapshots.LatestClosedMonth(ctx, staffID, year, month)
	if value == nil {
		return nil, err
	}
	result := MonthSnapshot(*value)
	return &result, err
}

func (c monthSnapshotCapability) ClosedMonthSnapshots(ctx context.Context, year, month int) ([]*MonthSnapshot, error) {
	values, err := c.MonthSnapshots.ClosedMonthSnapshots(ctx, year, month)
	return consumerMonthSnapshots(values), err
}

func (c monthSnapshotCapability) ClosedMonthSnapshotsForStaff(ctx context.Context, staffIDs []int64) ([]*MonthSnapshot, error) {
	values, err := c.MonthSnapshots.ClosedMonthSnapshotsForStaff(ctx, staffIDs)
	return consumerMonthSnapshots(values), err
}

func (c monthSnapshotCapability) RecordClosedMonth(ctx context.Context, value MonthSnapshot) (MonthSnapshot, error) {
	result, err := c.MonthSnapshots.RecordClosedMonth(ctx, workforce.StaffMonthBalanceSnapshot(value))
	return MonthSnapshot(result), err
}

func consumerMonthSnapshots(values []*workforce.StaffMonthBalanceSnapshot) []*MonthSnapshot {
	if values == nil {
		return nil
	}
	result := make([]*MonthSnapshot, 0, len(values))
	for _, value := range values {
		converted := MonthSnapshot(*value)
		result = append(result, &converted)
	}
	return result
}

func publicMonthSnapshots(values []*MonthSnapshot) []*workforce.StaffMonthBalanceSnapshot {
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
