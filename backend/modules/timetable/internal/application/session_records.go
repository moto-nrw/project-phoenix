package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SessionRecords exposes native values over the same joined rows and commands.
type SessionRecords struct {
	Instances ports.ActivityInstanceRecords
	Staff     ports.InstanceStaffRecords
	Groups    ports.ActivityGroupRecords
}

func (r SessionRecords) ListTemplateGroups(ctx context.Context) ([]timetable.Group, error) {
	rows, err := r.Groups.ListWithCategory(ctx, nil)
	if err != nil {
		return nil, err
	}
	result := make([]timetable.Group, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, CatalogGroup(row))
		}
	}
	return result, nil
}

func (r SessionRecords) FindSessionInstance(ctx context.Context, activeGroupID int64) (*timetable.ScheduledInstance, error) {
	row, err := r.Instances.FindByActiveGroupID(ctx, activeGroupID)
	if row == nil {
		return nil, err
	}
	result := ScheduledInstanceOf(row)
	return &result, err
}

func (r SessionRecords) ListSessionInstances(ctx context.Context, date calendar.Date) ([]timetable.ScheduledInstance, error) {
	rows, err := r.Instances.FindByTenantAndDate(ctx, calendar.Date(date))
	if err != nil {
		return nil, err
	}
	result := make([]timetable.ScheduledInstance, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, ScheduledInstanceOf(row))
		}
	}
	return result, nil
}

// CreateMirroredSessionInstance records a running kiosk session as a
// spontaneous, active instance whatever the input says about either.
func (r SessionRecords) CreateMirroredSessionInstance(ctx context.Context, input timetable.ScheduledInstance) (int64, error) {
	row := &schedule.ActivityInstance{
		Date: calendar.Date(input.Date), ActivityGroupID: input.ActivityGroupID, CalendarPeriodID: input.CalendarPeriodID, Title: input.Title, Description: input.Description,
		StartTime: calendar.NormalizeWallClock(input.StartTime), EndTime: calendar.NormalizeWallClock(input.EndTime), RoomID: input.RoomID,
		Status: schedule.InstanceStatusActive, ActiveGroupID: input.ActiveGroupID, IsSpontaneous: true,
		CreatedBy: input.CreatedBy, StartedBy: input.StartedBy, StartedAt: input.StartedAt,
		RequiredStaff: input.RequiredStaff, ListKind: input.ListKind, UnderstaffedAck: input.UnderstaffedAck,
		UnderstaffedNote: input.UnderstaffedNote, CancelReason: input.CancelReason, Notes: input.Notes,
	}
	row.SetTenantID(tenant.FromContext(ctx))
	if err := r.Instances.Create(ctx, row); err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r SessionRecords) AddSessionInstanceStaff(ctx context.Context, instanceID, staffID int64) error {
	row := &schedule.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
	row.SetTenantID(tenant.FromContext(ctx))
	return r.Staff.Create(ctx, row)
}
