package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// GroupRepository defines operations for managing active groups
type GroupRepository interface {
	base.Repository[*Group]

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

	// EndSession marks a group session as ended at the current time
	EndSession(ctx context.Context, id int64) error

	// Relations methods
	FindWithRelations(ctx context.Context, id int64) (*Group, error)
	FindWithVisits(ctx context.Context, id int64) (*Group, error)
	FindWithSupervisors(ctx context.Context, id int64) (*Group, error)

	// Activity session conflict detection methods
	FindActiveByGroupIDWithDevice(ctx context.Context, groupID int64) ([]*Group, error)
	FindActiveByDeviceID(ctx context.Context, deviceID int64) (*Group, error)
	FindActiveByDeviceIDWithRelations(ctx context.Context, deviceID int64) (*Group, error)
	FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*Group, error)

	// Room conflict detection methods
	CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *Group, error)

	// Session timeout methods
	UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error
	FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*Group, error)
	FindInactiveSessions(ctx context.Context, inactiveDuration time.Duration) ([]*Group, error)

	// Unclaimed groups (for frontend claiming feature)
	FindUnclaimed(ctx context.Context) ([]*Group, error)

	// FindActiveGroups finds all groups with no end time (currently active)
	FindActiveGroups(ctx context.Context) ([]*Group, error)

	// FindByIDs finds active groups by their IDs
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Group, error)

	// GetOccupiedRoomIDs returns a set of room IDs that currently have active groups
	GetOccupiedRoomIDs(ctx context.Context, roomIDs []int64) (map[int64]bool, error)

	// GetOccupiedActivityGroupIDs returns a set of activity group IDs that currently have active sessions
	GetOccupiedActivityGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]bool, error)

	// EndSessionsByIDs ends multiple group sessions in a single query.
	// Returns the number of sessions ended.
	EndSessionsByIDs(ctx context.Context, ids []int64) (int64, error)

	// AggregateRoomSessions returns one row per active.groups session in the
	// given room that was active at any point during [start, end] — i.e.
	// start_time <= end AND (end_time IS NULL OR end_time >= start). Sessions
	// that began before `start` but were still occupying the room inside the
	// window are included. Each row carries the aggregated session view
	// (activity, supervisors, distinct student count) used by the
	// room-history endpoint. When supervisorStaffID is non-nil the result is
	// filtered to sessions supervised by that staff member; pass nil to see
	// every session (admin / all_staff scope).
	AggregateRoomSessions(ctx context.Context, roomID int64, start, end time.Time, supervisorStaffID *int64) ([]*RoomSessionAggregate, error)
}

// RoomSessionAggregate is one row in the per-room session timeline. It is
// intentionally aggregated — no individual student IDs or names leave the
// repo, only counts and the activity / supervisor metadata needed to render
// the room occupancy history.
type RoomSessionAggregate struct {
	SessionID       int64      `bun:"session_id"`
	StartedAt       time.Time  `bun:"started_at"`
	EndedAt         *time.Time `bun:"ended_at"`
	DurationMinutes *int       `bun:"duration_minutes"`
	ActivityName    string     `bun:"activity_name"`
	SupervisorName  string     `bun:"supervisor_name"`
	StudentCount    int        `bun:"student_count"`
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

	// FindByStudentAndTimeRange finds all visits (active or ended) for a specific
	// student whose entry_time falls within [start, end], ordered by entry_time desc.
	FindByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*Visit, error)

	// FindByStudentAndActiveGroupIDs returns visits for the student whose
	// active_group_id is in the given list. Used by the timetable per-student
	// day view to detect unplanned attendance in a single batch query.
	// Returns an empty slice (no DB call) when the id list is empty.
	FindByStudentAndActiveGroupIDs(ctx context.Context, studentID int64, activeGroupIDs []int64) ([]*Visit, error)

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

	// GetCurrentByStudentID finds the current active visit for a student
	GetCurrentByStudentID(ctx context.Context, studentID int64) (*Visit, error)

	// GetCurrentByStudentIDWithRoom finds the current active visit with its active group and room.
	GetCurrentByStudentIDWithRoom(ctx context.Context, studentID int64) (*Visit, error)

	// GetCurrentByStudentIDs finds the current active visit for multiple students
	GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*Visit, error)

	// CountActiveByRoomID counts currently active visits across all active groups in a room.
	CountActiveByRoomID(ctx context.Context, roomID int64) (int, error)

	// ListActiveStudentIDsByRoomID returns the IDs of students currently
	// checked-in to any active (end_time IS NULL) group in the given room.
	// Callers feed the IDs into the standard student list pipeline, which
	// owns display fields, GDPR redaction, and pagination. Tenant scoping
	// flows through TenantTxMiddleware.
	ListActiveStudentIDsByRoomID(ctx context.Context, roomID int64) ([]int64, error)

	// CountActiveByGroupID counts currently active visits in a single active group.
	CountActiveByGroupID(ctx context.Context, activeGroupID int64) (int, error)

	// CountActiveByStudentID counts active visits for a student.
	CountActiveByStudentID(ctx context.Context, studentID int64) (int, error)

	// FindActiveVisits finds all visits with no exit time (currently active)
	FindActiveVisits(ctx context.Context) ([]*Visit, error)

	// EndVisitsByActiveGroupIDs ends all active visits for multiple group IDs in a single query.
	// Returns the number of visits ended.
	EndVisitsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) (int64, error)

	// GetTodayVisitNamesForStudents returns activity group + room names for all of
	// today's visits for the given students. Used for tracking indicator matching.
	GetTodayVisitNamesForStudents(ctx context.Context, studentIDs []int64) ([]VisitGroupNames, error)
}

// GroupSupervisorRepository defines operations for managing active group supervisors
type GroupSupervisorRepository interface {
	base.Repository[*GroupSupervisor]

	// FindActiveByStaffID finds all active supervisions for a specific staff member
	FindActiveByStaffID(ctx context.Context, staffID int64) ([]*GroupSupervisor, error)

	// FindAllActive finds all currently active supervisions across all staff
	FindAllActive(ctx context.Context) ([]*GroupSupervisor, error)

	// FindByActiveGroupID finds supervisors for a specific active group
	// If activeOnly is true, only returns supervisors with end_date IS NULL (currently active)
	FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*GroupSupervisor, error)

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

	// CreateBulk inserts multiple supervisors in a single query.
	// All supervisors must have valid fields and tenant IDs set before calling.
	CreateBulk(ctx context.Context, supervisors []*GroupSupervisor) error

	// EndSupervisionsByActiveGroupIDs ends all active supervisions for multiple group IDs in a single query.
	// Returns the number of supervisions ended.
	EndSupervisionsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) (int64, error)
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

// AttendanceRepository is already defined above

// WorkSessionRepository defines operations for managing staff work sessions
type WorkSessionRepository interface {
	base.Repository[*WorkSession]

	// GetByStaffAndDate returns the work session for a staff member on a given date
	GetByStaffAndDate(ctx context.Context, staffID int64, date time.Time) (*WorkSession, error)

	// GetCurrentByStaffID returns the active (not checked out) session for a staff member
	GetCurrentByStaffID(ctx context.Context, staffID int64) (*WorkSession, error)

	// GetHistoryByStaffID returns work sessions for a staff member in a date range
	GetHistoryByStaffID(ctx context.Context, staffID int64, from, to time.Time) ([]*WorkSession, error)

	// GetOpenSessions returns all sessions without check-out before a given date
	GetOpenSessions(ctx context.Context, beforeDate time.Time) ([]*WorkSession, error)

	// GetTodayPresenceMap returns a map of staff IDs to their work status for today
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)

	// CloseSession sets the check-out time and auto_checked_out flag
	CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) error

	// UpdateBreakMinutes sets the break_minutes cache field on a session
	UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error
}

// StaffAbsenceRepository defines operations for managing staff absences
type StaffAbsenceRepository interface {
	base.Repository[*StaffAbsence]

	// GetByStaffAndDateRange returns absences for a staff member overlapping the given date range
	GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to time.Time) ([]*StaffAbsence, error)

	// GetByStaffAndDate returns an absence for a staff member on a specific date, or nil
	GetByStaffAndDate(ctx context.Context, staffID int64, date time.Time) (*StaffAbsence, error)

	// GetByDateRange returns all absences overlapping the given date range
	GetByDateRange(ctx context.Context, from, to time.Time) ([]*StaffAbsence, error)

	// GetTodayAbsenceMap returns a map of staff IDs to their absence type for today
	// Priority order when multiple absences exist: sick > training > vacation > other
	GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error)
}

// WorkSessionBreakRepository defines operations for managing work session breaks
type WorkSessionBreakRepository interface {
	base.Repository[*WorkSessionBreak]

	// GetBySessionID returns all breaks for a given session ordered by started_at
	GetBySessionID(ctx context.Context, sessionID int64) ([]*WorkSessionBreak, error)

	// GetActiveBySessionID returns the currently active (no ended_at) break for a session, or nil
	GetActiveBySessionID(ctx context.Context, sessionID int64) (*WorkSessionBreak, error)

	// EndBreak sets ended_at and duration_minutes on a break
	EndBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) error

	// UpdateDuration updates the duration and ended_at of a completed break
	UpdateDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) error

	// GetExpiredBreaks returns all active breaks with planned_end_time <= before
	// Used by the scheduler to auto-end breaks
	GetExpiredBreaks(ctx context.Context, before time.Time) ([]*WorkSessionBreak, error)
}
