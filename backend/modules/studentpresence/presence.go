package studentpresence

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

// The presence capability is published as small facades. A consumer declares
// the facades it calls; the composition root wires one Presence value that
// implements all of them.

// SessionDetail is a room session with the display relations the location,
// kiosk and supervision reads render. Facilities owns the room and Timetable
// the activity; the session carries the projections it was read with.
type SessionDetail struct {
	ID, TenantID            int64
	CreatedAt, UpdatedAt    time.Time
	StartTime, LastActivity time.Time
	EndTime                 *time.Time
	TimeoutMinutes          int
	ActivityGroupID         *int64
	DeviceID                *int64
	RoomID                  int64
	Activity                *SessionActivitySummary
	Room                    *SessionRoomSummary
	Supervisors             []GroupSupervision
}

// IsActive reports whether the session has not ended yet.
func (s *SessionDetail) IsActive() bool { return s.EndTime == nil }

// ActivityConflictInfo reports whether starting an activity on a device
// collides with a running session.
type ActivityConflictInfo struct {
	HasConflict       bool       `json:"has_conflict"`
	ConflictingGroup  *LiveGroup `json:"conflicting_group,omitempty"`
	ConflictingDevice *string    `json:"conflicting_device,omitempty"`
	ConflictMessage   string     `json:"conflict_message"`
	CanOverride       bool       `json:"can_override"`
}

// PresenceModes resolves the tenant's presence mode ("detailed" | "binary").
// Missing wiring, lookup failures and invalid values return an error.
type PresenceModes interface {
	GetPresenceMode(ctx context.Context) (string, error)
}

// SessionReads reads running room sessions with their display relations.
type SessionReads interface {
	GetActiveGroup(ctx context.Context, id int64) (*SessionDetail, error)
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*SessionDetail, error)
	GetUnclaimedActiveGroups(ctx context.Context) ([]*SessionDetail, error)
	// GetRoomsByIDs retrieves the rooms of sessions by room ID.
	GetRoomsByIDs(ctx context.Context, ids []int64) ([]*SessionRoomSummary, error)
	// GetActiveGroupVisitsWithDisplay returns the open visits of a session
	// joined with tenant-scoped student display data.
	GetActiveGroupVisitsWithDisplay(ctx context.Context, activeGroupID int64) ([]*VisitDisplay, error)
	// GetActiveGroupVisitsWithDisplayForGroups is the batch form: one visit
	// query and one directory query for any number of sessions.
	GetActiveGroupVisitsWithDisplayForGroups(ctx context.Context, activeGroupIDs []int64) ([]*VisitDisplay, error)
	CountActiveVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) (int, error)
	// GetTrackingIndicators returns per-student match results for the given
	// labels; each student gets a []bool aligned with the labels slice.
	GetTrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error)
	YardRoomColorResolver
}

// KioskSessions runs the device-bound activity sessions of the kiosk.
type KioskSessions interface {
	StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*SessionDetail, error)
	ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*SessionDetail, error)
	CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error)
	GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*SessionDetail, error)
	FindDeviceActiveGroupInRoom(ctx context.Context, roomID, deviceID int64) (*SessionDetail, error)
	// UpdateActiveGroupSupervisors replaces the session's supervisors.
	UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*SessionDetail, error)
	ProcessSessionTimeout(ctx context.Context, deviceID int64) (*TimeoutResult, error)
	ValidateSessionTimeout(ctx context.Context, deviceID int64, timeoutMinutes int) error
	GetSessionTimeoutInfo(ctx context.Context, deviceID int64) (*SessionTimeoutInfo, error)
}

// SessionCommands opens, touches and ends room sessions and records their
// supervision. CreateActiveGroup and CreateGroupSupervisor fill the stored
// identity and timestamps into the passed value.
type SessionCommands interface {
	CreateActiveGroup(ctx context.Context, group *LiveGroup) error
	DeleteActiveGroup(ctx context.Context, id int64) error
	EndActiveGroupSession(ctx context.Context, id int64) error
	EndActivitySession(ctx context.Context, activeGroupID int64) error
	UpdateSessionActivity(ctx context.Context, activeGroupID int64) error
	CreateGroupSupervisor(ctx context.Context, supervision *GroupSupervision) error
}

// SessionMaintenance is the scheduled session housekeeping.
type SessionMaintenance interface {
	EndDailySessions(ctx context.Context) (*DailySessionCleanupResult, error)
	CleanupAbandonedSessions(ctx context.Context, olderThan time.Duration) (int, error)
}

// VisitReads answers where students are right now.
type VisitReads interface {
	GetStudentCurrentVisit(ctx context.Context, studentID int64) (*Visit, error)
	GetStudentCurrentVisitWithRoom(ctx context.Context, studentID int64) (*VisitWithRoom, error)
	GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*Visit, error)
	FindVisitsByStudentID(ctx context.Context, studentID int64) ([]Visit, error)
	ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error)
	ListStudentsInTransit(ctx context.Context) ([]int64, error)
	ListStudentsPresentToday(ctx context.Context) ([]int64, error)
	ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error)
}

// VisitCommands records and ends single room visits.
type VisitCommands interface {
	CreateVisit(ctx context.Context, visit *Visit) error
	EndVisit(ctx context.Context, id int64) error
}

// StudentMoves moves students between room sessions while preserving the
// visit timeline. Authorization is revalidated against the locked move state.
type StudentMoves interface {
	MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth StudentMoveAuthorization) (*StudentMoveResult, error)
	MoveStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID int64, auth StudentMoveAuthorization) (*StudentMoveResult, error)
	// EnsureOpenRoomSession finds or opens the room's own session (ADR 0018).
	EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*SessionDetail, error)
}

// AttendanceCommands checks students in to and out of school.
type AttendanceCommands interface {
	// ToggleStudentAttendance flips state based on the current row — used by
	// the IoT kiosk where a single device serializes scans. NOT safe under
	// concurrent web callers; web callers use CheckInStudent / CheckOutStudent.
	ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error)
	// CheckInStudent applies "in" unconditionally; Action is always
	// "checked_in" on return.
	CheckInStudent(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error)
	// CheckOutStudent applies "out" unconditionally and ends any open room
	// visit in the same transaction (issue #895).
	CheckOutStudent(ctx context.Context, studentID, staffID int64, skipAuthCheck bool) (*AttendanceResult, error)
	// ProcessSchoolCheckinBatch applies one explicit school check-in/out
	// action ("in" | "out") to a set of students in a single call (#2359).
	ProcessSchoolCheckinBatch(ctx context.Context, studentIDs []int64, staffID int64, action string) (*SchoolCheckinBatchResult, error)
	// ConfirmDailyCheckout processes a deferred daily-checkout confirmation
	// of an IoT device. Returns ErrNoAttendanceRecordForCheckout when no
	// attendance record exists today.
	ConfirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (*DailyCheckoutResult, error)
}

// AttendanceReads reads the students' school attendance of today.
type AttendanceReads interface {
	GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*DailyAttendanceStatus, error)
	GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*DailyAttendanceStatus, error)
	// HasOpenAttendanceOn reports whether any attendance row on the given
	// calendar date is still open. Used by the presence-mode switch guard.
	HasOpenAttendanceOn(ctx context.Context, date timezone.Date) (bool, error)
}

// Presence is the whole presence capability the composition root wires once.
type Presence interface {
	PresenceModes
	SessionReads
	KioskSessions
	SessionCommands
	SessionMaintenance
	VisitReads
	VisitCommands
	StudentMoves
	AttendanceCommands
	AttendanceReads
}

// PresenceCleanup applies and previews the data-retention cleanup.
type PresenceCleanup interface {
	// CleanupExpiredVisits runs the cleanup process for all students
	CleanupExpiredVisits(ctx context.Context) (*CleanupResult, error)
	// GetRetentionStatistics gets statistics about data that will be deleted
	GetRetentionStatistics(ctx context.Context) (*RetentionStats, error)
	// PreviewCleanup shows what would be deleted without actually deleting
	PreviewCleanup(ctx context.Context) (*CleanupPreview, error)
	// CleanupStaleAttendance closes attendance records from previous days
	CleanupStaleAttendance(ctx context.Context) (*AttendanceCleanupResult, error)
	// PreviewAttendanceCleanup shows what attendance records would be cleaned
	PreviewAttendanceCleanup(ctx context.Context) (*AttendanceCleanupPreview, error)
	// CleanupStaleSupervisors closes supervisor records from previous days that lack end_date
	CleanupStaleSupervisors(ctx context.Context) (*SupervisorCleanupResult, error)
	// PreviewSupervisorCleanup shows what supervisor records would be cleaned
	PreviewSupervisorCleanup(ctx context.Context) (*SupervisorCleanupPreview, error)
}

// StudentHistory exposes the reads (and the GDPR access-log write) behind the
// student attendance-history endpoints.
type StudentHistory interface {
	// GetAttendanceByStudentAndDateRange returns a student's attendance rows
	// between two dates (inclusive).
	GetAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*Attendance, error)
	// GetAttendanceForDateByStudentIDs returns attendance rows for the supplied
	// students on one calendar date.
	GetAttendanceForDateByStudentIDs(ctx context.Context, date timezone.Date, studentIDs []int64) ([]*Attendance, error)
	// GetVisitsByStudentAndTimeRange returns a student's visits (active or
	// ended) entered within the inclusive time range.
	GetVisitsByStudentAndTimeRange(ctx context.Context, studentID int64, start, end time.Time) ([]*VisitHistoryEntry, error)
	GetSlotAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*HistorySlot, error)
	// HasPlannedSlotsInRange reports whether the tenant has any planned slot
	// assignment on a non-cancelled instance within the inclusive date range.
	HasPlannedSlotsInRange(ctx context.Context, startDate, endDate timezone.Date) (bool, error)
	// RecordDataAccess writes a GDPR data-access log entry.
	RecordDataAccess(ctx context.Context, entry *DataAccessEvent) error
}

// StatusDayReads reads the reported status days (sick / excused / class
// trip). Care Plan owns the rows; presence reads them through its adapters.
type StatusDayReads interface {
	GetActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*absencerecords.StudentStatusDay, error)
	GetSignedOffByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*absencerecords.StudentStatusDay, error)
	GetActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*absencerecords.StudentStatusDay, error)
	GetByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*absencerecords.StudentStatusDay, error)
}

// StatusDayWrites records and clears status days. The writes persist through
// Care Plan, the single write owner of active.student_status_days.
type StatusDayWrites interface {
	UpsertReported(ctx context.Context, entry *absencerecords.StudentStatusDay) error
	MarkCleared(ctx context.Context, studentID int64, status string, date timezone.Date, clearedAt time.Time, source string) error
	CreateForDates(ctx context.Context, wc StatusDayWriteContext, studentID int64, status, reason string, dates []timezone.Date) error
	BulkCreateForDates(ctx context.Context, wc StatusDayWriteContext, studentIDs []int64, status, reason string, dates []timezone.Date) error
	// DeleteStatusDay removes one of the student's status days inside the
	// write context's tenant transaction.
	DeleteStatusDay(ctx context.Context, wc StatusDayWriteContext, statusDayID, studentID int64) error
}

// StatusDays is the status-day capability the composition root wires once.
type StatusDays interface {
	StatusDayReads
	StatusDayWrites
}

// StatusDayOverviews assembles the tenant-wide absence overview.
type StatusDayOverviews interface {
	GetOverview(ctx context.Context, groups []*StatusDayOverviewGroup, from, to, today timezone.Date, filters StatusDayOverviewFilters) (*StatusDayOverview, error)
}
