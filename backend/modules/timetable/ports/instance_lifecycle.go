package ports

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// LifecyclePresence is the Student Presence slice the transitions read and
// move live state through.
type LifecyclePresence interface {
	ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	QueryGroupSupervisions(ctx context.Context, filter studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
	TransferOpenVisits(ctx context.Context, fromGroupID, toGroupID int64) (int64, error)
	// EndGroup releases an absorbed unsupervised group after its visits
	// moved to the started block's group (#2697).
	EndGroup(ctx context.Context, groupID int64, endedAt time.Time) error
	CountOpenVisitsInRoom(ctx context.Context, roomID int64) (int, error)
}

// SessionEnder ends the Student Presence session of a block: visits and
// supervisors close and the checkout events fire.
type SessionEnder interface {
	EndActivitySession(ctx context.Context, activeGroupID int64) error
}

// LifecycleRooms is the consumer-owned port to the Facilities rooms the
// lifecycle validates, locks and names.
type LifecycleRooms interface {
	// RoomName names one room; ok is false when it does not exist. A
	// missing room may also surface as a not-found error.
	RoomName(ctx context.Context, id int64) (name string, ok bool, err error)
	// RoomNamesByID names the given rooms; a missing room is absent.
	RoomNamesByID(ctx context.Context, ids []int64) (map[int64]string, error)
	// LockRoom takes the room's row lock. ok is false when the room does
	// not exist; a nil capacity is unlimited.
	LockRoom(ctx context.Context, id int64) (capacity *int, ok bool, err error)
}

// CareDayLocks takes Care Plan's student → care-day lock a roster rewrite
// holds before it touches attendance rows.
type CareDayLocks interface {
	LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error
}

// LifecycleSettings is the Settings Platform's clock policy of the
// transitions.
type LifecycleSettings interface {
	// StartLeadMinutes is timetable.start_lead_minutes.
	StartLeadMinutes(ctx context.Context) (int, error)
	// EnforcePlannedEnd is timetable.enforce_planned_end.
	EnforcePlannedEnd(ctx context.Context) (bool, error)
	// CompleteLeadMinutes is timetable.complete_lead_minutes.
	CompleteLeadMinutes(ctx context.Context) (int, error)
}

// LifecycleStaff are the staff rows the instance lifecycle resolves the
// block's staff and the creator through.
type LifecycleStaff interface {
	FindByID(ctx context.Context, id any) (*timetable.StaffMember, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error)
	FindWithPerson(ctx context.Context, id int64) (*timetable.StaffMember, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error)
}

// LifecycleStudents reads and locks the children the instance lifecycle
// checks and announces.
type LifecycleStudents interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error)
	FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error)
	FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error)
}

// InstanceLifecycleDependencies wires the lifecycle. Broadcaster and
// GuardianNotices are optional (nil announces nothing and refuses every
// notice); Settings is required with EnforceTimePolicy; Logger and Now
// default. Every other field is required.
type InstanceLifecycleDependencies struct {
	InstanceRepo        ActivityInstanceRecords
	IdempotencyRepo     InstanceIdempotencyRecords
	InstanceStaffRepo   InstanceStaffRecords
	InstanceStudents    InstanceStudentRecords
	ExceptionRepo       ActivityExceptionRecords
	CalendarPeriodRepo  CalendarPeriodRecords
	RecoveryRepo        ActivityRecoveryRecords
	ActivityGroupRepo   LifecycleActivityGroups
	StaffRepo           LifecycleStaff
	StudentRepo         LifecycleStudents
	ActiveGroupRepo     studentpresence.SessionRecords
	SupervisorRepo      studentpresence.SupervisionRecords
	Presence            LifecyclePresence
	ActiveService       SessionEnder
	Rooms               LifecycleRooms
	CareDays            CareDays
	CareDayLocks        CareDayLocks
	Protocol            DeviationProtocol
	GuardianNotices     GuardianNotices
	Broadcaster         LifecycleBroadcaster
	Settings            LifecycleSettings
	Materialization     timetable.MaterializationCapability
	RecurrenceLock      timetable.RecurrenceWriteLock
	StartConflicts      timetable.StartConflictQuery
	SubstituteConflicts timetable.SubstituteConflictQuery
	// ContentHash is Security Runtime's content fingerprint (hex SHA-256)
	// of an idempotent create request.
	ContentHash       func([]byte) string
	Database          TenantDatabase
	Logger            *slog.Logger
	Now               func() time.Time
	EnforceTimePolicy bool
}

// LifecycleActivityGroups resolves the templates whose sessions are being started.
type LifecycleActivityGroups interface {
	FindByID(context.Context, int64) (*activityContract.Group, error)
	FindByIDs(context.Context, []int64) ([]*activityContract.Group, error)
}
