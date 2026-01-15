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
	GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error)

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
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*active.Group, error)

	// Attendance tracking operations
	GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*AttendanceStatus, error)
	GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*AttendanceStatus, error)
	ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*AttendanceResult, error)
	CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error)

	// Unclaimed groups management (deviceless claiming)
	GetUnclaimedActiveGroups(ctx context.Context) ([]*active.Group, error)
	ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*active.GroupSupervisor, error)

	// Visit display operations
	GetVisitsWithDisplayData(ctx context.Context, activeGroupID int64) ([]VisitWithDisplayData, error)
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
