package presence

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// Service defines operations for managing active groups and visits
type Service interface {
	// Active Group operations
	GetActiveGroup(ctx context.Context, id int64) (*ports.ActiveGroup, error)
	CreateActiveGroup(ctx context.Context, group *ports.ActiveGroup) error
	UpdateActiveGroup(ctx context.Context, group *ports.ActiveGroup) error
	DeleteActiveGroup(ctx context.Context, id int64) error
	FindDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) (*ports.ActiveGroup, error)
	EndActiveGroupSession(ctx context.Context, id int64) error

	// Visit operations
	CreateVisit(ctx context.Context, visit *studentpresence.Visit) error
	UpdateVisit(ctx context.Context, visit *studentpresence.Visit) error
	DeleteVisit(ctx context.Context, id int64) error
	FindVisitsByStudentID(ctx context.Context, studentID int64) ([]studentpresence.Visit, error)
	EndVisit(ctx context.Context, id int64) error
	GetStudentCurrentVisit(ctx context.Context, studentID int64) (*studentpresence.Visit, error)
	GetStudentCurrentVisitWithRoom(ctx context.Context, studentID int64) (*VisitWithRoom, error)
	GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*studentpresence.Visit, error)
	CountActiveVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) (int, error)
	ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error)
	ListOpenVisitStudentIDsByRoom(ctx context.Context) (map[int64][]int64, error)
	ListStudentsInTransit(ctx context.Context) ([]int64, error)
	ListStudentsPresentToday(ctx context.Context) ([]int64, error)
	AssignTransitStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64) (*TransitAssignResult, error)
	AssignTransitStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth StudentMoveAuthorization) (*TransitAssignResult, error)
	MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth StudentMoveAuthorization) (*StudentMoveResult, error)
	MoveStudentsToTransitAuthorized(ctx context.Context, studentIDs []int64, auth StudentMoveAuthorization) (*StudentMoveResult, error)

	// Group Supervisor operations
	CreateGroupSupervisor(ctx context.Context, supervisor *ports.GroupSupervisor) error
	UpdateGroupSupervisor(ctx context.Context, supervisor *ports.GroupSupervisor) error
	DeleteGroupSupervisor(ctx context.Context, id int64) error
	EndSupervision(ctx context.Context, id int64) error

	// Combined Group operations
	CreateCombinedGroup(ctx context.Context, group *studentpresence.CombinedGroup) error
	UpdateCombinedGroup(ctx context.Context, group *studentpresence.CombinedGroup) error
	DeleteCombinedGroup(ctx context.Context, id int64) error
	EndCombinedGroup(ctx context.Context, id int64) error
	CreateCombinedGroupWithGroups(ctx context.Context, group *studentpresence.CombinedGroup, groupIDs []int64) error

	// Activity Session Management with Conflict Detection
	StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*ports.ActiveGroup, error)
	CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error)
	EndActivitySession(ctx context.Context, activeGroupID int64) error
	ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*ports.ActiveGroup, error)
	GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*ports.ActiveGroup, error)

	// Dynamic Supervisor Management
	UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*ports.ActiveGroup, error)

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
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*ports.ActiveGroup, error)

	// Attendance tracking operations
	GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*AttendanceStatus, error)
	// GetActiveGroupVisitsWithDisplay returns the open visits of an active
	// group joined with tenant-scoped student display data.
	GetActiveGroupVisitsWithDisplay(ctx context.Context, activeGroupID int64) ([]*VisitWithStudentDisplay, error)
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
	// ConfirmDailyCheckout processes a deferred daily-checkout confirmation for
	// an IoT device: it validates the student has today's attendance record and,
	// when destination is "zuhause" and the student is still checked in, checks
	// them out and broadcasts the SSE update. Returns
	// ErrNoAttendanceRecordForCheckout when no attendance record exists today.
	ConfirmDailyCheckout(ctx context.Context, studentID, deviceID int64, destination string) (*DailyCheckoutResult, error)

	// Unclaimed groups management (deviceless claiming)
	GetUnclaimedActiveGroups(ctx context.Context) ([]*ports.ActiveGroup, error)
	ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*ports.GroupSupervisor, error)

	// Cross-tenant student visibility (Ferienbetreuung / holiday care)
	GetCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]ports.CrossTenantStudent, error)

	// Tracking indicators — returns per-student match results for the given labels.
	// Each student gets a []bool aligned with the labels slice.
	GetTrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error)

	// GetPresenceMode resolves the tenant's presence mode ("detailed" | "binary").
	// Missing wiring, lookup failures and invalid values return an error.
	GetPresenceMode(ctx context.Context) (string, error)
}

// The transit and move results, the attendance and session results and the
// cleanup reports are the owner's public values.
type (
	TransitAssignSkipped      = studentpresence.TransitAssignSkipped
	TransitAssignResult       = studentpresence.TransitAssignResult
	StudentMoveSkipped        = studentpresence.StudentMoveSkipped
	StudentMoveResult         = studentpresence.StudentMoveResult
	StudentMoveAuthorization  = studentpresence.StudentMoveAuthorization
	TimeoutResult             = studentpresence.TimeoutResult
	SessionTimeoutInfo        = studentpresence.SessionTimeoutInfo
	CleanupResult             = studentpresence.CleanupResult
	CleanupError              = studentpresence.CleanupError
	RetentionStats            = studentpresence.RetentionStats
	CleanupPreview            = studentpresence.CleanupPreview
	AttendanceStatus          = studentpresence.DailyAttendanceStatus
	DailyCheckoutResult       = studentpresence.DailyCheckoutResult
	AttendanceResult          = studentpresence.AttendanceResult
	DailySessionCleanupResult = studentpresence.DailySessionCleanupResult
	AttendanceCleanupResult   = studentpresence.AttendanceCleanupResult
	AttendanceCleanupPreview  = studentpresence.AttendanceCleanupPreview
	SupervisorCleanupResult   = studentpresence.SupervisorCleanupResult
	SupervisorCleanupPreview  = studentpresence.SupervisorCleanupPreview
)

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
	// HomeCandidateIDs are the students StudentsHome counts: active, not
	// present, not sick or excused. The caller holding the day plan moves the
	// ones still in class to "Schule" (#3260). Not serialized.
	HomeCandidateIDs []int64

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
	MaxCapacity  *int
	Status       string
}

// ActiveGroupInfo represents active group summary
type ActiveGroupInfo struct {
	Name         string
	Type         string
	StudentCount int
	// MaxCapacity is the activity's limit; nil without one (#3634).
	MaxCapacity *int
	Location    string
	Status      string
}

// ActivityConflictInfo represents information about a detected activity conflict
type ActivityConflictInfo struct {
	HasConflict       bool               `json:"has_conflict"`
	ConflictingGroup  *ports.ActiveGroup `json:"conflicting_group,omitempty"`
	ConflictingDevice *string            `json:"conflicting_device,omitempty"`
	ConflictMessage   string             `json:"conflict_message"`
	CanOverride       bool               `json:"can_override"`
}

// CleanupService is the owner's public retention cleanup facade.
type CleanupService = studentpresence.PresenceCleanup
