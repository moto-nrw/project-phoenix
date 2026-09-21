package ports

import (
	"context"
	"time"
)

// ActiveGroupRepository reads and writes room sessions with the display
// relations the presence services need.
type ActiveGroupRepository interface {
	Create(context.Context, *ActiveGroup) error
	FindByID(context.Context, int64) (*ActiveGroup, error)
	Update(context.Context, *ActiveGroup) error
	Delete(context.Context, int64) error

	// FindActiveByRoomID finds all active groups in a specific room
	FindActiveByRoomID(ctx context.Context, roomID int64) ([]*ActiveGroup, error)

	// FindActiveByRoomIDAndDeviceID finds the active group in a room that belongs to a specific device.
	FindActiveByRoomIDAndDeviceID(ctx context.Context, roomID int64, deviceID int64) (*ActiveGroup, error)

	// FindActiveByGroupID finds all active instances of a specific activity group
	FindActiveByGroupID(ctx context.Context, groupID int64) ([]*ActiveGroup, error)

	// FindActiveByGroupIDs finds all active groups for multiple group IDs in a single query
	FindActiveByGroupIDs(ctx context.Context, groupIDs []int64) ([]*ActiveGroup, error)

	// FindByTimeRange finds all groups active during a specific time range
	FindByTimeRange(ctx context.Context, start, end time.Time) ([]*ActiveGroup, error)

	FindWithSupervisors(ctx context.Context, id int64) (*ActiveGroup, error)

	FindActiveByDeviceID(ctx context.Context, deviceID int64) (*ActiveGroup, error)
	FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*ActiveGroup, error)

	// CheckRoomConflict reports another open group occupying the room.
	CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *ActiveGroup, error)

	// Session timeout methods
	UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error
	FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*ActiveGroup, error)

	// FindActiveGroups finds all groups with no end time (currently active)
	FindActiveGroups(ctx context.Context) ([]*ActiveGroup, error)

	// FindByIDs finds active groups by their IDs
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*ActiveGroup, error)

	// FindByIDForUpdate finds an active group by ID and locks the row for
	// the current transaction.
	FindByIDForUpdate(ctx context.Context, id int64) (*ActiveGroup, error)

	// GetOccupiedActivityGroupIDs returns a set of activity group IDs that currently have active sessions
	GetOccupiedActivityGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]bool, error)
}

// GroupSupervisorRepository reads and writes the supervision of room sessions.
type GroupSupervisorRepository interface {
	Create(context.Context, *GroupSupervisor) error
	FindByID(context.Context, int64) (*GroupSupervisor, error)
	Update(context.Context, *GroupSupervisor) error
	Delete(context.Context, int64) error

	// ListActiveSupervisionBlockers returns still-open supervisions as
	// caregiver-capability blocker rows.
	ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]SupervisionBlocker, error)

	// FindActiveByStaffID finds all active supervisions for a specific staff member
	FindActiveByStaffID(ctx context.Context, staffID int64) ([]*GroupSupervisor, error)
	// FindActiveByStaffIDForUpdate locks a staff member's current supervision
	// rows for the lifetime of the transaction.
	FindActiveByStaffIDForUpdate(ctx context.Context, staffID int64) ([]*GroupSupervisor, error)

	// ListActiveSupervisedRooms returns one (staff_id, room_id) pair per
	// currently supervised room in the tenant, for every staff member at once.
	ListActiveSupervisedRooms(ctx context.Context) ([]StaffRoomSupervision, error)

	// FindByActiveGroupID finds supervisors for a specific active group
	// If activeOnly is true, only returns supervisors whose date interval includes today.
	FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*GroupSupervisor, error)
	// FindByActiveGroupIDForUpdate locks a group's current supervision rows for
	// the lifetime of the transaction.
	FindByActiveGroupIDForUpdate(ctx context.Context, activeGroupID int64) ([]*GroupSupervisor, error)

	// FindByActiveGroupIDs finds supervisors for multiple active groups in a single query
	FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*GroupSupervisor, error)

	// EndSupervision marks a supervision as ended at the current date
	EndSupervision(ctx context.Context, id int64) error

	// GetStaffIDsWithSupervisionToday returns staff IDs who had any supervision activity today
	GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error)

	// EndAllActiveByStaffID ends all active supervisions for a staff member (sets end_date = CURRENT_DATE)
	// Returns the number of supervisions that were ended
	EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error)

	// EndByActiveGroupAndStaffID ends active supervisions for a specific
	// (active_group_id, staff_id) pair. Idempotent: zero rows matched is not
	// an error (staff already ended or never supervised this group).
	EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error)

	// FindStaleOpen returns supervisor rows started before the given day that
	// still lack an end_date. Feeds the nightly stale-supervisor cleanup and
	// its preview.
	FindStaleOpen(ctx context.Context, before Date) ([]*GroupSupervisor, error)

	// SetEndDate updates only end_date and updated_at, returning the number of matched rows.
	SetEndDate(ctx context.Context, supervisor *GroupSupervisor) (int64, error)
}

// ActiveGroupRecords is the owner's session record store the session
// repository reads and writes through.
type ActiveGroupRecords interface {
	QueryLiveGroups(context.Context, LiveGroupFilter) ([]LiveGroup, error)
	ListLiveGroups(context.Context, []int64) ([]LiveGroup, error)
	LockGroup(context.Context, int64) (LiveGroup, error)
	RecordGroup(context.Context, LiveGroup) (LiveGroup, error)
	ReviseGroup(context.Context, LiveGroup) (LiveGroup, error)
	DeleteGroup(context.Context, int64) error
	RecordGroupActivity(context.Context, int64, time.Time) error
	OccupiedActivityGroupIDs(context.Context, []int64) ([]int64, error)
	ListGroupSupervisions(context.Context, int64) ([]GroupSupervision, error)
}

// SupervisionRecords is the owner's supervision record store the supervision
// repository reads and writes through.
type SupervisionRecords interface {
	QueryGroupSupervisions(context.Context, GroupSupervisionFilter) ([]GroupSupervision, error)
	RecordSupervision(context.Context, GroupSupervision) (GroupSupervision, error)
	ReviseSupervision(context.Context, GroupSupervision) (GroupSupervision, error)
	RemoveSupervision(context.Context, int64) error
	SetSupervisionEnd(context.Context, int64, Date, time.Time) (int64, error)
	EndOpenGroupSupervisions(context.Context, int64, int64, Date) (int, error)
	EndStaffSupervisionsOn(context.Context, int64, Date) (int, error)
	EndSupervisionOn(context.Context, int64, Date) (int, error)
	SupervisedRoomsOn(context.Context, Date) ([]StaffRoomSupervision, error)
	StaffIDsWithSupervisionOn(context.Context, Date) ([]int64, error)
}

// RecordErrors keeps the retained repository error representation of the
// composition root. MissingRecordError must retain both the repository and
// the SQL not-found classification.
type RecordErrors interface {
	WrapRecordError(operation string, err error) error
	MissingRecordError(operation string) error
}

// DirectoryDevice is the Device Fleet projection the session reads resolve.
type DirectoryDevice struct {
	ID         int64
	TenantID   int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeviceID   string
	DeviceType string
	Name       *string
	Status     string
	LastSeen   *time.Time
}

// DeviceDirectory is the owner query session reads resolve devices through.
// There is no fallback join.
type DeviceDirectory interface {
	// ListDevicesByID returns the devices visible in the caller's
	// transaction. Missing IDs are absent, like the retired LEFT JOIN.
	ListDevicesByID(ctx context.Context, ids []int64) ([]DirectoryDevice, error)
}

// DirectoryRoom is the Facilities projection the session reads resolve.
type DirectoryRoom struct {
	ID         int64
	TenantID   int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Name       string
	Building   string
	Floor      *int
	Capacity   *int
	Category   *string
	Color      *string
	IsSystem   bool
	IsOpenRoom bool
}

// RoomDirectory is the owner query the session reads resolve rooms through.
// There is no fallback join.
type RoomDirectory interface {
	// ListRoomsByID returns the rooms visible in the caller's transaction.
	// Missing IDs are absent, like the former LEFT JOIN.
	ListRoomsByID(ctx context.Context, ids []int64) ([]DirectoryRoom, error)
}

// ActivityDirectory is the Timetable query the session reads resolve their
// activity templates through.
type ActivityDirectory interface {
	FindByIDs(context.Context, []int64) ([]*SessionActivity, error)
}

// SupervisionStaffDirectory resolves the staff members (with their person
// facts, once the People Directory is bound) behind supervision rows. Staff
// IDs the directory cannot resolve are absent from the result.
type SupervisionStaffDirectory interface {
	SupervisionStaff(ctx context.Context, staffIDs []int64) (map[int64]*SessionStaff, error)
}
