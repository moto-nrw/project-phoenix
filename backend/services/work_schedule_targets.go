package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

type WorkScheduleTargetRecords interface {
	FindByStaffIDsValidInRange(context.Context, []int64, config.CalendarDate, config.CalendarDate) ([]*config.StaffWorkSchedule, error)
	HasScheduleHistory(context.Context, int64) (bool, error)
	FindStaffIDsWithScheduleHistory(context.Context, []int64) (map[int64]bool, error)
}

type WorkScheduleTargets struct{ records WorkScheduleTargetRecords }

func NewWorkScheduleTargets(records WorkScheduleTargetRecords) *WorkScheduleTargets {
	return &WorkScheduleTargets{records: records}
}

func (r *WorkScheduleTargets) TargetsForStaff(ctx context.Context, id int64, from, to timezone.Date) (timetracking.WorkScheduleTargets, error) {
	rows, err := r.records.FindByStaffIDsValidInRange(ctx, []int64{id}, config.CalendarDate(from), config.CalendarDate(to))
	return workScheduleTargets(rows), err
}

func (r *WorkScheduleTargets) TargetsByStaff(ctx context.Context, ids []int64, from, to timezone.Date) (map[int64]timetracking.WorkScheduleTargets, error) {
	rows, err := r.records.FindByStaffIDsValidInRange(ctx, ids, config.CalendarDate(from), config.CalendarDate(to))
	if err != nil {
		return nil, err
	}
	byStaff := make(map[int64][]*config.StaffWorkSchedule, len(ids))
	for _, row := range rows {
		byStaff[row.StaffID] = append(byStaff[row.StaffID], row)
	}
	result := make(map[int64]timetracking.WorkScheduleTargets, len(byStaff))
	for staffID, entries := range byStaff {
		result[staffID] = workScheduleTargets(entries)
	}
	return result, nil
}

func (r *WorkScheduleTargets) HasScheduleHistory(ctx context.Context, id int64) (bool, error) {
	return r.records.HasScheduleHistory(ctx, id)
}
func (r *WorkScheduleTargets) FindStaffIDsWithScheduleHistory(ctx context.Context, ids []int64) (map[int64]bool, error) {
	return r.records.FindStaffIDsWithScheduleHistory(ctx, ids)
}

func workScheduleTargets(rows []*config.StaffWorkSchedule) timetracking.WorkScheduleTargets {
	return timetracking.WorkScheduleTargets{HasEntries: len(rows) > 0, DailyTarget: func(anchor *timezone.Date, date timezone.Date) (int, bool) {
		var legacyAnchor *config.CalendarDate
		if anchor != nil {
			value := config.CalendarDate(*anchor)
			legacyAnchor = &value
		}
		return config.DailyTargetFromSchedule(rows, legacyAnchor, config.CalendarDate(date))
	}}
}
