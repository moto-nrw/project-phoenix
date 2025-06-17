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
	CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*ActivityConflictInfo, error)
	EndActivitySession(ctx context.Context, activeGroupID int64) error
	ForceStartActivitySession(ctx context.Context, activityID, deviceID, staffID int64, roomID *int64) (*active.Group, error)
	GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*active.Group, error)

	// Session timeout operations
	ProcessSessionTimeout(ctx context.Context, deviceID int64) (*TimeoutResult, error)
	UpdateSessionActivity(ctx context.Context, activeGroupID int64) error
	ValidateSessionTimeout(ctx context.Context, deviceID int64, timeoutMinutes int) error
	GetSessionTimeoutInfo(ctx context.Context, deviceID int64) (*SessionTimeoutInfo, error)
	CleanupAbandonedSessions(ctx context.Context, olderThan time.Duration) (int, error)

	// Analytics and statistics
	GetActiveGroupsCount(ctx context.Context) (int, error)
	GetTotalVisitsCount(ctx context.Context) (int, error)
	GetActiveVisitsCount(ctx context.Context) (int, error)
	GetRoomUtilization(ctx context.Context, roomID int64) (float64, error)
	GetStudentAttendanceRate(ctx context.Context, studentID int64) (float64, error)
	GetDashboardAnalytics(ctx context.Context) (*DashboardAnalytics, error)
}

// DashboardAnalytics represents aggregated analytics for dashboard
type DashboardAnalytics struct {
	// Student Overview
	StudentsPresent      int
	StudentsEnrolled     int
	StudentsOnPlayground int
	StudentsInTransit    int

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
	StudentVisitCounts map[int64]int  // Student ID -> number of visits to delete
	TotalVisits        int64
	OldestVisit        *time.Time
}
