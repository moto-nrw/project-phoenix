package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Service defines operations for managing active groups and visits
type Service interface {
	base.TransactionalService

	// Active Group operations
	GetActiveGroup(ctx context.Context, id int64) (*active.Group, error)
	CreateActiveGroup(ctx context.Context, group *active.Group) error
	UpdateActiveGroup(ctx context.Context, group *active.Group) error
	DeleteActiveGroup(ctx context.Context, id int64) error
	ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error)
	FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error)
	FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error)
	FindActiveGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error)
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
	FindVisitsByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error)
	EndVisit(ctx context.Context, id int64) error
	GetStudentCurrentVisit(ctx context.Context, studentID int64) (*active.Visit, error)

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

	// Group Mapping operations
	AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error
	RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error
	GetGroupMappingsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error)
	GetGroupMappingsByCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error)

	// Activity Session Management with Conflict Detection
	StartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error)
	StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*active.Group, error)
	CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error)
	EndActivitySession(ctx context.Context, activeGroupID int64) error
	ForceStartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error)
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
	GetActiveGroupsCount(ctx context.Context) (int, error)
	GetTotalVisitsCount(ctx context.Context) (int, error)
	GetActiveVisitsCount(ctx context.Context) (int, error)
	GetRoomUtilization(ctx context.Context, roomID int64) (float64, error)
	GetStudentAttendanceRate(ctx context.Context, studentID int64) (float64, error)
	GetDashboardAnalytics(ctx context.Context) (*DashboardAnalytics, error)

	// Attendance tracking operations
	GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*AttendanceStatus, error)
	ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64) (*AttendanceResult, error)
	CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error)

	// Scheduled checkout operations
	CreateScheduledCheckout(ctx context.Context, checkout *active.ScheduledCheckout) error
	GetScheduledCheckout(ctx context.Context, id int64) (*active.ScheduledCheckout, error)
	GetPendingScheduledCheckout(ctx context.Context, studentID int64) (*active.ScheduledCheckout, error)
	CancelScheduledCheckout(ctx context.Context, id int64, cancelledBy int64) error
	ProcessDueScheduledCheckouts(ctx context.Context) (*ScheduledCheckoutResult, error)
	GetStudentScheduledCheckouts(ctx context.Context, studentID int64) ([]*active.ScheduledCheckout, error)

	// Unclaimed groups management (deviceless claiming)
	GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error)
	ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error)
}

// DashboardAnalytics represents aggregated analytics for dashboard
type DashboardAnalytics struct {
	// Student Overview
	StudentsPresent      int
	StudentsInTransit    int // Students present but not in any active visit
	StudentsOnPlayground int
	StudentsInRooms      int // Students in indoor rooms (excluding playground)

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

// TimeoutResult represents the result of processing a session timeout
type TimeoutResult struct {
	SessionID          int64     `json:"session_id"`
	ActivityID         int64     `json:"activity_id"`
	StudentsCheckedOut int       `json:"students_checked_out"`
	TimeoutAt          time.Time `json:"timeout_at"`
}

// SessionTimeoutInfo provides information about a session's timeout status
type SessionTimeoutInfo struct {
	SessionID          int64         `json:"session_id"`
	ActivityID         int64         `json:"activity_id"`
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

	// CleanupVisitsForStudent runs cleanup for a specific student
	CleanupVisitsForStudent(ctx context.Context, studentID int64) (int64, error)

	// GetRetentionStatistics gets statistics about data that will be deleted
	GetRetentionStatistics(ctx context.Context) (*RetentionStats, error)

	// PreviewCleanup shows what would be deleted without actually deleting
	PreviewCleanup(ctx context.Context) (*CleanupPreview, error)

	// CleanupStaleAttendance closes attendance records from previous days
	CleanupStaleAttendance(ctx context.Context) (*AttendanceCleanupResult, error)

	// PreviewAttendanceCleanup shows what attendance records would be cleaned
	PreviewAttendanceCleanup(ctx context.Context) (*AttendanceCleanupPreview, error)
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
	StudentID    int64      `json:"student_id"`
	Status       string     `json:"status"` // "not_checked_in", "checked_in", "checked_out"
	Date         time.Time  `json:"date"`
	CheckInTime  *time.Time `json:"check_in_time"`
	CheckOutTime *time.Time `json:"check_out_time"`
	CheckedInBy  string     `json:"checked_in_by"`  // Formatted as "FirstName LastName"
	CheckedOutBy string     `json:"checked_out_by"` // Formatted as "FirstName LastName"
}

// AttendanceResult represents the result of a student attendance toggle operation
type AttendanceResult struct {
	Action       string    `json:"action"` // "checked_in", "checked_out"
	AttendanceID int64     `json:"attendance_id"`
	StudentID    int64     `json:"student_id"`
	Timestamp    time.Time `json:"timestamp"`
}

// DailySessionCleanupResult represents the result of ending daily sessions
type DailySessionCleanupResult struct {
	SessionsEnded    int       `json:"sessions_ended"`
	VisitsEnded      int       `json:"visits_ended"`
	SupervisorsEnded int       `json:"supervisors_ended"`
	ExecutedAt       time.Time `json:"executed_at"`
	Success          bool      `json:"success"`
	Errors           []string  `json:"errors,omitempty"`
}

// AttendanceCleanupResult represents the result of cleaning stale attendance records
type AttendanceCleanupResult struct {
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      time.Time  `json:"completed_at"`
	RecordsClosed    int        `json:"records_closed"`
	StudentsAffected int        `json:"students_affected"`
	OldestRecordDate *time.Time `json:"oldest_record_date,omitempty"`
	Success          bool       `json:"success"`
	Errors           []string   `json:"errors,omitempty"`
}

// AttendanceCleanupPreview shows what attendance records would be cleaned
type AttendanceCleanupPreview struct {
	TotalRecords   int            `json:"total_records"`
	StudentRecords map[int64]int  `json:"student_records"` // studentID -> count
	OldestRecord   *time.Time     `json:"oldest_record,omitempty"`
	RecordsByDate  map[string]int `json:"records_by_date"` // date -> count
}

// ScheduledCheckoutResult represents the result of processing scheduled checkouts
type ScheduledCheckoutResult struct {
	ProcessedAt       time.Time `json:"processed_at"`
	CheckoutsExecuted int       `json:"checkouts_executed"`
	VisitsEnded       int       `json:"visits_ended"`
	AttendanceUpdated int       `json:"attendance_updated"`
	Errors            []string  `json:"errors,omitempty"`
	Success           bool      `json:"success"`
}
