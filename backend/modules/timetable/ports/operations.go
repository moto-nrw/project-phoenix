package ports

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OperationParticipants reads and patches the participant rows of blocks.
type OperationParticipants interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error)
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
	FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error)
	FindPresentInOtherActiveInstances(ctx context.Context, excludeInstanceID int64, date calendar.Date, studentIDs []int64) ([]scheduleModels.ParallelPresence, error)
	UpdateAttendanceFields(ctx context.Context, id int64, patch scheduleModels.AttendanceFieldPatch) error
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
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error)
}

// OperationPeople resolves persons and their staff profiles.
type OperationPeople interface {
	FindByAccountID(ctx context.Context, accountID int64) (*timetable.Person, error)
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.Person, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*timetable.StaffMember, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error)
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

// ScopedAction is an operational action with its own school scope setting
// (#3180, #3622).
type ScopedAction int

const (
	// ScopedAttendance is operations.attendance_edit_scope.
	ScopedAttendance ScopedAction = iota + 1
	// ScopedBlockStart is operations.block_start_scope.
	ScopedBlockStart
	// ScopedBlockComplete is operations.block_complete_scope.
	ScopedBlockComplete
)

// OperationSettings is the consumer-owned port to the Settings Platform
// policies of the operational day. ResolveString serves the shared
// operational-overview gate and the school-wide action scope rule, which
// OperationCaller evaluates.
type OperationSettings interface {
	ResolveString(ctx context.Context, key string) (string, error)
	// StartLeadMinutes is timetable.start_lead_minutes.
	StartLeadMinutes(ctx context.Context) (int, error)
	// EnforcePlannedEnd is timetable.enforce_planned_end.
	EnforcePlannedEnd(ctx context.Context) (bool, error)
	// CompleteLeadMinutes is timetable.complete_lead_minutes.
	CompleteLeadMinutes(ctx context.Context) (int, error)
	// ActionScopeKey names the action's scope setting.
	ActionScopeKey(action ScopedAction) (string, error)
	// StudentAbsenceEditAllStaff reports operations.student_absence_edit_scope
	// = all_staff.
	StudentAbsenceEditAllStaff(ctx context.Context) (bool, error)
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
	EffectiveArrivals(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*ExpectedArrival, error)
}

// EffectivePickups is the consumer-owned port to Care Plan's effective
// pickup times of a date; a child without a time is absent or nil.
type EffectivePickups interface {
	EffectivePickups(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]*time.Time, error)
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

// RequestSession reads the verified session of the request the operational
// day authorizes. The root binds it over Identity's session token adapter.
type RequestSession interface {
	// Claims are the verified claims of the request, the zero claims
	// without any.
	Claims(ctx context.Context) RequestClaims
	// Permissions are the permissions of the request, none without any.
	Permissions(ctx context.Context) []string
}

// RequestClaims are the session claims the operational day reads.
type RequestClaims struct {
	AccountID   int64
	TenantID    int64
	Scope       string
	SchoolScope bool
	Admin       bool
}

// OperationDependencies wires the operational day. PlanningTracks and
// Locks are optional: without tracks day-scope blocks carry no colour,
// without locks attendance patches skip the completion lock. Announcer is
// optional too. Every other collaborator is required. BlockMetadata names
// the room and planning track of the planned blocks (#4413).
type OperationDependencies struct {
	Instances            LegacyInstanceReads
	InstanceStaff        timetable.InstanceStaffQuery
	Participants         OperationParticipants
	Templates            timetable.ActivityGroupQuery
	PlanningTracks       timetable.PlanningTrackQuery
	Sessions             OperationSessions
	Supervisions         OperationSupervisions
	Visits               OperationVisits
	Attendance           OperationAttendance
	Students             OperationStudents
	People               OperationPeople
	EducationGroups      EducationGroupNames
	Rooms                RoomNames
	BlockMetadata        BlockDisplayMetadata
	Settings             OperationSettings
	CareDays             CareDays
	Arrivals             EffectiveArrivals
	Pickups              EffectivePickups
	Lifecycle            OperationLifecycle
	Session              RequestSession
	Locks                AttendanceLocks
	Announcer            AttendanceAnnouncer
	NormalizeSchoolClass func(string) string
	Logger               *slog.Logger
	Now                  func() time.Time
}
