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

	// FindActiveByGroupID finds all active instances of a specific activity group
	FindActiveByGroupID(ctx context.Context, groupID int64) ([]*Group, error)

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

	// GetCurrentByStudentID finds the current active visit for a student
	GetCurrentByStudentID(ctx context.Context, studentID int64) (*Visit, error)

	// GetCurrentByStudentIDs finds the current active visit for multiple students
	GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*Visit, error)

	// FindActiveVisits finds all visits with no exit time (currently active)
	FindActiveVisits(ctx context.Context) ([]*Visit, error)
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

	// GetStaffIDsWithSupervisionToday returns staff IDs who had any supervision activity today
	GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error)

	// EndAllActiveByStaffID ends all active supervisions for a staff member (sets end_date = CURRENT_DATE)
	// Returns the number of supervisions that were ended
	EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error)
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
