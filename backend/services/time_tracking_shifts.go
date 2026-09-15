package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type TimeTrackingShifts struct {
	source *repositories.TimeTrackingShifts
}

func NewTimeTrackingShifts(records repositories.TimeTrackingShiftRecords) *TimeTrackingShifts {
	return &TimeTrackingShifts{source: repositories.NewTimeTrackingShifts(records)}
}

func (r *TimeTrackingShifts) FindByStaffIDsAndDate(ctx context.Context, ids []int64, date timezone.Date) ([]*active.TimeTrackingShift, error) {
	rows, err := r.source.ForStaffOnDate(ctx, ids, date.String())
	return trackingShifts(rows), err
}
func (r *TimeTrackingShifts) FindByStaffAndDateRange(ctx context.Context, id int64, from, to timezone.Date) ([]*active.TimeTrackingShift, error) {
	rows, err := r.source.ForStaffInRange(ctx, id, from.String(), to.String())
	return trackingShifts(rows), err
}
func (r *TimeTrackingShifts) FindByDateRange(ctx context.Context, from, to timezone.Date) ([]*active.TimeTrackingShift, error) {
	rows, err := r.source.InRange(ctx, from.String(), to.String())
	return trackingShifts(rows), err
}
func (r *TimeTrackingShifts) FindByStaffIDsAndDateRange(ctx context.Context, ids []int64, from, to timezone.Date) (map[int64][]*active.TimeTrackingShift, error) {
	rows, err := r.source.ByStaffInRange(ctx, ids, from.String(), to.String())
	if rows == nil {
		return nil, err
	}
	result := make(map[int64][]*active.TimeTrackingShift, len(rows))
	for staffID, shifts := range rows {
		result[staffID] = trackingShifts(shifts)
	}
	return result, err
}
func trackingShifts(rows []*repositories.TimeTrackingShift) []*active.TimeTrackingShift {
	if rows == nil {
		return nil
	}
	result := make([]*active.TimeTrackingShift, len(rows))
	for i, row := range rows {
		if row != nil {
			result[i] = &active.TimeTrackingShift{StaffID: row.StaffID, Date: timezone.Date(row.Date), StartTime: row.StartTime, EndTime: row.EndTime, BreakMinutes: row.BreakMinutes, Cancelled: row.Cancelled}
		}
	}
	return result
}
