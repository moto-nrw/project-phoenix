package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// GroupRepository defines operations for managing active groups
type GroupRepository interface {
	base.Repository[*Group]

	// CountWithOptions is the generic filtered count promoted from the
	// embedded base repository.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)

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

	FindWithSupervisors(ctx context.Context, id int64) (*Group, error)

	FindActiveByDeviceID(ctx context.Context, deviceID int64) (*Group, error)
	FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*Group, error)

	// Room conflict detection methods
	CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *Group, error)

	// Session timeout methods
	UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error
	FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*Group, error)
	// Unclaimed groups (for frontend claiming feature)
	FindUnclaimed(ctx context.Context) ([]*Group, error)

	// FindActiveGroups finds all groups with no end time (currently active)
	FindActiveGroups(ctx context.Context) ([]*Group, error)

	// FindByIDs finds active groups by their IDs
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Group, error)

	// FindByIDForUpdate finds an active group by ID and locks the row for
	// the current transaction.
	FindByIDForUpdate(ctx context.Context, id int64) (*Group, error)

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

	// GetCurrentRoomNamesForStudents returns the room name of each student's
	// current open visit; students without one are absent from the map.
	GetCurrentRoomNamesForStudents(ctx context.Context, studentIDs []int64) (map[int64]string, error)

	// FindByActiveGroupID finds all visits for a specific active group
	FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*Visit, error)

	// FindByTimeRange finds all visits active during a specific time range
	FindByTimeRange(ctx context.Context, start, end time.Time) ([]*Visit, error)

	// FindByStudentAndTimeRange finds all visits (active or ended) for a specific
	// student whose entry_time falls within [start, end], ordered by entry_time desc.
	FindByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*Visit, error)

	// FindActiveWithStudentDisplayByGroup returns the open visits of an
	// active group joined with the students' display data (name, class,
	// education group, status flags, photo path), newest entry first.
	FindActiveWithStudentDisplayByGroup(ctx context.Context, activeGroupID int64) ([]*VisitWithStudentDisplay, error)

	// FindByStudentAndActiveGroupIDs returns visits for the student whose
	// active_group_id is in the given list. Used by the timetable per-student
	// day view to detect unplanned attendance in a single batch query.
	// Returns an empty slice (no DB call) when the id list is empty.
	FindByStudentAndActiveGroupIDs(ctx context.Context, studentID int64, activeGroupIDs []int64) ([]*Visit, error)

	// EndVisit marks a visit as ended at the current time
	EndVisit(ctx context.Context, id int64) error

	// TransferVisitsFromRecentSessions transfers active visits from recent ended sessions on the same device to a new session
	TransferVisitsFromRecentSessions(ctx context.Context, newActiveGroupID, deviceID int64) (int, error)

	// TransferActiveVisitsBetweenGroups moves still-open visits from one active
	// group to another. Ended visits are ignored so stale callers cannot reopen
	// a checkout by writing an old NULL exit_time back to the row.
	TransferActiveVisitsBetweenGroups(ctx context.Context, oldActiveGroupID, newActiveGroupID int64) (int, error)

	// Cleanup operations for data retention
	// DeleteExpiredVisits deletes visits older than retention days for a specific student
	DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error)

	// GetVisitRetentionStats gets statistics about visits that are candidates for deletion
	GetVisitRetentionStats(ctx context.Context) (map[int64]int, error)

	// CountExpiredVisits counts visits that are older than retention period for all students
	CountExpiredVisits(ctx context.Context) (int64, error)

	// OldestExpiredVisitDate returns the created_at of the oldest visit past
	// its per-student retention window, or nil when no visit is expired.
	OldestExpiredVisitDate(ctx context.Context) (*time.Time, error)

	// ExpiredVisitMonthlyCounts groups expired visits by calendar month of
	// created_at, keyed YYYY-MM. Feeds the GDPR retention statistics.
	ExpiredVisitMonthlyCounts(ctx context.Context) (map[string]int64, error)

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

	// ListActiveSupervisionBlockers returns still-open supervisions as
	// caregiver-capability blocker rows.
	ListActiveSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]users.BlockerSupervision, error)

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

	// EndByActiveGroupAndStaffID ends active supervisions for a specific
	// (active_group_id, staff_id) pair. Sets end_date=now() on all matching
	// rows with end_date IS NULL. Used by the substitute flow to remove the
	// absent staff's active supervisorship for a single active group without
	// touching their supervisions elsewhere. Idempotent: zero rows matched
	// is not an error (staff already ended or never supervised this group).
	EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error)

	// EndSupervisionsByActiveGroupIDs ends all active supervisions for multiple group IDs in a single query.
	// Returns the number of supervisions ended.
	EndSupervisionsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) (int64, error)

	// FindStaleOpen returns supervisor rows started before the given day that
	// still lack an end_date. Feeds the nightly stale-supervisor cleanup and
	// its preview.
	FindStaleOpen(ctx context.Context, before timezone.Date) ([]*GroupSupervisor, error)

	// UpdateColumns is the generic partial-update helper promoted from the
	// embedded base repository: updates only the named columns by primary
	// key and returns the number of rows affected.
	UpdateColumns(ctx context.Context, supervisor *GroupSupervisor, columns ...string) (int64, error)
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

	// LockStaffBalanceWrites serializes all work-session and break mutations
	// with absence and adjustment mutations for the same staff member.
	LockStaffBalanceWrites(ctx context.Context, staffID int64) error
	// LockStaffBalanceWritesForSession resolves a session's owner and takes
	// the same balance lock. Scheduler paths that start from a break only
	// know the session ID.
	LockStaffBalanceWritesForSession(ctx context.Context, sessionID int64) error

	// GetByStaffAndDate returns the work session for a staff member on a given date
	GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*WorkSession, error)

	// GetCurrentByStaffID returns the active (not checked out) session for a staff member
	GetCurrentByStaffID(ctx context.Context, staffID int64) (*WorkSession, error)

	// GetLatestOpenByStaffID returns the most recent not-checked-out session of
	// a staff member across all days. Callers that must not lose sight of a
	// session opened before midnight use this instead of GetCurrentByStaffID.
	GetLatestOpenByStaffID(ctx context.Context, staffID int64) (*WorkSession, error)

	// GetOpenByStaffAndDate returns the not-checked-out session of a staff
	// member on an explicit calendar day, for callers that must not re-derive
	// "today" while the request is running.
	GetOpenByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*WorkSession, error)

	// GetOpenByStaffAndDateForUpdate is GetOpenByStaffAndDate with a row lock.
	GetOpenByStaffAndDateForUpdate(ctx context.Context, staffID int64, date timezone.Date) (*WorkSession, error)

	// LockOpenByIDForUpdate returns and locks an open session row by ID.
	LockOpenByIDForUpdate(ctx context.Context, id int64) (*WorkSession, error)

	// GetHistoryByStaffID returns work sessions for a staff member in a date range
	GetHistoryByStaffID(ctx context.Context, staffID int64, from, to timezone.Date) ([]*WorkSession, error)

	// GetOpenSessions returns all sessions without check-out before a given date
	GetOpenSessions(ctx context.Context, beforeDate timezone.Date) ([]*WorkSession, error)

	// GetTodayPresenceMap returns a map of staff IDs to their work status for today
	GetTodayPresenceMap(ctx context.Context) (map[int64]string, error)

	// CloseSession sets the check-out time and auto_checked_out flag.
	// The boolean reports whether an open row was actually closed.
	CloseSession(ctx context.Context, id int64, checkOutTime time.Time, autoCheckedOut bool) (bool, error)

	// UpdateBreakMinutes sets the break_minutes cache field on a session
	UpdateBreakMinutes(ctx context.Context, id int64, breakMinutes int) error

	// Generic query helpers promoted from the embedded base repository.
	// Used by the time-tracking retention cleanup.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
	OldestBefore(ctx context.Context, dateColumn string, cutoff *timezone.Date) (*timezone.Date, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff timezone.Date) (int64, error)
}

// StaffAbsenceRepository defines operations for managing staff absences
type StaffAbsenceRepository interface {
	base.Repository[*StaffAbsence]

	// LockStaffAbsenceWrites serializes absence lifecycle writes for one staff
	// member inside the ambient tenant transaction. It also takes the shared
	// balance lock before the absence-specific lock. Callers must acquire it
	// before any overlap read-check-write sequence.
	LockStaffAbsenceWrites(ctx context.Context, staffID int64) error

	// GetByStaffAndDateRange returns absences for a staff member overlapping the given date range
	GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAbsence, error)

	// GetByStaffAndDate returns an absence for a staff member on a specific date, or nil
	GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*StaffAbsence, error)

	// GetTodayAbsenceMap returns a map of staff IDs to their absence type for today
	// Priority order when multiple absences exist:
	// sick > training > vacation > comp_time > other.
	GetTodayAbsenceMap(ctx context.Context) (map[int64]string, error)

	// ListByStatuses returns all absences whose status is in the given set,
	// ordered by requested_at (used for the /staff inbox: requested + question)
	ListByStatuses(ctx context.Context, statuses []string) ([]*StaffAbsence, error)

	// DeleteNonHistoricalByStaffID hard-deletes absences that are still pending
	// ('requested' or 'question') or not yet over (date_end >= from). Past
	// decided absences stay as history. Used by staff offboarding.
	DeleteNonHistoricalByStaffID(ctx context.Context, staffID int64, from timezone.Date) (int64, error)

	// Generic query helpers promoted from the embedded base repository.
	// Used by the time-tracking retention cleanup.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)
	OldestBefore(ctx context.Context, dateColumn string, cutoff *timezone.Date) (*timezone.Date, error)
	DeleteOlderThan(ctx context.Context, dateColumn string, cutoff timezone.Date) (int64, error)
}

type StaffAbsenceAuditRepository interface {
	Create(ctx context.Context, audit *StaffAbsenceAudit) error
}

// StaffBalanceAdjustmentRepository defines operations for Stundenkonto
// correction transactions (#1420).
type StaffBalanceAdjustmentRepository interface {
	base.Repository[*StaffBalanceAdjustment]

	// LockStaffBalanceWrites serializes balance-adjustment writes for one
	// staff member inside the ambient tenant transaction. Every adjustment
	// mutation acquires it before its first read or write.
	LockStaffBalanceWrites(ctx context.Context, staffID int64) error

	// GetByStaffAndDateRange returns adjustments whose effective_date lies in
	// [from, to], ordered by effective_date.
	GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffBalanceAdjustment, error)
}

// StaffVacationQuotaRepository defines operations for managing per-staff yearly entitlement
type StaffVacationQuotaRepository interface {
	base.Repository[*StaffVacationQuota]

	// GetByStaffAndYear returns the quota row for a specific staff/year, or nil
	GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*StaffVacationQuota, error)

	// Upsert creates or updates the quota for a staff/year combination
	Upsert(ctx context.Context, quota *StaffVacationQuota) error
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
