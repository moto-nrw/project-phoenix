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
