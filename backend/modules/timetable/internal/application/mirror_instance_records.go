package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	models "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

func mirrorInstance(row *models.ActivityInstance) *timetable.ScheduledInstance {
	if row == nil {
		return nil
	}
	value := ScheduledInstanceOf(row)
	return &value
}
func NativeMirrorInstances(rows ports.ActivityInstanceRecords) *ports.MirrorInstanceRecords {
	if rows == nil {
		return nil
	}
	return &ports.MirrorInstanceRecords{MirrorInstances: &ports.MirrorInstances{FindByActiveGroupID: func(ctx context.Context, id int64) (*timetable.ScheduledInstance, error) {
		row, err := rows.FindByActiveGroupID(ctx, id)
		return mirrorInstance(row), err
	}, FindByActiveGroups: func(ctx context.Context, ids []int64) ([]*timetable.ScheduledInstance, error) {
		rows, err := listInstances(ctx, rows, ports.LegacyInstanceFilter{ActiveGroupIDs: ids})
		if rows == nil {
			return nil, err
		}
		out := make([]*timetable.ScheduledInstance, len(rows))
		for i, row := range rows {
			out[i] = mirrorInstance(row)
		}
		return out, err
	}}, Create: func(ctx context.Context, value *timetable.ScheduledInstance) error {
		var row *models.ActivityInstance
		if value != nil {
			row = &models.ActivityInstance{Date: calendar.Date(value.Date), ActivityGroupID: value.ActivityGroupID, CalendarPeriodID: value.CalendarPeriodID, Title: value.Title, Description: value.Description, StartTime: value.StartTime, EndTime: value.EndTime, RoomID: value.RoomID, RequiredStaff: value.RequiredStaff, Status: value.Status, ActiveGroupID: value.ActiveGroupID, ListKind: value.ListKind, IsSpontaneous: value.IsSpontaneous, UnderstaffedAck: value.UnderstaffedAck, UnderstaffedNote: value.UnderstaffedNote, CancelReason: value.CancelReason, Notes: value.Notes, CreatedBy: value.CreatedBy, StartedBy: value.StartedBy, StartedAt: value.StartedAt, CompletedAt: value.CompletedAt, CompletedBy: value.CompletedBy, ReopenUntil: value.ReopenUntil}
			row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
		}
		err := rows.Create(ctx, row)
		if value != nil && row != nil {
			*value = *mirrorInstance(row)
		}
		return err
	}, FindByID: func(ctx context.Context, id any) (*timetable.ScheduledInstance, error) {
		recordID, ok := LegacyGroupID(id)
		if !ok {
			return nil, WrapDatabaseError("find by id", fmt.Errorf("invalid activity instance id %T", id))
		}
		row, err := rows.FindByID(ctx, recordID)
		return mirrorInstance(row), err
	}}
}
