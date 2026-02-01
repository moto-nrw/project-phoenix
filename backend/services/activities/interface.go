package activities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// ActivityService defines operations for activity management
type ActivityService interface {
	base.TransactionalService

	// Category operations
	CreateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error)
	GetCategory(ctx context.Context, id int64) (*activities.Category, error)
	UpdateCategory(ctx context.Context, category *activities.Category) (*activities.Category, error)
	DeleteCategory(ctx context.Context, id int64) error
	ListCategories(ctx context.Context) ([]*activities.Category, error)

	// Activity Group operations
	CreateGroup(ctx context.Context, group *activities.Group, supervisorIDs []int64, schedules []*activities.Schedule) (*activities.Group, error)
	GetGroup(ctx context.Context, id int64) (*activities.Group, error)
	UpdateGroup(ctx context.Context, group *activities.Group, requestingStaffID int64, hasManagePermission bool) (*activities.Group, error)
	DeleteGroup(ctx context.Context, id int64, requestingStaffID int64, hasManagePermission bool) error
	ListGroups(ctx context.Context, queryOptions *base.QueryOptions) ([]*activities.Group, error)
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
	GetStaffAssignments(ctx context.Context, staffID int64) ([]*activities.SupervisorPlanned, error)
	UpdateGroupSupervisors(ctx context.Context, groupID int64, staffIDs []int64) error

	// Enrollment operations
	EnrollStudent(ctx context.Context, groupID, studentID int64) error
	UnenrollStudent(ctx context.Context, groupID, studentID int64) error
	UpdateGroupEnrollments(ctx context.Context, groupID int64, studentIDs []int64) error
	GetEnrolledStudents(ctx context.Context, groupID int64) ([]*users.Student, error)
	GetStudentEnrollments(ctx context.Context, studentID int64) ([]*activities.Group, error)
	GetAvailableGroups(ctx context.Context, studentID int64) ([]*activities.Group, error)
	UpdateAttendanceStatus(ctx context.Context, enrollmentID int64, status *string) error
	GetEnrollmentsByDate(ctx context.Context, date time.Time) ([]*activities.StudentEnrollment, error)
	GetEnrollmentHistory(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*activities.StudentEnrollment, error)

	// Public operations
	GetPublicGroups(ctx context.Context, categoryID *int64) ([]*activities.Group, map[int64]int, error)
	GetPublicCategories(ctx context.Context) ([]*activities.Category, error)
	GetOpenGroups(ctx context.Context) ([]*activities.Group, error)

	// Device operations for RFID teacher selection
	GetTeacherTodaysActivities(ctx context.Context, staffID int64) ([]*activities.Group, error)
}
