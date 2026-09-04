package legacy

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// ActivityGroupWithOccupancy wraps an activity group with its occupancy status
type ActivityGroupWithOccupancy struct {
	*activities.Group
	IsOccupied bool
}

// ActivityService defines operations for activity management
type ActivityService interface {
	// Category operations
	CreateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error)
	GetCategory(ctx context.Context, id int64) (*activities.Category, error)
	ListCategories(ctx context.Context) ([]*activities.Category, error)
	// CategoryUsageCounts reports how many activity groups reference each
	// category, keyed by category id, for the Stammdaten management screen
	// (#2131). Separate from listing: the aggregate is an extra scan only
	// that screen needs.
	CategoryUsageCounts(ctx context.Context) (map[int64]int, error)
	// UpdateCategory renames a non-system, non-archived category.
	UpdateCategory(ctx context.Context, id int64, input CategoryInput) (*activities.Category, error)
	// ArchiveCategory retires a category without deleting it, so existing
	// Termine and Aktivitäten keep resolving it.
	ArchiveCategory(ctx context.Context, id int64) (*activities.Category, error)
	// RestoreCategory un-archives a category.
	RestoreCategory(ctx context.Context, id int64) (*activities.Category, error)
	// SetCategoryShiftTypeLinks maps categories to a Dienstplan shift type and
	// clears the mapping on de-selected ones (#1837 follow-up).
	SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error

	// Activity Group operations
	CreateGroup(ctx context.Context, group *activities.Group, supervisorIDs []int64, schedules []*activities.Schedule) (*activities.Group, error)
	GetGroup(ctx context.Context, id int64) (*activities.Group, error)
	UpdateGroup(ctx context.Context, group *activities.Group, requestingStaffID int64, hasManagePermission bool) (*activities.Group, error)
	// UpdateGroupWithDetails updates group fields + supervisor set + schedule
	// replacement as one failing-together unit; run inside a tenant tx so a
	// partial failure rolls everything back (issue #575 B10).
	UpdateGroupWithDetails(ctx context.Context, group *activities.Group, requestingStaffID int64, hasManagePermission bool, supervisorIDs []int64, schedules []*activities.Schedule) (*activities.Group, error)
	DeleteGroup(ctx context.Context, id int64, requestingStaffID int64, hasManagePermission bool) error
	ListGroups(ctx context.Context, query *activities.GroupListQuery) ([]*activities.Group, error)
	GetGroupWithDetails(ctx context.Context, id int64) (*activities.Group, []*activities.SupervisorPlanned, []*activities.Schedule, error)
	GetGroupsWithEnrollmentCounts(ctx context.Context) ([]*activities.Group, map[int64]int, error)
	FindByCategory(ctx context.Context, categoryID int64) ([]*activities.Group, error)

	// Permission check - returns true if user can modify (edit/delete) the activity
	// User can modify if: they created it, they are a supervisor, or they have manage permission
	CanModifyActivity(ctx context.Context, groupID int64, staffID int64, hasManagePermission bool) (bool, error)

	// Schedule operations
	AddSchedule(ctx context.Context, groupID int64, schedule *activities.Schedule) (*activities.Schedule, error)
	GetSchedule(ctx context.Context, id int64) (*activities.Schedule, error)
	GetGroupSchedules(ctx context.Context, groupID int64) ([]*activities.Schedule, error)
	DeleteSchedule(ctx context.Context, id int64) error
	UpdateSchedule(ctx context.Context, schedule *activities.Schedule) (*activities.Schedule, error)

	// Supervisor operations
	AddSupervisor(ctx context.Context, groupID int64, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error)
	GetSupervisor(ctx context.Context, id int64) (*activities.SupervisorPlanned, error)
	GetGroupSupervisors(ctx context.Context, groupID int64) ([]*activities.SupervisorPlanned, error)
	GetSupervisorsForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*activities.SupervisorPlanned, error)
	DeleteSupervisor(ctx context.Context, id int64) error
	SetPrimarySupervisor(ctx context.Context, id int64) error
	UpdateSupervisor(ctx context.Context, supervisor *activities.SupervisorPlanned) (*activities.SupervisorPlanned, error)
	UpdateGroupSupervisors(ctx context.Context, groupID int64, staffIDs []int64) error
	ReplaceSupervisor(ctx context.Context, activityID, supervisorID int64, replacementStaffID *int64) error

	// Enrollment operations
	EnrollStudent(ctx context.Context, groupID, studentID int64) error
	UnenrollStudent(ctx context.Context, groupID, studentID int64) error
	UpdateGroupEnrollments(ctx context.Context, groupID int64, studentIDs []int64) error
	GetEnrolledStudents(ctx context.Context, groupID int64) ([]*users.Student, error)
	GetStudentEnrollments(ctx context.Context, studentID int64) ([]*activities.Group, error)
	GetActiveStudentEnrollmentsByStudentIDs(ctx context.Context, studentIDs []int64, onDate timezone.Date) (map[int64][]*activities.Group, error)
	GetAvailableGroups(ctx context.Context, studentID int64) ([]*activities.Group, error)

	// Public operations

	// Device operations for RFID teacher selection

	// ListGroupsWithOccupancy returns all activity groups with their active session status
	ListGroupsWithOccupancy(ctx context.Context) ([]ActivityGroupWithOccupancy, error)
}
