package ports

import (
	"context"
	"log/slog"
)

type settingsResolver interface {
	PickupUpcomingEnabled(context.Context) (bool, error)
	PickupOverdueEnabled(context.Context) (bool, error)
	ActivityStartEnabled(context.Context) (bool, error)
	ActivityOverdueEnabled(context.Context) (bool, error)
	PickupLeadMinutes(context.Context) (int, error)
	ActivityLeadMinutes(context.Context) (int, error)
	OverdueThresholdMinutes(context.Context) (int, error)
	BinaryPresence(context.Context) (bool, error)
}

type attendanceReader interface {
	ListOpenStudentIDsForDate(ctx context.Context, date string) ([]int64, error)
}

type pickupReader interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date string) (map[int64]*EffectivePickupTime, error)
}

type instanceReader interface {
	FindByTenantAndDate(ctx context.Context, date string) ([]*ActivityInstance, error)
}

type roomReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*Room, error)
}

type studentReader interface {
	// FindReadScopeByIDs returns only the id/group_id/person_id/school_class
	// projection this service needs — read-access gating plus name display. It
	// deliberately avoids the repository's full FindByIDs hydration (weekday
	// bus-day / departure jsonb + an information_schema probe), which would run on
	// every 60s header poll for the whole present population and buys this service
	// nothing.
	FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*Student, error)
}

type personReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Person, error)
}

type supervisionReader interface {
	GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*GroupSupervisor, error)
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*Group, error)
}

type roomPresenceReader interface {
	ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error)
}

// QueryDependencies supplies tenant-scoped reminder facts and settings.
// Clock returns a canonical YYYY-MM-DD date and minute of day from one Berlin instant.
type QueryDependencies struct {
	CurrentStaff func(context.Context) (int64, error)
	Clock        func() (date string, minute int)
	Settings     settingsResolver
	Attendance   attendanceReader
	Pickup       pickupReader
	Instance     instanceReader
	Room         roomReader
	Student      studentReader
	Person       personReader
	Supervision  supervisionReader
	Visits       roomPresenceReader
	Logger       *slog.Logger

	// The bulk readers below are used only by ComputeBatch. They are optional so
	// the query can run without bulk evaluation; a nil one fails
	// closed (empty room set or nobody readable), never open.
	BulkSupervision bulkSupervisionReader
	// BulkInstanceStaff resolves the planned staff of today's activity
	// instances. Without it the batch falls back to room supervision alone,
	// which never reaches the person who is supposed to START a slot.
	BulkInstanceStaff bulkInstanceStaffReader
}

// bulkInstanceStaffReader answers who is planned on which activity instance.
type bulkInstanceStaffReader interface {
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*InstanceStaff, error)
}

// bulkSupervisionReader answers "who supervises which room right now" for the
// whole tenant in one query.
type bulkSupervisionReader interface {
	ListActiveSupervisedRooms(ctx context.Context) ([]StaffRoomSupervision, error)
}
