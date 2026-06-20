package activities

import (
	"context"
	"database/sql"
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

	// ListTemplateRows returns the template list read model (template +
	// schedule + aggregate people counts), optionally filtered to one
	// template. Issue #584: the aggregation query moved verbatim out of
	// api/timetable; its multi-schema joins cannot be expressed through the
	// generic repository shape.
	ListTemplateRows(ctx context.Context, templateID *int64) ([]TemplateListRow, error)

	// ListTemplateRowsForPeriod is the calendar-period-filtered variant used
	// by the template list endpoint (people aggregates and schedules filter
	// on the period when given).
	ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]TemplateListRow, error)

	// UpdateTemplateFields patches the editable template fields of a
	// non-archived template row.
	UpdateTemplateFields(ctx context.Context, id int64, name, groupType string, categoryID, roomID int64, educationGroupID *int64, maxParticipants int) (rowsAffected int64, err error)

	// ArchiveTemplate soft-deletes a non-archived template (sets archived_at).
	ArchiveTemplate(ctx context.Context, id int64) (rowsAffected int64, err error)

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

	// DeleteByGroupID removes all schedules of an activity group.
	DeleteByGroupID(ctx context.Context, groupID int64) error

	// FindTemplateStartTimesByGroupIDs returns (activity_group_id, weekday,
	// timeframe.start_time) tuples for the given group IDs. Joins
	// activities.schedules to schedule.timeframes and filters out rows
	// without a linked timeframe. Both tables are tenant-scoped in the
	// WHERE clause as defense-in-depth alongside RLS. Returns an empty
	// slice when groupIDs is empty.
	FindTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*TemplateStartTime, error)

	// CapValidUntil ends the recurrence of every schedule of the given
	// template at validUntil (exclusive): rows that are open-ended
	// (valid_until IS NULL) or end later than validUntil are capped to
	// validUntil. Returns the number of rows changed. Used by the template
	// split ("Dieser und alle folgenden", WP-B3).
	CapValidUntil(ctx context.Context, activityGroupID int64, validUntil timezone.Date) (int64, error)
}

// SupervisorPlannedRepository defines operations for managing activity supervisors
type SupervisorPlannedRepository interface {
	base.Repository[*SupervisorPlanned]

	// ListPlannedSupervisionBlockers returns planned activity supervisions
	// as caregiver-capability blocker rows.
	ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]users.BlockerActivity, error)

	// CloseOpenByGroupAndPeriod closes the open planned supervisions of a
	// group for the given calendar period (NULL period matches rows without
	// one) by setting valid_until.
	CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error

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

	// DeleteByStaffID removes all planned supervisions for a staff member
	// (staff offboarding cleanup).
	DeleteByStaffID(ctx context.Context, staffID int64) (int64, error)

	// CapActiveByGroup ends every still-active supervision (valid_until IS
	// NULL) of the given group at validUntil (exclusive). Returns the number
	// of rows changed. Used by the template split (WP-B3).
	CapActiveByGroup(ctx context.Context, groupID int64, validUntil timezone.Date) (int64, error)
}

// StudentEnrollmentRepository defines operations for managing student enrollments
type StudentEnrollmentRepository interface {
	base.Repository[*StudentEnrollment]

	// CloseOpenByGroupAndPeriod closes the open enrollments of a group for
	// the given calendar period (NULL period matches rows without one) by
	// setting valid_until.
	CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom timezone.Date) error

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

	// DeleteByStudentGroupsAndWindow removes enrollments for one student,
	// a bounded set of activity groups, and an exact validity window.
	DeleteByStudentGroupsAndWindow(ctx context.Context, studentID int64, groupIDs []int64, validFrom timezone.Date, validUntil *timezone.Date) (int64, error)

	// CapActiveByGroup ends every still-active enrollment (valid_until IS
	// NULL) of the given group at validUntil (exclusive). Returns the number
	// of rows changed. Used by the template split (WP-B3).
	CapActiveByGroup(ctx context.Context, groupID int64, validUntil timezone.Date) (int64, error)
}

// TemplateListRow is one row of the template list read model produced by
// GroupRepository.ListTemplateRows: template fields joined with one schedule
// row and the aggregated people counts. Issue #584: moved verbatim from
// api/timetable.
type TemplateListRow struct {
	TemplateID         int64          `bun:"template_id"`
	Name               string         `bun:"name"`
	Type               string         `bun:"type"`
	CategoryID         int64          `bun:"category_id"`
	CategoryName       string         `bun:"category_name"`
	RoomID             sql.NullInt64  `bun:"room_id"`
	RoomName           sql.NullString `bun:"room_name"`
	EducationGroupID   sql.NullInt64  `bun:"education_group_id"`
	EducationGroupName sql.NullString `bun:"education_group_name"`
	IsOpen             bool           `bun:"is_open"`
	MaxParticipants    int            `bun:"max_participants"`
	EnrollmentCount    int            `bun:"enrollment_count"`
	SupervisorCount    int            `bun:"supervisor_count"`
	StudentIDs         []int64        `bun:"student_ids,array"`
	StaffIDs           []int64        `bun:"staff_ids,array"`
	PrimaryStaffID     sql.NullInt64  `bun:"primary_staff_id"`
	ScheduleID         int64          `bun:"schedule_id"`
	Weekday            int            `bun:"weekday"`
	StartTime          sql.NullString `bun:"start_time"`
	EndTime            sql.NullString `bun:"end_time"`
	WeekPattern        int            `bun:"week_pattern"`
	CalendarPeriodID   sql.NullInt64  `bun:"calendar_period_id"`
	ScheduleValidUntil sql.NullString `bun:"schedule_valid_until"`
}
