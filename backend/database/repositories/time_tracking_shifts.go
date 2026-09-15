package repositories

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
)

type TimeTrackingShiftRecords interface {
	FindByStaffIDsAndDate(context.Context, []int64, schedule.Date) ([]*schedule.StaffShift, error)
	FindByStaffIDsAndDateRange(context.Context, []int64, schedule.Date, schedule.Date) (map[int64][]*schedule.StaffShift, error)
	FindByStaffAndDateRange(context.Context, int64, schedule.Date, schedule.Date) ([]*schedule.StaffShift, error)
	FindByDateRange(context.Context, schedule.Date, schedule.Date) ([]*schedule.StaffShift, error)
}

// TimeTrackingShift is the planned-window projection consumed by time tracking.
type TimeTrackingShift struct {
	StaffID      int64
	Date         string
	StartTime    time.Time
	EndTime      time.Time
	BreakMinutes int
	Cancelled    bool
}

type TimeTrackingShifts struct{ records TimeTrackingShiftRecords }

func NewTimeTrackingShifts(records TimeTrackingShiftRecords) *TimeTrackingShifts {
	return &TimeTrackingShifts{records: records}
}

func (r *TimeTrackingShifts) ForStaffOnDate(ctx context.Context, staffIDs []int64, date string) ([]*TimeTrackingShift, error) {
	rows, err := r.records.FindByStaffIDsAndDate(ctx, staffIDs, schedule.Date(date))
	return timeTrackingShifts(rows), err
}

func (r *TimeTrackingShifts) ForStaffInRange(ctx context.Context, staffID int64, from, to string) ([]*TimeTrackingShift, error) {
	rows, err := r.records.FindByStaffAndDateRange(ctx, staffID, schedule.Date(from), schedule.Date(to))
	return timeTrackingShifts(rows), err
}

func (r *TimeTrackingShifts) InRange(ctx context.Context, from, to string) ([]*TimeTrackingShift, error) {
	rows, err := r.records.FindByDateRange(ctx, schedule.Date(from), schedule.Date(to))
	return timeTrackingShifts(rows), err
}

func (r *TimeTrackingShifts) ByStaffInRange(ctx context.Context, staffIDs []int64, from, to string) (map[int64][]*TimeTrackingShift, error) {
	rows, err := r.records.FindByStaffIDsAndDateRange(ctx, staffIDs, schedule.Date(from), schedule.Date(to))
	if rows == nil {
		return nil, err
	}
	result := make(map[int64][]*TimeTrackingShift, len(rows))
	for staffID, shifts := range rows {
		result[staffID] = timeTrackingShifts(shifts)
	}
	return result, err
}

func timeTrackingShifts(rows []*schedule.StaffShift) []*TimeTrackingShift {
	if rows == nil {
		return nil
	}
	result := make([]*TimeTrackingShift, len(rows))
	for i, row := range rows {
		if row != nil {
			result[i] = &TimeTrackingShift{StaffID: row.StaffID, Date: row.Date.String(), StartTime: row.StartTime, EndTime: row.EndTime, BreakMinutes: row.BreakMinutes, Cancelled: row.Cancelled}
		}
	}
	return result
}
