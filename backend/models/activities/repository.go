package activities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// CategoryRepository defines operations for managing activity categories
type CategoryRepository interface {
	base.Repository[*Category]

	// FindByName finds a category by its name
	FindByName(ctx context.Context, name string) (*Category, error)

	// ListAll returns all categories
	ListAll(ctx context.Context) ([]*Category, error)
}

// GroupRepository defines operations for managing activity groups
type GroupRepository interface {
	base.Repository[*Group]

	// FindByCategory finds all groups in a specific category
	FindByCategory(ctx context.Context, categoryID int64) ([]*Group, error)

	// FindOpenGroups finds all groups that are open for enrollment
	FindOpenGroups(ctx context.Context) ([]*Group, error)

	// FindWithEnrollmentCounts returns groups with their current enrollment counts
	FindWithEnrollmentCounts(ctx context.Context) ([]*Group, map[int64]int, error)

	// FindWithSupervisors returns a group with its supervisors
	FindWithSupervisors(ctx context.Context, groupID int64) (*Group, []*SupervisorPlanned, error)

	// FindWithSchedules returns a group with its scheduled times
	FindWithSchedules(ctx context.Context, groupID int64) (*Group, []*Schedule, error)

	// FindByStaffSupervisor finds all activity groups where a staff member is a supervisor
	FindByStaffSupervisor(ctx context.Context, staffID int64) ([]*Group, error)

	// FindByStaffSupervisorToday finds all activity groups where a staff member is a supervisor for today
	FindByStaffSupervisorToday(ctx context.Context, staffID int64) ([]*Group, error)
}

// ScheduleRepository defines operations for managing activity schedules
type ScheduleRepository interface {
	base.Repository[*Schedule]

	// FindByGroupID finds all schedules for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*Schedule, error)

	// FindByWeekday finds all schedules for a specific weekday
	FindByWeekday(ctx context.Context, weekday string) ([]*Schedule, error)

	// FindByTimeframeID finds all schedules for a specific timeframe
	FindByTimeframeID(ctx context.Context, timeframeID int64) ([]*Schedule, error)
}

// SupervisorPlannedRepository defines operations for managing activity supervisors
type SupervisorPlannedRepository interface {
	base.Repository[*SupervisorPlanned]

	// FindByStaffID finds all supervisions for a specific staff member
	FindByStaffID(ctx context.Context, staffID int64) ([]*SupervisorPlanned, error)

	// FindByGroupID finds all supervisors for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*SupervisorPlanned, error)

	// FindPrimaryByGroupID finds the primary supervisor for a specific group
	FindPrimaryByGroupID(ctx context.Context, groupID int64) (*SupervisorPlanned, error)

	// SetPrimary sets a supervisor as the primary supervisor for a group
	SetPrimary(ctx context.Context, id int64) error
}

// StudentEnrollmentRepository defines operations for managing student enrollments
type StudentEnrollmentRepository interface {
	base.Repository[*StudentEnrollment]

	// FindByStudentID finds all enrollments for a specific student
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentEnrollment, error)

	// FindByGroupID finds all enrollments for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*StudentEnrollment, error)

	// CountByGroupID counts the number of students enrolled in a specific group
	CountByGroupID(ctx context.Context, groupID int64) (int, error)

	// FindByEnrollmentDateRange finds enrollments within a date range
	FindByEnrollmentDateRange(ctx context.Context, start, end time.Time) ([]*StudentEnrollment, error)

	// UpdateAttendanceStatus updates the attendance status for a specific enrollment
	UpdateAttendanceStatus(ctx context.Context, id int64, status *string) error
}
