package timetablehttp

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// testRoomOccupancy asks Student Presence whether another open session
// occupies the room, the way the composition root binds it
// (services/timetable_data_composition.go).
type testRoomOccupancy struct {
	sessions studentpresence.SessionRecords
}

func (o testRoomOccupancy) SpontaneousRoomOccupied(ctx context.Context, roomID int64) (bool, error) {
	occupied, _, err := o.sessions.CheckRoomConflict(ctx, roomID, 0)
	return occupied, err
}

// unitDataDeps selects the collaborators a database-free test drives through
// the real planner reads. Unset ones are stubs that panic when reached, so
// an accidental dependency fails loudly; without Locks the attendance lock
// and without a database the spontaneous-start locks are skipped.
type unitDataDeps struct {
	Instances     timetablePorts.LegacyInstanceReads
	Participants  timetablePorts.ParticipantRows
	Templates     timetablePorts.DataTemplates
	Groups        timetablePorts.DataGroups
	BlockMetadata timetablePorts.BlockDisplayMetadata
	Categories    timetablePorts.DataCategories
	Rooms         timetablePorts.RoomNames
	Sessions      studentpresence.SessionRecords
	Locks         timetablePorts.AttendanceLocks
}

type (
	stubDataInstances struct {
		timetablePorts.LegacyInstanceReads
	}
	stubDataInstanceStaff struct {
		timetable.InstanceStaffQuery
	}
	stubParticipantPlans struct {
		timetable.InstanceStudentCapability
	}
	stubParticipantPresence struct {
		timetablePorts.ParticipantPresence
	}
	stubParticipantReads struct {
		timetablePorts.LegacyParticipantReads
	}
	stubDataPickupExceptions struct {
		timetablePorts.DataPickupExceptions
	}
	stubDataArrivalExceptions struct {
		timetablePorts.DataArrivalExceptions
	}
	stubDataVisits struct {
		timetablePorts.OperationVisits
	}
	stubDataTemplates struct{ timetablePorts.DataTemplates }
	stubDataGroups    struct{ timetablePorts.DataGroups }
	stubBlockMetadata struct {
		timetablePorts.BlockDisplayMetadata
	}
	stubDataCategories struct {
		timetablePorts.DataCategories
	}
	stubDataRooms struct {
		timetablePorts.RoomNames
	}
	stubDataSessions        struct{ studentpresence.SessionRecords }
	stubDataDeviationEvents struct {
		timetablePorts.DeviationEventReader
	}
	stubDataConflictAcks struct {
		timetable.ConflictAckCapability
	}
)

// stubParticipantRows are participant rows whose parts panic when reached.
func stubParticipantRows() timetablePorts.ParticipantRows {
	return timetablePorts.ParticipantRows{Plans: stubParticipantPlans{}, Presence: stubParticipantPresence{}, Reads: stubParticipantReads{}}
}

// unitTimetableData composes the Timetable owner's planner reads over the
// given fakes.
func unitTimetableData(deps unitDataDeps) timetable.TimetableDataCapability {
	composed := timetablePorts.TimetableDataDependencies{
		RunInTx:           func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
		Instances:         stubDataInstances{},
		InstanceStaff:     stubDataInstanceStaff{},
		Participants:      stubParticipantRows(),
		PickupExceptions:  stubDataPickupExceptions{},
		ArrivalExceptions: stubDataArrivalExceptions{},
		Visits:            stubDataVisits{},
		Templates:         stubDataTemplates{},
		Groups:            stubDataGroups{},
		BlockMetadata:     stubBlockMetadata{},
		Categories:        stubDataCategories{},
		Rooms:             stubDataRooms{},
		RoomOccupancy:     testRoomOccupancy{sessions: stubDataSessions{}},
		DeviationEvents:   stubDataDeviationEvents{},
		ConflictAcks:      stubDataConflictAcks{},
		Locks:             deps.Locks,
	}
	if deps.Instances != nil {
		composed.Instances = deps.Instances
	}
	if deps.Participants.Reads != nil {
		composed.Participants = deps.Participants
	}
	if deps.Templates != nil {
		composed.Templates = deps.Templates
	}
	if deps.Groups != nil {
		composed.Groups = deps.Groups
	}
	if deps.BlockMetadata != nil {
		composed.BlockMetadata = deps.BlockMetadata
	}
	if deps.Categories != nil {
		composed.Categories = deps.Categories
	}
	if deps.Rooms != nil {
		composed.Rooms = deps.Rooms
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
	ListTemplateEntriesForPeriodFn func(ctx context.Context, periodID *int64, childrenPerStaffRatio int, window *timetable.TemplateScheduleWindow) ([]timetable.TemplateListEntry, error)
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

func (f *fakeTimetableData) ListTemplateEntriesForPeriod(ctx context.Context, periodID *int64, childrenPerStaffRatio int, window *timetable.TemplateScheduleWindow) ([]timetable.TemplateListEntry, error) {
	if f.ListTemplateEntriesForPeriodFn == nil {
		return f.TimetableDataCapability.ListTemplateEntriesForPeriod(ctx, periodID, childrenPerStaffRatio, window)
	}
	return f.ListTemplateEntriesForPeriodFn(ctx, periodID, childrenPerStaffRatio, window)
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
