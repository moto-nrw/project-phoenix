package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type monthSnapshotCapability struct{ workforce.MonthSnapshots }

// MonthSnapshotCapability maps Workforce records onto the month consumers' port.
func MonthSnapshotCapability(snapshots workforce.MonthSnapshots) active.MonthSnapshots {
	if snapshots == nil {
		panic("month snapshots: Workforce is required")
	}
	return monthSnapshotCapability{snapshots}
}

func (c monthSnapshotCapability) LatestClosedMonth(ctx context.Context, staffID int64, year, month int) (*active.MonthSnapshot, error) {
	value, err := c.MonthSnapshots.LatestClosedMonth(ctx, staffID, year, month)
	if value == nil {
		return nil, err
	}
	result := active.MonthSnapshot(*value)
	return &result, err
}

func (c monthSnapshotCapability) ClosedMonthSnapshots(ctx context.Context, year, month int) ([]*active.MonthSnapshot, error) {
	values, err := c.MonthSnapshots.ClosedMonthSnapshots(ctx, year, month)
	return consumerMonthSnapshots(values), err
}

func (c monthSnapshotCapability) ClosedMonthSnapshotsForStaff(ctx context.Context, staffIDs []int64) ([]*active.MonthSnapshot, error) {
	values, err := c.MonthSnapshots.ClosedMonthSnapshotsForStaff(ctx, staffIDs)
	return consumerMonthSnapshots(values), err
}

func (c monthSnapshotCapability) RecordClosedMonth(ctx context.Context, value active.MonthSnapshot) (active.MonthSnapshot, error) {
	result, err := c.MonthSnapshots.RecordClosedMonth(ctx, workforce.StaffMonthBalanceSnapshot(value))
	return active.MonthSnapshot(result), err
}

func consumerMonthSnapshots(values []*workforce.StaffMonthBalanceSnapshot) []*active.MonthSnapshot {
	if values == nil {
		return nil
	}
	result := make([]*active.MonthSnapshot, 0, len(values))
	for _, value := range values {
		converted := active.MonthSnapshot(*value)
		result = append(result, &converted)
	}
	return result
}

func publicMonthSnapshots(values []*active.MonthSnapshot) []*workforce.StaffMonthBalanceSnapshot {
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
