package repositories

import (
	"context"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimetableOwnerRows are the retained repository rows the Timetable owner's
// operational day, planner reads and ended-session completion run over
// (#3551). Their status carries the Student Presence session state the
// owner's own rows do not. The composition root fills them from its
// repository set and hands them to the owner's ports; the readers below
// serve the ports whose retained repository needs a narrower view, and the
// Facilities room names, School Structure group names and the Audit
// Platform's Änderungsprotokoll the owner may not name itself.
type TimetableOwnerRows struct {
	Instances         scheduleModels.ActivityInstanceRepository
	InstanceStaff     scheduleModels.InstanceStaffRepository
	Participants      scheduleModels.InstanceStudentRepository
	Templates         activitiesModels.GroupRepository
	Categories        activitiesModels.CategoryRepository
	Students          usersModels.StudentRepository
	EducationGroups   educationModels.GroupRepository
	Rooms             facilitiesModels.RoomRepository
	PickupExceptions  scheduleModels.StudentPickupExceptionRepository
	ArrivalExceptions scheduleModels.StudentArrivalExceptionRepository
	DeviationEvents   auditModels.DeviationEventRepository
	// Locks serializes attendance writes with a completion; optional.
	Locks scheduleModels.ActivityRecoveryRepository
}

// OperationTemplates is the template read of the operational day, which the
// retained activity group repository serves beside its model interface.
func (r TimetableOwnerRows) OperationTemplates() (timetableCompose.OperationTemplates, error) {
	templates, ok := r.Templates.(timetableCompose.OperationTemplates)
	if !ok {
		return nil, fmt.Errorf("timetable operations: %T cannot read template targets", r.Templates)
	}
	return templates, nil
}

// DataTemplates is the Vorlagen read of the planner.
func (r TimetableOwnerRows) DataTemplates() (timetableCompose.DataTemplates, error) {
	templates, ok := r.Templates.(timetableCompose.DataTemplates)
	if !ok {
		return nil, fmt.Errorf("timetable data: %T cannot read the template list", r.Templates)
	}
	return templates, nil
}

// EndedSessionInstances is the session listing of the ended-session
// completion.
func (r TimetableOwnerRows) EndedSessionInstances() (timetableCompose.EndedSessionInstances, error) {
	instances, ok := r.Instances.(timetableCompose.EndedSessionInstances)
	if !ok {
		return nil, fmt.Errorf("ended session completion: %T cannot list instances by session", r.Instances)
	}
	return instances, nil
}

// AttendanceLocks is the optional completion lock; nil without Locks.
func (r TimetableOwnerRows) AttendanceLocks() timetableCompose.AttendanceLocks {
	if r.Locks == nil {
		return nil
	}
	return r.Locks
}

// RoomNames names the Facilities rooms.
func (r TimetableOwnerRows) RoomNames() timetableCompose.RoomNames {
	return timetableRoomNames{rooms: r.Rooms}
}

// EducationGroupNames names School Structure's education groups.
func (r TimetableOwnerRows) EducationGroupNames() timetableCompose.EducationGroupNames {
	return timetableEducationGroupNames{groups: r.EducationGroups}
}

// DeviationEventReader reads the Audit Platform's Änderungsprotokoll.
func (r TimetableOwnerRows) DeviationEventReader() timetableCompose.DeviationEventReader {
	return timetableDeviationEvents{events: r.DeviationEvents}
}

// SickCascadeRows are the Betreuungsplan rows the #1843 sick cascade reads,
// with the day-wide staffing lock (#1840) the caller supplies.
func (r TimetableOwnerRows) SickCascadeRows(dayLock func(context.Context, calendar.Date) error) SickCascadeTimetableRows {
	return SickCascadeTimetableRows{instances: r.Instances, staff: r.InstanceStaff, dayLock: dayLock}
}

// SickCascadeTimetableRows serves the shift-plan-sync workflow's timetable
// port from the retained rows.
type SickCascadeTimetableRows struct {
	instances scheduleModels.ActivityInstanceRepository
	staff     scheduleModels.InstanceStaffRepository
	dayLock   func(context.Context, calendar.Date) error
}

// AcquireSubstituteDayLock serializes the day-wide staffing mutations of the
// tenant within the caller's transaction.
func (r SickCascadeTimetableRows) AcquireSubstituteDayLock(ctx context.Context, date calendar.Date) error {
	return r.dayLock(ctx, date)
}

func (r SickCascadeTimetableRows) GetInstanceStaffByStaffAndDate(ctx context.Context, staffID int64, date calendar.Date) ([]*scheduleModels.InstanceStaff, error) {
	return r.staff.FindByStaffAndDate(ctx, staffID, scheduleModels.Date(date))
}

func (r SickCascadeTimetableRows) GetActivityInstancesByID(ctx context.Context, ids []int64) (map[int64]*scheduleModels.ActivityInstance, error) {
	instances, err := r.instances.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*scheduleModels.ActivityInstance, len(instances))
	for _, instance := range instances {
		if instance != nil {
			byID[instance.ID] = instance
		}
	}
	return byID, nil
}

func (r SickCascadeTimetableRows) GetInstanceStaff(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStaff, error) {
	return r.staff.FindByInstanceID(ctx, instanceID)
}

type timetableEducationGroupNames struct {
	groups educationModels.GroupRepository
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

type timetableRoomNames struct {
	rooms facilitiesModels.RoomRepository
}

func (r timetableRoomNames) AllRoomNames(ctx context.Context) (map[int64]string, error) {
	rooms, err := r.rooms.List(ctx, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rooms))
	for _, room := range rooms {
		if room != nil {
			names[room.ID] = room.Name
		}
	}
	return names, nil
}

func (r timetableRoomNames) RoomName(ctx context.Context, id int64) (string, bool, error) {
	room, err := r.rooms.FindByID(ctx, id)
	if err != nil || room == nil {
		return "", false, err
	}
	return room.Name, true, nil
}

type timetableDeviationEvents struct {
	events auditModels.DeviationEventRepository
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
