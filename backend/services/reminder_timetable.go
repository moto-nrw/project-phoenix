package services

import (
	"context"
	"fmt"
	"time"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

type reminderTimetableReader struct {
	source interface {
		timetable.ActivityInstanceQuery
		timetable.InstanceStaffQuery
	}
}

func (r reminderTimetableReader) FindByTenantAndDate(ctx context.Context, date string) ([]*ports.ActivityInstance, error) {
	values, err := r.source.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{Date: &date, OrderByDateAndTime: true})
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find by tenant and date", err)
	}
	result := make([]*ports.ActivityInstance, 0, len(values))
	for _, value := range values {
		instance, err := reminderInstance(value)
		if err != nil {
			return nil, scheduleRepo.WrapDatabaseError("find by tenant and date", err)
		}
		result = append(result, instance)
	}
	return result, nil
}

func reminderInstance(value timetable.ActivityInstance) (*ports.ActivityInstance, error) {
	// Retain the compatibility adapter's validation and error contract.
	if _, err := scheduleRepo.ParseActivityInstanceDate(value.Date); err != nil {
		return nil, fmt.Errorf("parse activity instance date: %w", err)
	}
	start, err := time.Parse("15:04:05", value.StartTime)
	if err != nil {
		return nil, fmt.Errorf("parse activity instance start time: %w", err)
	}
	end, err := time.Parse("15:04:05", value.EndTime)
	if err != nil {
		return nil, fmt.Errorf("parse activity instance end time: %w", err)
	}
	return &ports.ActivityInstance{ID: value.ID, Title: value.Title, RoomID: value.RoomID, StartTime: start, EndTime: end, Status: value.Status}, nil
}

func (r reminderTimetableReader) FindByInstanceIDs(ctx context.Context, ids []int64) ([]*ports.InstanceStaff, error) {
	if len(ids) == 0 {
		return []*ports.InstanceStaff{}, nil
	}
	values, err := r.source.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: ids, OrderByInstanceAndCreated: true})
	if err != nil {
		return nil, scheduleRepo.WrapDatabaseError("find by instance ids", err)
	}
	result := make([]*ports.InstanceStaff, 0, len(values))
	for _, value := range values {
		result = append(result, &ports.InstanceStaff{InstanceID: value.InstanceID, StaffID: value.StaffID, IsAbsent: value.IsAbsent, IsSubstitute: value.IsSubstitute})
	}
	return result, nil
}
