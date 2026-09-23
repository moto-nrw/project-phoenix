package compose

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// The operational day of the Timetable owner (#3424 slice S5): the
// "Heute geplant" views, the supervision rosters and the lifecycle actions
// staff take on a running block, behind timetable.OperationCapability. The
// reads use the retained repository rows, whose status carries the Student
// Presence session a block runs in. Collaborators the owner may not name
// arrive as consumer-owned ports the composition root binds: Care Plan's
// care days and effective times, the Settings Platform's policies, School
// Structure's groups and class names, Facilities' rooms, the realtime
// broadcaster, the staff attribution of the presence writes and the
// instance lifecycle that still lives in the retained planning services.

// OperationInstances reads the blocks of the retained activity instance
// repository.
type OperationInstances interface {
	FindByID(ctx context.Context, id any) (*scheduleModels.ActivityInstance, error)
	FindByTenantAndDate(ctx context.Context, date scheduleModels.Date) ([]*scheduleModels.ActivityInstance, error)
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) (*scheduleModels.ActivityInstance, error)
}

// OperationInstanceStaff reads the staff rows of blocks.
type OperationInstanceStaff interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStaff, error)
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStaff, error)
}

// OperationParticipants reads and patches the participant rows of blocks.
type OperationParticipants interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error)
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
	FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error)
	FindPresentInOtherActiveInstances(ctx context.Context, excludeInstanceID int64, date scheduleModels.Date, studentIDs []int64) ([]scheduleModels.ParallelPresence, error)
	UpdateAttendanceFields(ctx context.Context, id int64, patch scheduleModels.AttendanceFieldPatch) error
}

// OperationTemplates reads the templates (activity groups) behind blocks.
type OperationTemplates interface {
	FindByID(ctx context.Context, id any) (*activitiesModels.Group, error)
	FindByIDs(ctx context.Context, ids []int64) ([]*activitiesModels.Group, error)
	FindTargetsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]*activitiesModels.GroupTarget, error)
}

// OperationSessions reads and touches the Student Presence live sessions.
type OperationSessions interface {
	FindSession(ctx context.Context, id int64) (*studentpresence.LiveGroup, error)
	UpdateLastActivity(ctx context.Context, id int64, at time.Time) error
}

// OperationSupervisions reads who supervises a live session.
type OperationSupervisions interface {
	FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*studentpresence.StaffedSupervision, error)
}

// OperationVisits reads the Student Presence visits.
type OperationVisits interface {
	ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}

// OperationAttendance writes the visits a check-in or check-out records,
// attributed to the acting staff member.
type OperationAttendance interface {
	CreateVisitAs(ctx context.Context, staffID int64, visit *studentpresence.Visit) error
	EndVisitAs(ctx context.Context, staffID, tenantID, visitID int64) error
	MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error)
}

// OperationStudents reads the People Directory child rows of a roster.
type OperationStudents interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Student, error)
}

// OperationPeople resolves persons and their staff profiles.
type OperationPeople interface {
	FindByAccountID(ctx context.Context, accountID int64) (*usersModels.Person, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Person, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*usersModels.Staff, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Staff, error)
}

// EducationGroupNames is the consumer-owned port to School Structure's
// education groups: the names of the given groups, missing ones absent.
type EducationGroupNames interface {
	EducationGroupNames(ctx context.Context, ids []int64) (map[int64]string, error)
}

// RoomNames is the consumer-owned port to the Facilities rooms.
type RoomNames interface {
	// AllRoomNames names every room of the tenant.
	AllRoomNames(ctx context.Context) (map[int64]string, error)
	// RoomName names one room; ok is false when it does not exist.
	RoomName(ctx context.Context, id int64) (name string, ok bool, err error)
}

// AttendanceEditScope is the tenant's operations.attendance_edit_scope.
type AttendanceEditScope int

const (
	// AttendanceEditUnset grants nothing beyond the block's own staff.
	AttendanceEditUnset AttendanceEditScope = iota
	// AttendanceEditOwn keeps attendance edits with the block's operators.
	AttendanceEditOwn
	// AttendanceEditAllStaff opens them to every verified OGS staff member.
	AttendanceEditAllStaff
)

// OperationSettings is the consumer-owned port to the Settings Platform
// policies of the operational day. ResolveString serves the shared
// operational-overview gate (auth/authorize).
type OperationSettings interface {
	authorize.OverviewSettingsResolver
	// StartLeadMinutes is timetable.start_lead_minutes.
	StartLeadMinutes(ctx context.Context) (int, error)
	// EnforcePlannedEnd is timetable.enforce_planned_end.
	EnforcePlannedEnd(ctx context.Context) (bool, error)
	AttendanceEditScope(ctx context.Context) (AttendanceEditScope, error)
	// StudentAbsenceEditAllStaff reports operations.student_absence_edit_scope
	// = all_staff.
	StudentAbsenceEditAllStaff(ctx context.Context) (bool, error)
	// OperationalOverviewAllStaff reports operations.operational_overview_scope
	// = all_staff.
	OperationalOverviewAllStaff(ctx context.Context) (bool, error)
}

// ClassArrivalNotice is a class-wide day exception behind an arrival time
// (#2962).
type ClassArrivalNotice struct {
	ArrivalTime string
	Label       string
}

// ExpectedArrival is Care Plan's effective arrival of one child on a date.
type ExpectedArrival struct {
	ArrivalTime    *time.Time
	IsException    bool
	ClassException *ClassArrivalNotice
}

// EffectiveArrivals is the consumer-owned port to Care Plan's effective
// arrival times of a date.
type EffectiveArrivals interface {
	EffectiveArrivals(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*ExpectedArrival, error)
}

// EffectivePickups is the consumer-owned port to Care Plan's effective
// pickup times of a date; a child without a time is absent or nil.
type EffectivePickups interface {
	EffectivePickups(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*time.Time, error)
}

// AttendanceAnnouncer wakes the tenant's clients after an attendance patch;
// the event carries no child (#2085).
type AttendanceAnnouncer interface {
	AnnounceAttendanceChanged(tenantID, activeGroupID, instanceID int64) error
}

// OperationLifecycle drives the lifecycle transitions of the retained
// instance service (#3424 slice S1) for the operational commands.
type OperationLifecycle interface {
	CreateSpontaneous(ctx context.Context, in timetable.SpontaneousStart) (instanceID int64, err error)
	// Start starts the block for the staff member; spontaneous applies the
	// weekday guard of an ad-hoc start.
	Start(ctx context.Context, instanceID, staffID int64, spontaneous bool) (*timetable.StartedOperation, error)
	Complete(ctx context.Context, instanceID, accountID int64) (*timetable.ScheduledInstance, error)
	Reopen(ctx context.Context, instanceID, accountID int64, isAdmin bool) (*timetable.StartedOperation, error)
}

// AttendanceLocks serializes attendance writes with the completion of the
// block.
type AttendanceLocks interface {
	LockAttendance(ctx context.Context, instanceID int64) error
}

// OperationDependencies wires the operational day. PlanningTracks and
// Locks are optional: without tracks day-scope blocks carry no colour,
// without locks attendance patches skip the completion lock. Announcer is
// optional too. Every other collaborator is required.
type OperationDependencies struct {
	Instances            OperationInstances
	InstanceStaff        OperationInstanceStaff
	Participants         OperationParticipants
	Templates            OperationTemplates
	PlanningTracks       timetable.PlanningTrackQuery
	Sessions             OperationSessions
	Supervisions         OperationSupervisions
	Visits               OperationVisits
	Attendance           OperationAttendance
	Students             OperationStudents
	People               OperationPeople
	EducationGroups      EducationGroupNames
	Rooms                RoomNames
	Settings             OperationSettings
	CareDays             CareDays
	Arrivals             EffectiveArrivals
	Pickups              EffectivePickups
	Lifecycle            OperationLifecycle
	Locks                AttendanceLocks
	Announcer            AttendanceAnnouncer
	NormalizeSchoolClass func(string) string
	Logger               *slog.Logger
	Now                  func() time.Time
}

type operations struct {
	deps OperationDependencies
}

// NewOperations composes the Timetable owner's operational day.
func NewOperations(deps OperationDependencies) (timetable.OperationCapability, error) {
	if deps.Instances == nil || deps.InstanceStaff == nil || deps.Participants == nil || deps.Templates == nil ||
		deps.Sessions == nil || deps.Supervisions == nil || deps.Visits == nil || deps.Attendance == nil ||
		deps.Students == nil || deps.People == nil || deps.EducationGroups == nil || deps.Rooms == nil ||
		deps.Settings == nil || deps.CareDays == nil || deps.Arrivals == nil || deps.Pickups == nil ||
		deps.Lifecycle == nil || deps.NormalizeSchoolClass == nil {
		return nil, errors.New("timetable operations: required dependency is nil")
	}
	return &operations{deps: deps}, nil
}

func (s *operations) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func (s *operations) today() timezone.Date {
	return timezone.DateFromTime(s.now())
}

func (s *operations) logger() *slog.Logger {
	if s.deps.Logger != nil {
		return s.deps.Logger
	}
	return slog.Default()
}

func (s *operations) loadInstance(ctx context.Context, instanceID int64) (*scheduleModels.ActivityInstance, error) {
	inst, err := s.deps.Instances.FindByID(ctx, instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, timetable.ErrTimetableOperationNotFound
		}
		return nil, err
	}
	if inst == nil {
		return nil, timetable.ErrTimetableOperationNotFound
	}
	return inst, nil
}

// resolveStaffID resolves the caller's staff profile; a caller without one
// has hasStaff false.
func (s *operations) resolveStaffID(ctx context.Context, accountID int64) (int64, bool, error) {
	if accountID <= 0 {
		return 0, false, timetable.ErrTimetableOperationForbidden
	}
	person, err := s.deps.People.FindByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, usersSvc.ErrPersonNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if person == nil {
		return 0, false, nil
	}
	staff, err := s.deps.People.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if staff == nil {
		return 0, false, nil
	}
	return staff.ID, true, nil
}

// lifecycleWindow is the planned window of a retained block.
func lifecycleWindow(inst *scheduleModels.ActivityInstance) timetable.LifecycleWindow {
	return timetable.LifecycleWindow{
		Date: timezone.Date(inst.Date), StartTime: inst.StartTime, EndTime: inst.EndTime, IsSpontaneous: inst.IsSpontaneous,
	}
}

func retainedInstanceIDs(instances []*scheduleModels.ActivityInstance) []int64 {
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	return ids
}

func indexInstanceStaffRows(rows []*scheduleModels.InstanceStaff) map[int64][]*scheduleModels.InstanceStaff {
	byInstance := make(map[int64][]*scheduleModels.InstanceStaff)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}

func indexInstanceStudentRows(rows []*scheduleModels.InstanceStudent) map[int64][]*scheduleModels.InstanceStudent {
	byInstance := make(map[int64][]*scheduleModels.InstanceStudent)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}

func staffAssigned(rows []*scheduleModels.InstanceStaff, staffID int64) bool {
	for _, row := range rows {
		if row.StaffID == staffID && !row.IsAbsent {
			return true
		}
	}
	return false
}

func findPlanned(rows []*scheduleModels.InstanceStudent, studentID int64) (*scheduleModels.InstanceStudent, bool) {
	for _, row := range rows {
		if row.StudentID == studentID {
			return row, true
		}
	}
	return nil, false
}
