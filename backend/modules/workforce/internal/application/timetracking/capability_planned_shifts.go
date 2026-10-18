package timetracking

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimeTrackingShifts serves the time-tracking planned-shift port from the
// public Workforce shift query (#3418); it performs no persistence of its own.
type TimeTrackingShifts struct {
	shifts workforce.ShiftQuery
}

// NewTimeTrackingShifts binds the planned-shift port to the shift query.
func NewTimeTrackingShifts(shifts workforce.ShiftQuery) *TimeTrackingShifts {
	if shifts == nil {
		panic("time tracking shifts: Workforce shift query is required")
	}
	return &TimeTrackingShifts{shifts: shifts}
}

func (r *TimeTrackingShifts) FindByStaffIDsAndDate(ctx context.Context, ids []int64, date calendar.Date) ([]*TimeTrackingShift, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return r.list(ctx, workforce.StaffShiftFilter{
		StaffIDs: ids, Dates: []string{date.String()},
		Order: timeTrackingShiftOrder(workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *TimeTrackingShifts) FindByStaffAndDateRange(ctx context.Context, id int64, from, to calendar.Date) ([]*TimeTrackingShift, error) {
	return r.list(ctx, workforce.StaffShiftFilter{
		StaffID: id, From: from.String(), To: to.String(),
		Order: timeTrackingShiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStartTime),
	})
}

func (r *TimeTrackingShifts) FindByDateRange(ctx context.Context, from, to calendar.Date) ([]*TimeTrackingShift, error) {
	return r.list(ctx, workforce.StaffShiftFilter{
		From: from.String(), To: to.String(),
		Order: timeTrackingShiftOrder(workforce.StaffShiftOrderDate, workforce.StaffShiftOrderStaffID, workforce.StaffShiftOrderStartTime),
	})
}

func (r *TimeTrackingShifts) FindByStaffIDsAndDateRange(ctx context.Context, ids []int64, from, to calendar.Date) (map[int64][]*TimeTrackingShift, error) {
	result := make(map[int64][]*TimeTrackingShift, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := r.list(ctx, workforce.StaffShiftFilter{
		StaffIDs: ids, From: from.String(), To: to.String(),
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

func (r *TimeTrackingShifts) list(ctx context.Context, filter workforce.StaffShiftFilter) ([]*TimeTrackingShift, error) {
	rows, err := r.shifts.ListStaffShifts(ctx, filter)
	if err != nil {
		return nil, err
	}
	result := make([]*TimeTrackingShift, 0, len(rows))
	for _, row := range rows {
		result = append(result, &TimeTrackingShift{
			StaffID: row.StaffID, Date: calendar.Date(row.Date),
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
