package active

import (
	"context"
	"time"

	domain "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

type Group = domain.Group
type Visit = domain.Visit
type GroupSupervisor = domain.GroupSupervisor
type CombinedGroup = domain.CombinedGroup
type GroupMapping = domain.GroupMapping
type VisitWithDisplayData = domain.VisitWithDisplayData

// GroupReadRepository defines read-only operations for active groups
type GroupReadRepository interface {
	// FindByID retrieves an active group by its ID
	FindByID(ctx context.Context, id interface{}) (*Group, error)

	// List retrieves active groups matching the provided filters
	List(ctx context.Context, options *base.QueryOptions) ([]*Group, error)

	// FindActiveByRoomID finds all active groups in a specific room
	FindActiveByRoomID(ctx context.Context, roomID int64) ([]*Group, error)

	// FindActiveByGroupID finds all active instances of a specific activity group
	FindActiveByGroupID(ctx context.Context, groupID int64) ([]*Group, error)

	// FindByTimeRange finds all groups active during a specific time range
	FindByTimeRange(ctx context.Context, start, end time.Time) ([]*Group, error)

	// FindBySourceIDs finds active groups based on source IDs and source type
	FindBySourceIDs(ctx context.Context, sourceIDs []int64, sourceType string) ([]*Group, error)

	// Activity session conflict detection methods
	FindActiveByDeviceID(ctx context.Context, deviceID int64) (*Group, error)
	FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*Group, error)

	// Room conflict detection methods
	CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *Group, error)

	// Session timeout methods
	FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*Group, error)
	FindInactiveSessions(ctx context.Context, inactiveDuration time.Duration) ([]*Group, error)

	// Unclaimed groups (for frontend claiming feature)
	FindUnclaimed(ctx context.Context) ([]*Group, error)

	// FindActiveGroups finds all groups with no end time (currently active)
	FindActiveGroups(ctx context.Context) ([]*Group, error)

	// FindByIDs finds active groups by their IDs
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Group, error)
}

// GroupWriteRepository defines write operations for active groups
type GroupWriteRepository interface {
	// Create inserts a new active group
	Create(ctx context.Context, group *Group) error

	// Update updates an existing active group
	Update(ctx context.Context, group *Group) error

	// Delete removes an active group
	Delete(ctx context.Context, id interface{}) error

	// EndSession marks a group session as ended at the current time
	EndSession(ctx context.Context, id int64) error

	// UpdateLastActivity updates the last activity timestamp for a session
	UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error
}

// GroupRelationsRepository defines relation-loading operations for active groups
type GroupRelationsRepository interface {
	// FindWithRelations retrieves a group with its associated relations
	FindWithRelations(ctx context.Context, id int64) (*Group, error)

	// FindWithVisits retrieves a group with its associated visits
	FindWithVisits(ctx context.Context, id int64) (*Group, error)

	// FindWithSupervisors retrieves a group with its supervisors
	FindWithSupervisors(ctx context.Context, id int64) (*Group, error)
}

// VisitRepository defines operations for managing active visits
type VisitRepository interface {
	base.Repository[*Visit]

	// FindActiveByStudentID finds all active visits for a specific student
	FindActiveByStudentID(ctx context.Context, studentID int64) ([]*Visit, error)

	// FindByActiveGroupID finds all visits for a specific active group
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*Visit, error)

	// FindByTimeRange finds all visits active during a specific time range
	FindByTimeRange(ctx context.Context, start, end time.Time) ([]*Visit, error)

	// EndVisit marks a visit as ended at the current time
	EndVisit(ctx context.Context, id int64) error

	// TransferVisitsFromRecentSessions transfers active visits from recent ended sessions on the same device to a new session
	TransferVisitsFromRecentSessions(ctx context.Context, newActiveGroupID, deviceID int64) (int, error)

	// Cleanup operations for data retention
	// DeleteExpiredVisits deletes visits older than retention days for a specific student
	DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error)

	// DeleteVisitsBeforeDate deletes visits created before a specific date for a student
	DeleteVisitsBeforeDate(ctx context.Context, studentID int64, beforeDate time.Time) (int64, error)

	// GetVisitRetentionStats gets statistics about visits that are candidates for deletion
	GetVisitRetentionStats(ctx context.Context) (map[int64]int, error)

	// CountExpiredVisits counts visits that are older than retention period for all students
	CountExpiredVisits(ctx context.Context) (int64, error)

	// GetOldestExpiredVisit returns the timestamp of the oldest visit that is past retention
	GetOldestExpiredVisit(ctx context.Context) (*time.Time, error)

	// GetExpiredVisitsByMonth returns counts of expired visits grouped by month
	GetExpiredVisitsByMonth(ctx context.Context) (map[string]int64, error)

	// GetCurrentByStudentID finds the current active visit for a student
	GetCurrentByStudentID(ctx context.Context, studentID int64) (*Visit, error)

	// GetCurrentByStudentIDs finds the current active visit for multiple students
	GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*Visit, error)

	// FindActiveVisits finds all visits with no exit time (currently active)
	FindActiveVisits(ctx context.Context) ([]*Visit, error)

	// FindActiveByGroupIDWithDisplayData finds active visits for a group with student display info
	FindActiveByGroupIDWithDisplayData(ctx context.Context, activeGroupID int64) ([]VisitWithDisplayData, error)
}

// GroupSupervisorRepository defines operations for managing active group supervisors
type GroupSupervisorRepository interface {
	base.Repository[*GroupSupervisor]

	// FindActiveByStaffID finds all active supervisions for a specific staff member
	FindActiveByStaffID(ctx context.Context, staffID int64) ([]*GroupSupervisor, error)

	// FindByActiveGroupID finds supervisors for a specific active group
	// If activeOnly is true, only returns supervisors with end_date IS NULL (currently active)
	FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*GroupSupervisor, error)

	// FindByActiveGroupIDs finds supervisors for multiple active groups in a single query
	// If activeOnly is true, only returns supervisors with end_date IS NULL (currently active)
	FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*GroupSupervisor, error)

	// EndSupervision marks a supervision as ended at the current date
	EndSupervision(ctx context.Context, id int64) error
}

// CombinedGroupRepository defines operations for managing active combined groups
type CombinedGroupRepository interface {
	base.Repository[*CombinedGroup]

	// FindActive finds all currently active combined groups
	FindActive(ctx context.Context) ([]*CombinedGroup, error)

	// FindByTimeRange finds all combined groups active during a specific time range
	FindByTimeRange(ctx context.Context, start, end time.Time) ([]*CombinedGroup, error)

	// EndCombination marks a combined group as ended at the current time
	EndCombination(ctx context.Context, id int64) error

	// FindWithGroups finds a combined group with all its associated active groups
	FindWithGroups(ctx context.Context, id int64) (*CombinedGroup, error)
}

// GroupMappingRepository defines operations for managing active group mappings
type GroupMappingRepository interface {
	base.Repository[*GroupMapping]

	// FindByActiveCombinedGroupID finds all mappings for a specific combined group
	FindByActiveCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*GroupMapping, error)

	// FindByActiveGroupID finds all mappings for a specific active group
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*GroupMapping, error)

	// AddGroupToCombination adds an active group to a combined group
	AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error

	// RemoveGroupFromCombination removes an active group from a combined group
	RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error

	// FindWithRelations retrieves a mapping with its associated CombinedGroup and ActiveGroup relations
	FindWithRelations(ctx context.Context, id int64) (*GroupMapping, error)
}
