package timetable

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The ports the Timetable owner's planner reads consume, bound the way the
// composition root binds them (services/timetable_data_composition.go).

// testPickupBaselines serves the pickup port from Care Plan's baseline
// projection.
type testPickupBaselines struct {
	reader careplan.PickupBaselineReader
}

func (p testPickupBaselines) ProjectPickups(ctx context.Context, studentIDs []int64, from, to timezone.Date) (timetableCompose.PickupBaselines, error) {
	projection, err := p.reader.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return testPickupProjection{projection: projection}, nil
}

type testPickupProjection struct {
	projection *careplan.PickupBaselineProjection
}

func (p testPickupProjection) RegularPickup(studentID int64, date timezone.Date) (time.Time, bool) {
	row := p.projection.ForDate(studentID, date)
	if row == nil {
		return time.Time{}, false
	}
	return row.PickupTime, true
}

// testRoomNames names the Facilities rooms.
type testRoomNames struct {
	rooms facilitiesModels.RoomRepository
}

func (r testRoomNames) AllRoomNames(ctx context.Context) (map[int64]string, error) {
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

func (r testRoomNames) RoomName(ctx context.Context, id int64) (string, bool, error) {
	room, err := r.rooms.FindByID(ctx, id)
	if err != nil || room == nil {
		return "", false, err
	}
	return room.Name, true, nil
}

// testRoomOccupancy asks Student Presence whether another open session
// occupies the room.
type testRoomOccupancy struct {
	sessions studentpresence.SessionRecords
}

func (o testRoomOccupancy) RoomOccupied(ctx context.Context, roomID int64) (bool, error) {
	occupied, _, err := o.sessions.CheckRoomConflict(ctx, roomID, 0)
	return occupied, err
}

// testDeviationEvents reads the Audit Platform's Änderungsprotokoll.
type testDeviationEvents struct {
	events auditModels.DeviationEventRepository
}

func (e testDeviationEvents) ListDeviationEvents(ctx context.Context, from, to timezone.Date, activityGroupID *int64, startTime *string) ([]timetable.DeviationEvent, error) {
	rows, err := e.events.ListByRange(ctx, auditModels.Date(from), auditModels.Date(to), activityGroupID, startTime)
	if err != nil {
		return nil, err
	}
	events := make([]timetable.DeviationEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, timetable.DeviationEvent{
			ID: row.ID, ActivityGroupID: row.ActivityGroupID, OccurrenceDate: timezone.Date(row.OccurrenceDate),
			StartTime: row.StartTime, InstanceID: row.InstanceID, EventType: row.EventType,
			SubjectStaffID: row.SubjectStaffID, RelatedStaffID: row.RelatedStaffID, ActorAccountID: row.ActorAccountID,
			OldValue: row.OldValue, NewValue: row.NewValue, Reason: row.Reason, OccurredAt: row.OccurredAt,
		})
	}
	return events, nil
}

// testAdvisoryLocks takes transaction-scoped advisory locks on the request's
// transaction, with the statement the Transaction Runtime issues.
type testAdvisoryLocks struct{}

func (testAdvisoryLocks) AcquireXactLock(ctx context.Context, key string) error {
	value, _ := tenant.TransactionFromContext(ctx)
	tx, ok := value.(bun.IDB)
	if !ok {
		return errors.New("tenant transaction is required")
	}
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	return err
}

// unitDataDeps selects the collaborators a database-free test drives through
// the real planner reads. Unset ones are stubs that panic when reached, so
// an accidental dependency fails loudly; without Locks the attendance lock
// and without a database the spontaneous-start locks are skipped.
type unitDataDeps struct {
	Instances    timetableCompose.DataInstances
	Participants timetableCompose.DataParticipants
	Templates    timetableCompose.DataTemplates
	Groups       timetableCompose.DataGroups
	Categories   timetableCompose.DataCategories
	Rooms        facilitiesModels.RoomRepository
	Sessions     studentpresence.SessionRecords
	Locks        timetableCompose.AttendanceLocks
}

type (
	stubDataInstances     struct{ timetableCompose.DataInstances }
	stubDataInstanceStaff struct {
		timetableCompose.OperationInstanceStaff
	}
	stubDataParticipants struct {
		timetableCompose.DataParticipants
	}
	stubDataPickupExceptions struct {
		timetableCompose.DataPickupExceptions
	}
	stubDataArrivalExceptions struct {
		timetableCompose.DataArrivalExceptions
	}
	stubDataVisits struct {
		timetableCompose.OperationVisits
	}
	stubDataTemplates  struct{ timetableCompose.DataTemplates }
	stubDataGroups     struct{ timetableCompose.DataGroups }
	stubDataCategories struct {
		timetableCompose.DataCategories
	}
	stubDataRooms struct {
		facilitiesModels.RoomRepository
	}
	stubDataSessions        struct{ studentpresence.SessionRecords }
	stubDataDeviationEvents struct {
		timetableCompose.DeviationEventReader
	}
	stubDataConflictAcks struct {
		timetable.ConflictAckCapability
	}
)

// unitTimetableData composes the Timetable owner's planner reads over the
// given fakes.
func unitTimetableData(deps unitDataDeps) timetable.TimetableDataCapability {
	composed := timetableCompose.TimetableDataDependencies{
		Instances:         stubDataInstances{},
		InstanceStaff:     stubDataInstanceStaff{},
		Participants:      stubDataParticipants{},
		PickupExceptions:  stubDataPickupExceptions{},
		ArrivalExceptions: stubDataArrivalExceptions{},
		Visits:            stubDataVisits{},
		Templates:         stubDataTemplates{},
		Groups:            stubDataGroups{},
		Categories:        stubDataCategories{},
		Rooms:             testRoomNames{rooms: stubDataRooms{}},
		RoomOccupancy:     testRoomOccupancy{sessions: stubDataSessions{}},
		DeviationEvents:   stubDataDeviationEvents{},
		ConflictAcks:      stubDataConflictAcks{},
		Locks:             deps.Locks,
	}
	if deps.Instances != nil {
		composed.Instances = deps.Instances
	}
	if deps.Participants != nil {
		composed.Participants = deps.Participants
	}
	if deps.Templates != nil {
		composed.Templates = deps.Templates
	}
	if deps.Groups != nil {
		composed.Groups = deps.Groups
	}
	if deps.Categories != nil {
		composed.Categories = deps.Categories
	}
	if deps.Rooms != nil {
		composed.Rooms = testRoomNames{rooms: deps.Rooms}
	}
	if deps.Sessions != nil {
		composed.RoomOccupancy = testRoomOccupancy{sessions: deps.Sessions}
	}
	data, err := timetableCompose.NewTimetableData(composed)
	if err != nil {
		panic(err)
	}
	return data
}

// plannedInstanceRepo answers every block as planned, so the attendance
// freeze check lets a write through.
type plannedInstanceRepo struct {
	timetableCompose.DataInstances
}

func (plannedInstanceRepo) FindByID(_ context.Context, id any) (*schedule.ActivityInstance, error) {
	instance := &schedule.ActivityInstance{Status: schedule.InstanceStatusPlanned}
	if value, ok := id.(int64); ok {
		instance.ID = value
	}
	return instance, nil
}
