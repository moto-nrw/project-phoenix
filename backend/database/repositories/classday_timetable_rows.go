package repositories

import (
	"context"

	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// NewClassDayTimetableRows binds the slot lists' block and roster reads to
// the two owners of a block since the presence cutover (#2762): Timetable
// plans it and its participants, Student Presence runs it and records the
// attendance. Each read is one statement over the tenant-safe projection.
func NewClassDayTimetableRows(db *bun.DB) classdayCompose.TimetableReader {
	return classDayTimetableRows{reads: mustPresenceReads(db)}
}

type classDayTimetableRows struct {
	reads timetableCompose.PresenceReads
}

func (r classDayTimetableRows) ListActivityInstancesOn(ctx context.Context, date string) ([]classdayCompose.SlotBlock, error) {
	instances, err := r.reads.ListLegacyInstances(ctx, timetableCompose.LegacyInstanceFilter{Date: &date, OrderByDateAndTime: true})
	if err != nil {
		return nil, err
	}
	result := make([]classdayCompose.SlotBlock, 0, len(instances))
	for _, instance := range instances {
		result = append(result, classdayCompose.SlotBlock{
			ID: instance.ID, Title: instance.Title, Date: instance.Date.String(), StartTime: instance.StartTime.Format("15:04:05"),
			EndTime: instance.EndTime.Format("15:04:05"), RoomID: instance.RoomID, Status: instance.Status,
			ActiveGroupID: instance.ActiveGroupID, ListKind: instance.ListKind,
		})
	}
	return result, nil
}

func (r classDayTimetableRows) ListRoster(ctx context.Context, instanceIDs []int64) ([]classdayCompose.SlotRosterRow, error) {
	if len(instanceIDs) == 0 {
		return []classdayCompose.SlotRosterRow{}, nil
	}
	participants, err := r.reads.ListLegacyParticipants(ctx, timetableCompose.LegacyParticipantFilter{InstanceIDs: instanceIDs, OrderByInstanceStudent: true})
	if err != nil {
		return nil, err
	}
	result := make([]classdayCompose.SlotRosterRow, 0, len(participants))
	for _, participant := range participants {
		result = append(result, classdayCompose.SlotRosterRow{
			ID: participant.ID, InstanceID: participant.InstanceID, StudentID: participant.StudentID, RoomID: participant.RoomID,
			Status: participant.Status, Substatus: participant.Substatus, Note: participant.Note,
			CheckedInAt: participant.CheckedInAt, CheckedOutAt: participant.CheckedOutAt,
			IsUnplanned: participant.IsUnplanned, NotScheduled: participant.NotScheduled, ManualStatusAt: participant.ManualStatusAt,
			StudentStatusDayID: participant.StudentStatusDayID, PickupExceptionID: participant.PickupExceptionID,
		})
	}
	return result, nil
}
