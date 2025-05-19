package activities

import (
	"context"

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
	UpdateGroup(ctx context.Context, group *activities.Group) (*activities.Group, error)
	DeleteGroup(ctx context.Context, id int64) error
	ListGroups(ctx context.Context, filters map[string]interface{}) ([]*activities.Group, error)
	GetGroupWithDetails(ctx context.Context, id int64) (*activities.Group, []*activities.SupervisorPlanned, []*activities.Schedule, error)
	GetGroupsWithEnrollmentCounts(ctx context.Context) ([]*activities.Group, map[int64]int, error)

	// Schedule operations
	AddSchedule(ctx context.Context, groupID int64, schedule *activities.Schedule) (*activities.Schedule, error)
	GetSchedule(ctx context.Context, id int64) (*activities.Schedule, error)
	GetGroupSchedules(ctx context.Context, groupID int64) ([]*activities.Schedule, error)
	DeleteSchedule(ctx context.Context, id int64) error

	// Supervisor operations
	AddSupervisor(ctx context.Context, groupID int64, staffID int64, isPrimary bool) (*activities.SupervisorPlanned, error)
	GetSupervisor(ctx context.Context, id int64) (*activities.SupervisorPlanned, error)
	GetGroupSupervisors(ctx context.Context, groupID int64) ([]*activities.SupervisorPlanned, error)
	DeleteSupervisor(ctx context.Context, id int64) error
	SetPrimarySupervisor(ctx context.Context, id int64) error

	// Enrollment operations
	EnrollStudent(ctx context.Context, groupID, studentID int64) error
	UnenrollStudent(ctx context.Context, groupID, studentID int64) error
	GetEnrolledStudents(ctx context.Context, groupID int64) ([]*users.Student, error)
	GetStudentEnrollments(ctx context.Context, studentID int64) ([]*activities.Group, error)
	GetAvailableGroups(ctx context.Context, studentID int64) ([]*activities.Group, error)
	UpdateAttendanceStatus(ctx context.Context, enrollmentID int64, status *string) error

	// Public operations
	GetPublicGroups(ctx context.Context, categoryID *int64) ([]*activities.Group, map[int64]int, error)
	GetPublicCategories(ctx context.Context) ([]*activities.Category, error)
}
