package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// timeTrackingShiftRow is the planned-window projection consumed by time tracking.
// StartTime and EndTime are wall clocks anchored at 0001-01-01 UTC, the
// normalized shape the retained repositories handed their callers.
type timeTrackingShiftRow struct {
	StaffID      int64
	Date         string
	StartTime    time.Time
	EndTime      time.Time
	BreakMinutes int
	Cancelled    bool
}

// timeTrackingShiftSource reads planned shifts for the time-tracking services
// through the public Workforce capability (#3418); it performs no persistence
// of its own.
type timeTrackingShiftSource struct{ shifts workforce.ShiftQuery }

func newTimeTrackingShiftSource(shifts workforce.ShiftQuery) *timeTrackingShiftSource {
	if shifts == nil {
		panic("time tracking shifts: Workforce shift query is required")
	}
	return &timeTrackingShiftSource{shifts: shifts}
}

func (r *timeTrackingShiftSource) ForStaffOnDate(ctx context.Context, staffIDs []int64, date string) ([]*timeTrackingShiftRow, error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}
	return r.list(ctx, workforce.StaffShiftFilter{
		StaffIDs: staffIDs, Dates: []string{date},
		Order: timeTrackingShiftOrder(workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *timeTrackingShiftSource) ForStaffInRange(ctx context.Context, staffID int64, from, to string) ([]*timeTrackingShiftRow, error) {
	return r.list(ctx, workforce.StaffShiftFilter{
		StaffID: staffID, From: from, To: to,
		Order: timeTrackingShiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStartTime),
	})
}

func (r *timeTrackingShiftSource) InRange(ctx context.Context, from, to string) ([]*timeTrackingShiftRow, error) {
	return r.list(ctx, workforce.StaffShiftFilter{
		From: from, To: to,
		Order: timeTrackingShiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *timeTrackingShiftSource) ByStaffInRange(ctx context.Context, staffIDs []int64, from, to string) (map[int64][]*timeTrackingShiftRow, error) {
	result := make(map[int64][]*timeTrackingShiftRow, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	rows, err := r.list(ctx, workforce.StaffShiftFilter{
		StaffIDs: staffIDs, From: from, To: to,
		Order: timeTrackingShiftOrder(workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStartTime),
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.StaffID] = append(result[row.StaffID], row)
	}
	return result, nil
}

func (r *timeTrackingShiftSource) list(ctx context.Context, filter workforce.StaffShiftFilter) ([]*timeTrackingShiftRow, error) {
	rows, err := r.shifts.ListStaffShifts(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*timeTrackingShiftRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, &timeTrackingShiftRow{
			StaffID: row.StaffID, Date: row.Date,
			StartTime: timeTrackingWallClock(row.StartTime), EndTime: timeTrackingWallClock(row.EndTime),
			BreakMinutes: row.BreakMinutes, Cancelled: row.Cancelled,
		})
	}
	return result, nil
}

func timeTrackingShiftOrder(fields ...workforce.StaffShiftOrderField) []workforce.StaffShiftOrder {
	order := make([]workforce.StaffShiftOrder, 0, len(fields))
	for _, field := range fields {
		order = append(order, workforce.StaffShiftOrder{Field: field})
	}
	return order
}

// timeTrackingWallClock restores the normalized wall clock the retained
// repositories returned: the clock anchored at 0001-01-01 UTC.
func timeTrackingWallClock(value string) time.Time {
	parsed, err := time.Parse(workforce.ClockLayout, value)
	if err != nil {
		return time.Time{}
	}
	return time.Date(1, time.January, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC)
}
