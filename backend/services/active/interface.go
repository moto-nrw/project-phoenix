package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Service defines operations for managing active groups and visits
type Service interface {
	// Active Group operations
	GetActiveGroup(ctx context.Context, id int64) (*active.Group, error)
	CreateActiveGroup(ctx context.Context, group *active.Group) error
	UpdateActiveGroup(ctx context.Context, group *active.Group) error
	DeleteActiveGroup(ctx context.Context, id int64) error
	ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error)
	FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error)
	FindDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) (*active.Group, error)
	FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error)
	EndActiveGroupSession(ctx context.Context, id int64) error
	GetActiveGroupWithVisits(ctx context.Context, id int64) (*active.Group, error)
	GetActiveGroupWithSupervisors(ctx context.Context, id int64) (*active.Group, error)

	// Visit operations
	GetVisit(ctx context.Context, id int64) (*active.Visit, error)
	CreateVisit(ctx context.Context, visit *active.Visit) error
	UpdateVisit(ctx context.Context, visit *active.Visit) error
	DeleteVisit(ctx context.Context, id int64) error
	ListVisits(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error)
	FindVisitsByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error)
	FindVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error)
	EndVisit(ctx context.Context, id int64) error
	GetStudentCurrentVisit(ctx context.Context, studentID int64) (*active.Visit, error)
	GetStudentCurrentVisitWithRoom(ctx context.Context, studentID int64) (*active.Visit, error)
	GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error)
	CountActiveVisitsByRoomID(ctx context.Context, roomID int64) (int, error)
	CountActiveVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) (int, error)
	ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error)
	ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error)
	ListStudentsInTransit(ctx context.Context) ([]int64, error)
	ListStudentsPresentToday(ctx context.Context) ([]int64, error)
	AssignTransitStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64) (*TransitAssignResult, error)
	MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth StudentMoveAuthorization) (*StudentMoveResult, error)
	MoveStudentsToTransitAuthorized(ctx context.Context, studentIDs []int64, auth StudentMoveAuthorization) (*StudentMoveResult, error)

	// Group Supervisor operations
	GetGroupSupervisor(ctx context.Context, id int64) (*active.GroupSupervisor, error)
	CreateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error
	UpdateGroupSupervisor(ctx context.Context, supervisor *active.GroupSupervisor) error
	DeleteGroupSupervisor(ctx context.Context, id int64) error
	ListGroupSupervisors(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error)
	FindSupervisorsByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error)
	FindSupervisorsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupSupervisor, error)
	FindSupervisorsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*active.GroupSupervisor, error)
	EndSupervision(ctx context.Context, id int64) error
	GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error)

	// Combined Group operations
	GetCombinedGroup(ctx context.Context, id int64) (*active.CombinedGroup, error)
	CreateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error
	UpdateCombinedGroup(ctx context.Context, group *active.CombinedGroup) error
	DeleteCombinedGroup(ctx context.Context, id int64) error
	ListCombinedGroups(ctx context.Context, options *base.QueryOptions) ([]*active.CombinedGroup, error)
	FindActiveCombinedGroups(ctx context.Context) ([]*active.CombinedGroup, error)
	FindCombinedGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.CombinedGroup, error)
	EndCombinedGroup(ctx context.Context, id int64) error
	GetCombinedGroupWithGroups(ctx context.Context, id int64) (*active.CombinedGroup, error)
	CreateCombinedGroupWithGroups(ctx context.Context, group *active.CombinedGroup, groupIDs []int64) error

	// Group Mapping operations
	AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error
	RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error
	GetGroupMappingsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error)
	GetGroupMappingsByCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error)

	// Activity Session Management with Conflict Detection
	StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error)
	CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error)
	EndActivitySession(ctx context.Context, activeGroupID int64) error
	ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error)
	GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*active.Group, error)

	// Dynamic Supervisor Management
	UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*active.Group, error)

	// Session timeout operations
	ProcessSessionTimeout(ctx context.Context, deviceID int64) (*TimeoutResult, error)
	UpdateSessionActivity(ctx context.Context, activeGroupID int64) error
	ValidateSessionTimeout(ctx context.Context, deviceID int64, timeoutMinutes int) error
	GetSessionTimeoutInfo(ctx context.Context, deviceID int64) (*SessionTimeoutInfo, error)
	CleanupAbandonedSessions(ctx context.Context, olderThan time.Duration) (int, error)

	// Daily session management
	EndDailySessions(ctx context.Context) (*DailySessionCleanupResult, error)

	// Analytics and statistics
	GetDashboardAnalytics(ctx context.Context) (*DashboardAnalytics, error)
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*active.Group, error)

	// Attendance tracking operations
	GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*AttendanceStatus, error)
	// GetRoomsByIDs retrieves rooms by ID (issue #584 lookup; repository
	// result returned verbatim).
	GetRoomsByIDs(ctx context.Context, ids []int64) ([]*facilityModels.Room, error)
	// GetActiveGroupVisitsWithDisplay returns the open visits of an active
	// group joined with student display data (issue #584 lookup; repository
	// result returned verbatim).
	GetActiveGroupVisitsWithDisplay(ctx context.Context, activeGroupID int64) ([]*active.VisitWithStudentDisplay, error)
	// HasOpenAttendanceOn reports whether any attendance row on the given
	// calendar date is still open (issue #584 lookup; repository result
	// returned verbatim). Used by the operator presence-mode switch guard.
	HasOpenAttendanceOn(ctx context.Context, date timezone.Date) (bool, error)
	GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*AttendanceStatus, error)
	// ToggleStudentAttendance flips state based on the current row — used by
	// the IoT kiosk where a single device serializes scans. NOT safe under
	// concurrent web callers because the read-then-flip can swap an "in"
	// click into an "out" if another caller wins the race; web callers must
	// use CheckInStudent / CheckOutStudent below, which never flip the
	// requested action against the observed state.
	ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error)
	// CheckInStudent applies "in" unconditionally. The insert is ON CONFLICT
	// DO NOTHING against the partial unique index, so a concurrent winner is
	// transparently absorbed; Action is always "checked_in" on return.
	CheckInStudent(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error)
	// CheckOutStudent applies "out" unconditionally via a state-checked
	// UPDATE WHERE check_out_time IS NULL — closes the open row when one
	// exists, returns idempotent success otherwise. Action is always
	// "checked_out" on return. Every checkout (this method and the toggle's
	// "out" branch) also ends any open room visit in the same request
	// transaction, so attendance "checked_out" never coexists with an open
	// visit (issue #895).
	CheckOutStudent(ctx context.Context, studentID, staffID int64, skipAuthCheck bool) (*AttendanceResult, error)
	// CheckOutStudentFromDevice applies "out" for an IoT device after
	// resolving the active session supervisor used as the checkout principal.
	CheckOutStudentFromDevice(ctx context.Context, studentID, deviceID int64) (*AttendanceResult, error)
	// ProcessSchoolCheckinBatch applies one explicit school check-in/out
	// action ("in" | "out") to a set of students in a single call (#2359).
	// The caller must already be authorized (route-level users:checkin gate);
	// ids the caller cannot act on (unknown, another tenant's, graduated)
	// come back as OK=false items, unexpected write errors fail the whole
	// batch. See the implementation doc comment for the ordering and
	// idempotency contract.
	ProcessSchoolCheckinBatch(ctx context.Context, studentIDs []int64, staffID int64, action string) (*SchoolCheckinBatchResult, error)
	CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error)
	// ConfirmDailyCheckout processes a deferred daily-checkout confirmation for
	// an IoT device: it validates the student has today's attendance record and,
	// when destination is "zuhause" and the student is still checked in, checks
	// them out and broadcasts the SSE update. Returns
	// ErrNoAttendanceRecordForCheckout when no attendance record exists today.
	ConfirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (*DailyCheckoutResult, error)

	// Unclaimed groups management (deviceless claiming)
	GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error)
	ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error)

	// Cross-tenant student visibility (Ferienbetreuung / holiday care)
	GetCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]active.CrossTenantStudent, error)

	// Tracking indicators — returns per-student match results for the given labels.
	// Each student gets a []bool aligned with the labels slice.
	GetTrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error)

	// GetPresenceMode resolves the tenant's presence mode ("detailed" | "binary").
	// Fails safe to "detailed" when the settings resolver is nil or the lookup
	// errors. Use this instead of calling the settings service directly so every
	// caller gets consistent fallback behavior.
	GetPresenceMode(ctx context.Context) string

	// Injects the tenant-scoped settings resolver (optional).
	// Called by the factory after the settings service is constructed.
	SetSettingsService(resolver SettingsResolver)

	// SetTenantRuntime injects the transaction runtime bound to this service's
	// repository pool. The cleanup commands run without a request transaction
	// and need it to open one of their own.
	SetTenantRuntime(runtime tenant.UnitOfWork)
}

// TransitAssignSkipped describes a student that was not assigned during a
// transit bulk operation.
type TransitAssignSkipped struct {
	StudentID int64  `json:"student_id"`
	Reason    string `json:"reason"`
}

// TransitAssignResult is returned when transit students are assigned to an
// active room session.
type TransitAssignResult struct {
	Assigned      []int64                `json:"assigned"`
	Skipped       []TransitAssignSkipped `json:"skipped"`
	ActiveGroupID int64                  `json:"active_group_id"`
	RoomID        int64                  `json:"room_id"`
}

// StudentMoveSkipped describes a student that could not be moved during a
// historical room/Schulhof move operation.
type StudentMoveSkipped struct {
	StudentID int64  `json:"student_id"`
	Reason    string `json:"reason"`
}

// StudentMoveResult is returned by real move operations that preserve the
// room-visit timeline instead of mutating an existing visit's active_group_id.
type StudentMoveResult struct {
	Moved         []int64              `json:"moved"`
	Unchanged     []int64              `json:"unchanged"`
	Skipped       []StudentMoveSkipped `json:"skipped"`
	ActiveGroupID *int64               `json:"active_group_id,omitempty"`
	RoomID        *int64               `json:"room_id,omitempty"`
	// PreviousActiveGroupIDs records the source observed after move serialization.
	// It is service metadata and intentionally not part of the bulk-move API.
	PreviousActiveGroupIDs map[int64]int64 `json:"-"`
}

// StudentMoveAuthorization carries the caller context needed to authorize
// bulk move requests against the locked move state inside the service.
type StudentMoveAuthorization struct {
	StaffID              int64
	BypassResourceChecks bool
}

// DashboardAnalytics represents aggregated analytics for dashboard
type DashboardAnalytics struct {
	// Student Overview
	StudentsPresent      int
	StudentsInTransit    int // Students present but not in any active visit
	StudentsOnPlayground int
	StudentsInRooms      int // Students in indoor rooms (excluding playground)
	StudentsSick         int // Students currently flagged as sick
	StudentsExcused      int // Students currently flagged as excused
	StudentsHome         int // Active students neither present nor sick/excused — see calculateStudentsHome

	// Activities & Rooms
	ActiveActivities    int
	FreeRooms           int
	TotalRooms          int
	CapacityUtilization float64
	ActivityCategories  int

	// OGS Groups
	ActiveOGSGroups      int
	StudentsInGroupRooms int
	SupervisorsToday     int
	StudentsInHomeRoom   int

	// Recent Activity (Privacy-compliant)
	RecentActivity []RecentActivity

	// Current Activities (No personal data)
	CurrentActivities []CurrentActivity

	// Active Groups Summary
	ActiveGroupsSummary []ActiveGroupInfo

	// Timestamp
	LastUpdated time.Time
}

// RecentActivity represents a recent activity without personal data
type RecentActivity struct {
	Type      string
	GroupName string
	RoomName  string
	Count     int
	Timestamp time.Time
}

// CurrentActivity represents current activity status
type CurrentActivity struct {
	ID           int64
	Name         string
	Category     string
	Participants int
	MaxCapacity  int
	Status       string
}

// ActiveGroupInfo represents active group summary
type ActiveGroupInfo struct {
	Name         string
	Type         string
	StudentCount int
	Location     string
	Status       string
}

// ActivityConflictInfo represents information about a detected activity conflict
type ActivityConflictInfo struct {
	HasConflict       bool          `json:"has_conflict"`
	ConflictingGroup  *active.Group `json:"conflicting_group,omitempty"`
	ConflictingDevice *string       `json:"conflicting_device,omitempty"`
	ConflictMessage   string        `json:"conflict_message"`
	CanOverride       bool          `json:"can_override"`
}

// TimeoutResult represents the result of processing a session timeout.
// ActivityID is *int64 because spontaneous sessions (WP-B6) carry no parent
// template; it is serialized as null on the wire rather than omitted.
type TimeoutResult struct {
	SessionID          int64     `json:"session_id"`
	ActivityID         *int64    `json:"activity_id"`
	StudentsCheckedOut int       `json:"students_checked_out"`
	TimeoutAt          time.Time `json:"timeout_at"`
}

// SessionTimeoutInfo provides information about a session's timeout status.
// ActivityID follows the same *int64 contract as TimeoutResult.
type SessionTimeoutInfo struct {
	SessionID          int64         `json:"session_id"`
	ActivityID         *int64        `json:"activity_id"`
	StartTime          time.Time     `json:"start_time"`
	LastActivity       time.Time     `json:"last_activity"`
	TimeoutMinutes     int           `json:"timeout_minutes"`
	InactivityDuration time.Duration `json:"inactivity_duration"`
	TimeUntilTimeout   time.Duration `json:"time_until_timeout"`
	IsTimedOut         bool          `json:"is_timed_out"`
	ActiveStudentCount int           `json:"active_student_count"`
}

// CleanupService defines operations for data retention and cleanup
type CleanupService interface {
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

// CleanupResult represents the result of a cleanup operation
type CleanupResult struct {
	StartedAt         time.Time
	CompletedAt       time.Time
	StudentsProcessed int
	RecordsDeleted    int64
	Errors            []CleanupError
	Success           bool
}

// CleanupError represents an error during cleanup for a specific student
type CleanupError struct {
	StudentID int64
	Error     string
	Timestamp time.Time
}

// RetentionStats represents statistics about data retention
type RetentionStats struct {
	TotalExpiredVisits   int64
	StudentsAffected     int
	OldestExpiredVisit   *time.Time
	ExpiredVisitsByMonth map[string]int64
}

// CleanupPreview shows what would be deleted
type CleanupPreview struct {
	StudentVisitCounts map[int64]int // Student ID -> number of visits to delete
	TotalVisits        int64
	OldestVisit        *time.Time
}

// AttendanceStatus represents a student's current attendance status for the day
type AttendanceStatus struct {
	StudentID int64 `json:"student_id"`
	// Status is derived from the attendance row's timestamps:
	//   "not_checked_in" — no attendance row today
	//   "checked_in"     — row exists, CheckOutTime nil, YardSince nil (in the building)
	//   "on_yard"        — row exists, CheckOutTime nil, YardSince non-nil (on premises, outside the building)
	//   "checked_out"    — CheckOutTime non-nil (formally left school)
	Status       string        `json:"status"`
	Date         timezone.Date `json:"date"`
	CheckInTime  *time.Time    `json:"check_in_time"`
	CheckOutTime *time.Time    `json:"check_out_time"`
	// YardSince, when non-nil, marks the moment the student moved to the
	// schoolyard without checking out. Only meaningful while Status == "on_yard".
	YardSince    *time.Time `json:"yard_since,omitempty"`
	CheckedInBy  string     `json:"checked_in_by"`  // Formatted as "FirstName LastName"
	CheckedOutBy string     `json:"checked_out_by"` // Formatted as "FirstName LastName"
}

// IsCurrentlyPresent reports whether the attendance row represents a child
// who is still on school premises. A completed attendance keeps its historical
// check-in time but is no longer current presence.
func (s *AttendanceStatus) IsCurrentlyPresent() bool {
	return s != nil && s.CheckInTime != nil &&
		(s.Status == "checked_in" || s.Status == "on_yard")
}

// DailyCheckoutResult represents the outcome of confirming a deferred daily
// checkout from an IoT device.
type DailyCheckoutResult struct {
	// Action is "checked_out_daily" when the student went home ("zuhause") or
	// "checked_out" when they stayed in transit ("unterwegs").
	Action string
}

// AttendanceResult represents the result of a student attendance toggle operation
type AttendanceResult struct {
	Action       string    `json:"action"` // "checked_in", "checked_out"
	AttendanceID int64     `json:"attendance_id"`
	StudentID    int64     `json:"student_id"`
	Timestamp    time.Time `json:"timestamp"`
	// Changed reports whether THIS call mutated the attendance row: false when
	// a concurrent caller already established the target state (absorbed
	// in/in race, already-closed row). Not serialized — it informs the web
	// no-op display, while the IoT wire format stays byte-identical.
	Changed bool `json:"-"`
}

// DailySessionCleanupResult represents the result of ending daily sessions
type DailySessionCleanupResult struct {
	SessionsEnded       int       `json:"sessions_ended"`
	VisitsEnded         int       `json:"visits_ended"`
	SupervisorsEnded    int       `json:"supervisors_ended"`
	EndedActiveGroupIDs []int64   `json:"-"`
	ExecutedAt          time.Time `json:"executed_at"`
	Success             bool      `json:"success"`
	Errors              []string  `json:"errors,omitempty"`
}

// AttendanceCleanupResult represents the result of cleaning stale attendance records
type AttendanceCleanupResult struct {
	StartedAt        time.Time      `json:"started_at"`
	CompletedAt      time.Time      `json:"completed_at"`
	RecordsClosed    int            `json:"records_closed"`
	StudentsAffected int            `json:"students_affected"`
	OldestRecordDate *timezone.Date `json:"oldest_record_date,omitempty"`
	Success          bool           `json:"success"`
	Errors           []string       `json:"errors,omitempty"`
}

// AttendanceCleanupPreview shows what attendance records would be cleaned
type AttendanceCleanupPreview struct {
	TotalRecords   int            `json:"total_records"`
	StudentRecords map[int64]int  `json:"student_records"` // studentID -> count
	OldestRecord   *timezone.Date `json:"oldest_record,omitempty"`
	RecordsByDate  map[string]int `json:"records_by_date"` // date -> count
}

// SupervisorCleanupResult represents the result of cleaning stale supervisor records
type SupervisorCleanupResult struct {
	StartedAt        time.Time      `json:"started_at"`
	CompletedAt      time.Time      `json:"completed_at"`
	RecordsClosed    int            `json:"records_closed"`
	StaffAffected    int            `json:"staff_affected"`
	OldestRecordDate *timezone.Date `json:"oldest_record_date,omitempty"`
	Success          bool           `json:"success"`
	Errors           []string       `json:"errors,omitempty"`
}

// SupervisorCleanupPreview shows what supervisor records would be cleaned
type SupervisorCleanupPreview struct {
	TotalRecords  int            `json:"total_records"`
	StaffRecords  map[int64]int  `json:"staff_records"` // staffID -> count
	OldestRecord  *timezone.Date `json:"oldest_record,omitempty"`
	RecordsByDate map[string]int `json:"records_by_date"` // date -> count
}
