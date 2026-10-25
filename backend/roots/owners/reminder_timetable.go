package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// ReminderTimetableReader binds the reminder evaluation to Timetable. Today's
// blocks come from the session records, whose status carries the execution
// state (planned, active, completed, cancelled) of Student Presence: the plan
// table alone knows only planned and cancelled, so a started block would read
// as "not started" and a running one could never be told apart (#4765).
type ReminderTimetableReader struct {
	Source interface {
		timetable.InstanceStaffQuery
	}
	Sessions interface {
		ListSessionInstances(context.Context, calendar.Date) ([]timetable.ScheduledInstance, error)
	}
}

func (r ReminderTimetableReader) FindByTenantAndDate(ctx context.Context, date string) ([]*ports.ActivityInstance, error) {
	day, err := calendar.ParseDate(date)
	if err != nil {
		return nil, timetableCompose.WrapDatabaseError("find by tenant and date", fmt.Errorf("parse activity instance date: %w", err))
	}
	values, err := r.Sessions.ListSessionInstances(ctx, day)
	if err != nil {
		return nil, err
	}
	result := make([]*ports.ActivityInstance, 0, len(values))
	for _, value := range values {
		result = append(result, &ports.ActivityInstance{
			ID: value.ID, Title: value.Title, RoomID: value.RoomID,
			StartTime: value.StartTime, EndTime: value.EndTime, Status: value.Status,
		})
	}
	return result, nil
}

func (r ReminderTimetableReader) FindByInstanceIDs(ctx context.Context, ids []int64) ([]*ports.InstanceStaff, error) {
	if len(ids) == 0 {
		return []*ports.InstanceStaff{}, nil
	}
	values, err := r.Source.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: ids, OrderByInstanceAndCreated: true})
	if err != nil {
		return nil, timetableCompose.WrapDatabaseError("find by instance ids", err)
	}
	result := make([]*ports.InstanceStaff, 0, len(values))
	for _, value := range values {
		result = append(result, &ports.InstanceStaff{InstanceID: value.InstanceID, StaffID: value.StaffID, IsAbsent: value.IsAbsent, IsSubstitute: value.IsSubstitute})
	}
	return result, nil
}
