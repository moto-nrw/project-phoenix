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
	Instances ports.InstanceRows
	Staff     timetable.InstanceStaffCapability
	Groups    timetable.ActivityGroupQuery
}

func (r SessionRecords) ListTemplateGroups(ctx context.Context) ([]timetable.Group, error) {
	groups, err := r.Groups.ListGroups(ctx, timetable.GroupFilter{})
	if err != nil {
		return nil, legacyGroupReadError("list", err)
	}
	return groups, nil
}

func (r SessionRecords) FindSessionInstance(ctx context.Context, activeGroupID int64) (*timetable.ScheduledInstance, error) {
	rows, err := r.Instances.Reads.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{ActiveGroupIDs: []int64{activeGroupID}, Limit: 1})
	if err != nil {
		return nil, WrapDatabaseError("find by active group id", err)
	}
	if len(rows) == 0 || rows[0] == nil {
		return nil, nil
	}
	result := ScheduledInstanceOf(rows[0])
	return &result, nil
}

func (r SessionRecords) ListSessionInstances(ctx context.Context, date calendar.Date) ([]timetable.ScheduledInstance, error) {
	day := date.String()
	rows, err := r.Instances.Reads.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{Date: &day, OrderByDateAndTime: true})
	if err != nil {
		return nil, WrapDatabaseError("find by tenant and date", err)
	}
	result := make([]timetable.ScheduledInstance, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			result = append(result, ScheduledInstanceOf(row))
		}
	}
	return result, nil
}

// ListActiveSessionInstances returns every still-running session in the tenant,
// including a block started before midnight.
func (r SessionRecords) ListActiveSessionInstances(ctx context.Context) ([]timetable.ScheduledInstance, error) {
	return listActiveSessionInstances(ctx, r.Instances.Reads)
}

func listActiveSessionInstances(ctx context.Context, reads ports.LegacyInstanceReads) ([]timetable.ScheduledInstance, error) {
	rows, err := reads.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{Status: timetable.InstanceStatusActive, OrderByDateAndTime: true})
	if err != nil {
		return nil, WrapDatabaseError("list active", err)
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
	if err := createLegacyInstance(ctx, r.Instances, row); err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (r SessionRecords) AddSessionInstanceStaff(ctx context.Context, instanceID, staffID int64) error {
	row := &schedule.InstanceStaff{InstanceID: instanceID, StaffID: staffID}
	if err := row.Validate(); err != nil {
		return err
	}
	if _, err := r.Staff.CreateInstanceStaff(ctx, publicInstanceStaffInput(row)); err != nil {
		return WrapDatabaseError("create", err)
	}
	return nil
}
