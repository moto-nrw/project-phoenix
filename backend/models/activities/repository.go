package activities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
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

	// FindByName finds a non-archived activity group by its name.
	FindByName(ctx context.Context, name string) (*Group, error)

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

	// FindAllTemplates returns all activity groups flagged as templates
	// (is_template = true). Used by the materialization service to enumerate
	// candidates when generating schedule.activity_instances rows.
	FindAllTemplates(ctx context.Context) ([]*Group, error)
}

// TemplateStartTime is a (activity_group_id, weekday) → timeframe.start_time
// lookup row returned by ScheduleRepository.FindTemplateStartTimesByGroupIDs.
// Used by the WP-B13 exception-conflict endpoint to resolve the "original"
// start_time for modified exceptions.
//
// Multiple rows may share the same (ActivityGroupID, Weekday) when a template
// has several schedules on the same weekday (e.g. morning + afternoon slots).
// The caller is responsible for disambiguating or flagging ambiguity. Rows
// are returned sorted by (ActivityGroupID ASC, Weekday ASC, StartTime ASC),
// so the earliest slot comes first when multiple exist for a given key.
type TemplateStartTime struct {
	ActivityGroupID int64     `bun:"activity_group_id"`
	Weekday         int       `bun:"weekday"`
	StartTime       time.Time `bun:"start_time"`
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

	// FindTemplateStartTimesByGroupIDs returns (activity_group_id, weekday,
	// timeframe.start_time) tuples for the given group IDs. Joins
	// activities.schedules to schedule.timeframes and filters out rows
	// without a linked timeframe. Both tables are tenant-scoped in the
	// WHERE clause as defense-in-depth alongside RLS. Returns an empty
	// slice when groupIDs is empty.
	FindTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*TemplateStartTime, error)
}

// SupervisorPlannedRepository defines operations for managing activity supervisors
type SupervisorPlannedRepository interface {
	base.Repository[*SupervisorPlanned]

	// ListPlannedSupervisionBlockers returns planned activity supervisions
	// as caregiver-capability blocker rows.
	ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]users.BlockerActivity, error)

	// FindByStaffID finds all supervisions for a specific staff member
	FindByStaffID(ctx context.Context, staffID int64) ([]*SupervisorPlanned, error)

	// FindByGroupID finds all supervisors for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*SupervisorPlanned, error)

	// FindByGroupIDs finds all supervisors for multiple groups in a single query
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*SupervisorPlanned, error)

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

	// FindByValidFromRange finds enrollments within a valid_from date range
	FindByValidFromRange(ctx context.Context, start, end timezone.Date) ([]*StudentEnrollment, error)

	// UpdateAttendanceStatus updates the attendance status for a specific enrollment
	UpdateAttendanceStatus(ctx context.Context, id int64, status *string) error
}
