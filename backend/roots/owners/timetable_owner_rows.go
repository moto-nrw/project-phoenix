package owners

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// timetableGroupLookup is the School Structure group read the owner rows
// name the education groups through.
type timetableGroupLookup interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*schoolStructureCompose.Group, error)
}

// TimetableEducationGroupNames names School Structure's education groups
// for the Timetable owner rows (#3551).
func TimetableEducationGroupNames(groups timetableGroupLookup) timetablePorts.EducationGroupNames {
	return timetableEducationGroupNames{groups: groups}
}

type timetableEducationGroupNames struct {
	groups timetableGroupLookup
}

func (g timetableEducationGroupNames) EducationGroupNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	groups, err := g.groups.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(groups))
	for id, group := range groups {
		if group != nil {
			names[id] = group.Name
		}
	}
	return names, nil
}

// TimetableDeviationPorts binds the Audit Platform's Änderungsprotokoll to
// the Timetable owner rows: the reader, and the protocol, which stays unbound
// without a repository so the deviation writes fail closed.
func TimetableDeviationPorts(events TimetableDeviationTrail) (timetablePorts.DeviationEventReader, timetablePorts.DeviationProtocol) {
	if events == nil {
		return timetableDeviationEvents{}, nil
	}
	return timetableDeviationEvents{events: events}, TimetableDeviationProtocol(events)
}

// TimetableDeviationProtocol appends the owner's Änderungsprotokoll entries
// to the Audit Platform's deviation events.
func TimetableDeviationProtocol(events TimetableDeviationTrail) timetablePorts.DeviationProtocol {
	return timetableDeviationProtocol{events: events}
}

type timetableDeviationProtocol struct {
	events TimetableDeviationTrail
}

func (p timetableDeviationProtocol) RecordDeviationEvent(ctx context.Context, event timetablePorts.DeviationEventRecord) error {
	return p.events.Create(ctx, &auditModels.DeviationEvent{
		ActivityGroupID: event.ActivityGroupID,
		OccurrenceDate:  auditModels.Date(event.OccurrenceDate),
		StartTime:       event.StartTime,
		InstanceID:      event.InstanceID,
		SubjectStaffID:  event.SubjectStaffID,
		RelatedStaffID:  event.RelatedStaffID,
		EventType:       event.EventType,
		OldValue:        event.OldValue,
		NewValue:        event.NewValue,
		Reason:          event.Reason,
		ActorAccountID:  event.ActorAccountID,
	})
}

type timetableDeviationEvents struct {
	events TimetableDeviationTrail
}

func (e timetableDeviationEvents) ListDeviationEvents(ctx context.Context, from, to calendar.Date, activityGroupID *int64, startTime *string) ([]timetable.DeviationEvent, error) {
	rows, err := e.events.ListByRange(ctx, auditModels.Date(from), auditModels.Date(to), activityGroupID, startTime)
	if err != nil {
		return nil, err
	}
	events := make([]timetable.DeviationEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, timetable.DeviationEvent{
			ID: row.ID, ActivityGroupID: row.ActivityGroupID, OccurrenceDate: calendar.Date(row.OccurrenceDate),
			StartTime: row.StartTime, InstanceID: row.InstanceID, EventType: row.EventType,
			SubjectStaffID: row.SubjectStaffID, RelatedStaffID: row.RelatedStaffID, ActorAccountID: row.ActorAccountID,
			OldValue: row.OldValue, NewValue: row.NewValue, Reason: row.Reason, OccurredAt: row.OccurredAt,
		})
	}
	return events, nil
}
