package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// SupervisionBlocker is an open supervision projected for capability checks.
type SupervisionBlocker struct {
	ID        int64
	GroupID   int64
	GroupName string
	StartDate string
}

// GroupRepository defines operations for managing active groups
type GroupRepository interface {
	Create(context.Context, *Group) error
	FindByID(context.Context, int64) (*Group, error)
	Update(context.Context, *Group) error
	Delete(context.Context, int64) error

	// FindActiveByRoomID finds all active groups in a specific room
	FindActiveByRoomID(ctx context.Context, roomID int64) ([]*Group, error)

	// FindActiveByRoomIDAndDeviceID finds the active group in a room that belongs to a specific device.
	FindActiveByRoomIDAndDeviceID(ctx context.Context, roomID int64, deviceID int64) (*Group, error)

	// FindActiveByGroupID finds all active instances of a specific activity group
	FindActiveByGroupID(ctx context.Context, groupID int64) ([]*Group, error)

	// FindActiveByGroupIDs finds all active groups for multiple group IDs in a single query
	FindActiveByGroupIDs(ctx context.Context, groupIDs []int64) ([]*Group, error)

	// FindByTimeRange finds all groups active during a specific time range
	FindByTimeRange(ctx context.Context, start, end time.Time) ([]*Group, error)

	FindWithSupervisors(ctx context.Context, id int64) (*Group, error)

	FindActiveByDeviceID(ctx context.Context, deviceID int64) (*Group, error)
	FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*Group, error)

	// Room conflict detection methods
	CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *Group, error)

	// Session timeout methods
	UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error
	FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*Group, error)
	// Unclaimed groups (for frontend claiming feature)

	// FindActiveGroups finds all groups with no end time (currently active)
	FindActiveGroups(ctx context.Context) ([]*Group, error)

	// FindByIDs finds active groups by their IDs
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Group, error)

	// FindByIDForUpdate finds an active group by ID and locks the row for
	// the current transaction.
	FindByIDForUpdate(ctx context.Context, id int64) (*Group, error)

	// GetOccupiedActivityGroupIDs returns a set of activity group IDs that currently have active sessions
	GetOccupiedActivityGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]bool, error)
}

// GroupSupervisorRepository defines operations for managing active group supervisors.
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
	// It answers in a single query what FindActiveByStaffID plus
	// GetActiveGroupsByIDs answer per staff member, so a caller that needs the
	// whole tenant does not pay a per-person round trip.
	ListActiveSupervisedRooms(ctx context.Context) ([]StaffRoomSupervision, error)

	// FindByActiveGroupID finds supervisors for a specific active group
	// If activeOnly is true, only returns supervisors whose date interval includes today.
	FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*GroupSupervisor, error)
	// FindByActiveGroupIDForUpdate locks a group's current supervision rows for
	// the lifetime of the transaction.
	FindByActiveGroupIDForUpdate(ctx context.Context, activeGroupID int64) ([]*GroupSupervisor, error)

	// FindByActiveGroupIDs finds supervisors for multiple active groups in a single query
	// If activeOnly is true, only returns supervisors with end_date IS NULL (currently active)
	FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*GroupSupervisor, error)

	// EndSupervision marks a supervision as ended at the current date
	EndSupervision(ctx context.Context, id int64) error

	// GetStaffIDsWithSupervisionToday returns staff IDs who had any supervision activity today
	GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error)

	// EndAllActiveByStaffID ends all active supervisions for a staff member (sets end_date = CURRENT_DATE)
	// Returns the number of supervisions that were ended
	EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error)

	// EndByActiveGroupAndStaffID ends active supervisions for a specific
	// (active_group_id, staff_id) pair. Sets end_date=now() on all matching
	// rows with end_date IS NULL. Used by the substitute flow to remove the
	// absent staff's active supervisorship for a single active group without
	// touching their supervisions elsewhere. Idempotent: zero rows matched
	// is not an error (staff already ended or never supervised this group).
	EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error)

	// FindStaleOpen returns supervisor rows started before the given day that
	// still lack an end_date. Feeds the nightly stale-supervisor cleanup and
	// its preview.
	FindStaleOpen(ctx context.Context, before timezone.Date) ([]*GroupSupervisor, error)

	// SetEndDate updates only end_date and updated_at, returning the number of matched rows.
	SetEndDate(ctx context.Context, supervisor *GroupSupervisor) (int64, error)
}
