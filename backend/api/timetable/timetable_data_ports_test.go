package timetable

import (
	"context"
	"errors"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

func (p testPickupBaselines) ProjectPickups(ctx context.Context, studentIDs []int64, from, to calendar.Date) (timetableCompose.PickupBaselines, error) {
	projection, err := p.reader.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return testPickupProjection{projection: projection}, nil
}

type testPickupProjection struct {
	projection *careplan.PickupBaselineProjection
}

func (p testPickupProjection) RegularPickup(studentID int64, date calendar.Date) (time.Time, bool) {
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

func (e testDeviationEvents) ListDeviationEvents(ctx context.Context, from, to calendar.Date, activityGroupID *int64, startTime *string) ([]timetable.DeviationEvent, error) {
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

// fakeTimetableData is a capability-level fake of the Timetable owner's
// planner reads for database-free handler tests. Each overridable method
// answers from its func field; an unset field, like every other method of
// the capability, reaches the nil embedded capability and panics, so an
// accidental dependency fails loudly.
type fakeTimetableData struct {
	timetable.TimetableDataCapability

	FindScheduledInstanceFn        func(ctx context.Context, id int64) (timetable.ScheduledInstance, error)
	FindBlockParticipantFn         func(ctx context.Context, instanceID, studentID int64) (*timetable.ScheduledParticipant, error)
	LockBlockAttendanceFn          func(ctx context.Context, instanceID int64) error
	PatchSlotAttendanceFn          func(ctx context.Context, participantID int64, patch timetable.AttendancePatch) error
	ListTemplateEntriesForPeriodFn func(ctx context.Context, periodID *int64, childrenPerStaffRatio int) ([]timetable.TemplateListEntry, error)
	ListTemplateWeekdayRosterFn    func(ctx context.Context, templateID, calendarPeriodID *int64) ([]timetable.TemplateWeekdayRosterRow, error)
	SpontaneousRoomExistsFn        func(ctx context.Context, roomID int64) (bool, error)
	LockSpontaneousStartRoomFn     func(ctx context.Context, roomID int64) error
	SpontaneousRoomOccupiedFn      func(ctx context.Context, roomID int64) (bool, error)
	ResolveSpontaneousActivityFn   func(ctx context.Context, title string, requestedID *int64, createdBy int64) (*int64, error)
}

func (f *fakeTimetableData) FindScheduledInstance(ctx context.Context, id int64) (timetable.ScheduledInstance, error) {
	if f.FindScheduledInstanceFn == nil {
		return f.TimetableDataCapability.FindScheduledInstance(ctx, id)
	}
	return f.FindScheduledInstanceFn(ctx, id)
}

func (f *fakeTimetableData) FindBlockParticipant(ctx context.Context, instanceID, studentID int64) (*timetable.ScheduledParticipant, error) {
	if f.FindBlockParticipantFn == nil {
		return f.TimetableDataCapability.FindBlockParticipant(ctx, instanceID, studentID)
	}
	return f.FindBlockParticipantFn(ctx, instanceID, studentID)
}

func (f *fakeTimetableData) LockBlockAttendance(ctx context.Context, instanceID int64) error {
	if f.LockBlockAttendanceFn == nil {
		return f.TimetableDataCapability.LockBlockAttendance(ctx, instanceID)
	}
	return f.LockBlockAttendanceFn(ctx, instanceID)
}

func (f *fakeTimetableData) PatchSlotAttendance(ctx context.Context, participantID int64, patch timetable.AttendancePatch) error {
	if f.PatchSlotAttendanceFn == nil {
		return f.TimetableDataCapability.PatchSlotAttendance(ctx, participantID, patch)
	}
	return f.PatchSlotAttendanceFn(ctx, participantID, patch)
}

func (f *fakeTimetableData) ListTemplateEntriesForPeriod(ctx context.Context, periodID *int64, childrenPerStaffRatio int) ([]timetable.TemplateListEntry, error) {
	if f.ListTemplateEntriesForPeriodFn == nil {
		return f.TimetableDataCapability.ListTemplateEntriesForPeriod(ctx, periodID, childrenPerStaffRatio)
	}
	return f.ListTemplateEntriesForPeriodFn(ctx, periodID, childrenPerStaffRatio)
}

func (f *fakeTimetableData) ListTemplateWeekdayRoster(ctx context.Context, templateID, calendarPeriodID *int64) ([]timetable.TemplateWeekdayRosterRow, error) {
	if f.ListTemplateWeekdayRosterFn == nil {
		return f.TimetableDataCapability.ListTemplateWeekdayRoster(ctx, templateID, calendarPeriodID)
	}
	return f.ListTemplateWeekdayRosterFn(ctx, templateID, calendarPeriodID)
}

func (f *fakeTimetableData) SpontaneousRoomExists(ctx context.Context, roomID int64) (bool, error) {
	if f.SpontaneousRoomExistsFn == nil {
		return f.TimetableDataCapability.SpontaneousRoomExists(ctx, roomID)
	}
	return f.SpontaneousRoomExistsFn(ctx, roomID)
}

func (f *fakeTimetableData) LockSpontaneousStartRoom(ctx context.Context, roomID int64) error {
	if f.LockSpontaneousStartRoomFn == nil {
		return f.TimetableDataCapability.LockSpontaneousStartRoom(ctx, roomID)
	}
	return f.LockSpontaneousStartRoomFn(ctx, roomID)
}

func (f *fakeTimetableData) SpontaneousRoomOccupied(ctx context.Context, roomID int64) (bool, error) {
	if f.SpontaneousRoomOccupiedFn == nil {
		return f.TimetableDataCapability.SpontaneousRoomOccupied(ctx, roomID)
	}
	return f.SpontaneousRoomOccupiedFn(ctx, roomID)
}

func (f *fakeTimetableData) ResolveSpontaneousActivity(ctx context.Context, title string, requestedID *int64, createdBy int64) (*int64, error) {
	if f.ResolveSpontaneousActivityFn == nil {
		return f.TimetableDataCapability.ResolveSpontaneousActivity(ctx, title, requestedID, createdBy)
	}
	return f.ResolveSpontaneousActivityFn(ctx, title, requestedID, createdBy)
}
