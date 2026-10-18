package activitycontract

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ActivityService defines operations for activity management
type ActivityService interface {
	CategoryOperations
	GroupOperations
	ScheduleOperations
	SupervisorOperations
	EnrollmentOperations
	timetable.OccupancyQuery
}

type CategoryOperations interface {
	// Category operations
	CreateCategory(ctx context.Context, category *Category) (*Category, error)
	ListCategories(ctx context.Context) ([]*Category, error)
	// CategoryUsageCounts reports how many activity groups reference each
	// category, keyed by category id, for the Stammdaten management screen
	// (#2131). Separate from listing: the aggregate is an extra scan only
	// that screen needs.
	CategoryUsageCounts(ctx context.Context) (map[int64]int, error)
	// UpdateCategory renames a non-system, non-archived category.
	UpdateCategory(ctx context.Context, id int64, input CategoryInput) (*Category, error)
	// ArchiveCategory retires a category without deleting it, so existing
	// Termine and Aktivitäten keep resolving it.
	ArchiveCategory(ctx context.Context, id int64) (*Category, error)
	// RestoreCategory un-archives a category.
	RestoreCategory(ctx context.Context, id int64) (*Category, error)
	// SetCategoryShiftTypeLinks maps categories to a Dienstplan shift type and
	// clears the mapping on de-selected ones (#1837 follow-up).
	SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error
}

type GroupOperations interface {
	// Activity Group operations
	CreateGroup(ctx context.Context, group *Group, supervisorIDs []int64, schedules []*Schedule) (*Group, error)
	GetGroup(ctx context.Context, id int64) (*Group, error)
	UpdateGroup(ctx context.Context, group *Group, requestingStaffID int64, hasManagePermission bool) (*Group, error)
	// UpdateGroupWithDetails updates group fields + supervisor set + schedule
	// replacement as one failing-together unit; run inside a tenant tx so a
	// partial failure rolls everything back (issue #575 B10).
	UpdateGroupWithDetails(ctx context.Context, group *Group, requestingStaffID int64, hasManagePermission bool, supervisorIDs []int64, schedules []*Schedule) (*Group, error)
	DeleteGroup(ctx context.Context, id int64, requestingStaffID int64, hasManagePermission bool) error
	ListGroups(ctx context.Context, query *GroupListQuery) ([]*Group, error)
	GetGroupWithDetails(ctx context.Context, id int64) (*Group, []*SupervisorPlanned, []*Schedule, error)
	GetGroupsWithEnrollmentCounts(ctx context.Context) ([]*Group, map[int64]int, error)
	FindByCategory(ctx context.Context, categoryID int64) ([]*Group, error)

	// Permission check - returns true if user can modify (edit/delete) the activity
	// User can modify if: they created it, they are a supervisor, or they have manage permission
	CanModifyActivity(ctx context.Context, groupID int64, staffID int64, hasManagePermission bool) (bool, error)
}

type ScheduleOperations interface {
	// Schedule operations
	AddSchedule(ctx context.Context, groupID int64, schedule *Schedule) (*Schedule, error)
	GetSchedule(ctx context.Context, id int64) (*Schedule, error)
	GetGroupSchedules(ctx context.Context, groupID int64) ([]*Schedule, error)
	DeleteSchedule(ctx context.Context, id int64) error
	UpdateSchedule(ctx context.Context, schedule *Schedule) (*Schedule, error)
}

type SupervisorOperations interface {
	// Supervisor operations
	AddSupervisor(ctx context.Context, groupID int64, staffID int64, isPrimary bool) (*SupervisorPlanned, error)
	GetSupervisor(ctx context.Context, id int64) (*SupervisorPlanned, error)
	GetGroupSupervisors(ctx context.Context, groupID int64) ([]*SupervisorPlanned, error)
	GetSupervisorsForGroups(ctx context.Context, groupIDs []int64) (map[int64][]*SupervisorPlanned, error)
	DeleteSupervisor(ctx context.Context, id int64) error
	SetPrimarySupervisor(ctx context.Context, id int64) error
	UpdateSupervisor(ctx context.Context, supervisor *SupervisorPlanned) (*SupervisorPlanned, error)
	UpdateGroupSupervisors(ctx context.Context, groupID int64, staffIDs []int64) error
	ReplaceSupervisor(ctx context.Context, activityID, supervisorID int64, replacementStaffID *int64) error
}

type EnrollmentOperations interface {
	// Enrollment operations
	EnrollStudent(ctx context.Context, groupID, studentID int64) error
	UnenrollStudent(ctx context.Context, groupID, studentID int64) error
	UpdateGroupEnrollments(ctx context.Context, groupID int64, studentIDs []int64) error
	GetEnrolledStudents(ctx context.Context, groupID int64) ([]*timetable.OperationStudent, error)
	GetStudentEnrollments(ctx context.Context, studentID int64) ([]*Group, error)
	GetActiveStudentEnrollmentsByStudentIDs(ctx context.Context, studentIDs []int64, onDate calendar.Date) (map[int64][]*Group, error)
	GetAvailableGroups(ctx context.Context, studentID int64) ([]*Group, error)
}
