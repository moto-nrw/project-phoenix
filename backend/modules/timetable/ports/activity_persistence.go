package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	activityrecords "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/activityrecords"
)

// Native activity persistence capabilities are bounded to their record and
// query shapes. There is no generic CRUD or query-options protocol.
type ActivityCategoryRecords interface {
	Create(context.Context, *activityContract.Category) error
	FindByID(context.Context, int64) (*activityContract.Category, error)
	Update(context.Context, *activityContract.Category) error
	Delete(context.Context, int64) error

	// FindByName finds a category by its name
	FindByName(ctx context.Context, name string) (*activityContract.Category, error)

	// FindByNameIncludingArchivedForShare finds a category by name across active
	// and archived rows, preferring the active row and holding a transaction-
	// scoped FOR SHARE lock against a concurrent archive.
	FindByNameIncludingArchivedForShare(ctx context.Context, name string) (*activityContract.Category, error)

	// FindByIDForShare returns a category while holding a transaction-scoped
	// FOR SHARE lock. New category assignments use it so an archive cannot
	// commit between validation and the referencing write.
	FindByIDForShare(ctx context.Context, id int64) (*activityContract.Category, error)

	// ListAll returns all categories
	ListAll(ctx context.Context) ([]*activityContract.Category, error)

	// UpdateColumns is the generic partial-update helper promoted from the
	// embedded base repository: updates only the named columns by primary key
	// and returns the number of rows affected. Archive/restore writes just
	// archived_at through it (#2131), so a concurrent rename is never
	// clobbered and a restore still surfaces the case-insensitive partial
	// unique-index violation on its own.
	UpdateColumns(ctx context.Context, category *activityContract.Category, columns ...string) (int64, error)
}

type ActivityGroupRecords interface {
	ListAll(context.Context) ([]*activityContract.Group, error)
	Create(context.Context, *activityContract.Group) error
	FindByID(context.Context, int64) (*activityContract.Group, error)
	Update(context.Context, *activityContract.Group) error
	Delete(context.Context, int64) error
	// ListWithCategory returns groups and their category from one LEFT JOIN.
	ListWithCategory(ctx context.Context, query *activityContract.GroupListQuery) ([]*activityContract.Group, error)

	// FindByIDForUpdate locks the group until the surrounding transaction ends.
	// It is used when modification authority must remain valid through a write.
	FindByIDForUpdate(ctx context.Context, id int64) (*activityContract.Group, error)

	// FindByName finds a non-archived activity group by its name.
	FindByName(ctx context.Context, name string) (*activityContract.Group, error)

	// FindByIDs returns the groups matching the given IDs in one
	// tenant-scoped IN query. Archived groups are included — callers resolve
	// display names for historical references (matching FindByID). Empty
	// input returns an empty slice without hitting the DB.
	FindByIDs(ctx context.Context, ids []int64) ([]*activityContract.Group, error)

	// ListTemplateRows returns the template list read model (template +
	// schedule + aggregate people counts), optionally filtered to one
	// template. Issue #584: the aggregation query moved verbatim out of
	// api/timetable; its multi-schema joins cannot be expressed through the
	// generic repository shape.
	ListTemplateRows(ctx context.Context, templateID *int64) ([]activityrecords.TemplateListRow, error)

	// ListTemplateRowsForTemplatePeriod returns the editable detail read model
	// for one template, scoped to one calendar period.
	ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) ([]activityrecords.TemplateListRow, error)

	// ListTemplateRowsForPeriod is the calendar-period-filtered variant used
	// by the template list endpoint (people aggregates and schedules filter
	// on the period when given).
	ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activityrecords.TemplateListRow, error)

	// ListTemplateWeekdayRoster returns the weekday-scoped roster rows of the
	// current roster (open rows, plus still-running phase-bounded student rows
	// on offering-sourced templates) of every non-archived template, optionally narrowed
	// to one template and calendar period (issue #2129). A concrete period also
	// includes series-wide rows whose calendar_period_id is NULL; a nil period
	// returns only unscoped rows and never merges several periods. It includes
	// empty markers for scheduled weekdays and expands every applicable
	// series-wide roster row onto the weekdays where it materializes.
	ListTemplateWeekdayRoster(ctx context.Context, templateID, calendarPeriodID *int64) ([]timetable.TemplateWeekdayRosterRow, error)

	// ListTemplateCapacityOccurrences returns one staffing snapshot per actual
	// recurrence date in periodID. It applies the same schedule, A/B-week,
	// cancellation, roster-validity, period, and selected-weekday rules as
	// materialization. Choosing which occurrence is worst depends on the
	// tenant's staffing ratio and therefore remains a service concern.
	// A nil periodID evaluates all active periods, used by template detail
	// responses that have no period query parameter.
	ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64) ([]activityrecords.TemplateCapacityOccurrence, error)

	// UpdateTemplateFields patches the editable template fields of a
	// non-archived template row.
	UpdateTemplateFields(ctx context.Context, id int64, fields timetable.TemplateUpdate) (rowsAffected int64, err error)

	// ArchiveTemplate soft-deletes a non-archived template (sets archived_at).
	ArchiveTemplate(ctx context.Context, id int64) (rowsAffected int64, err error)

	// FindOpenGroups finds all groups that are open for enrollment
	FindOpenGroups(ctx context.Context) ([]*activityContract.Group, error)

	// FindWithEnrollmentCounts returns groups with their current enrollment counts
	FindWithEnrollmentCounts(ctx context.Context) ([]*activityContract.Group, map[int64]int, error)

	// FindWithSupervisors returns a group with its supervisors
	FindWithSupervisors(ctx context.Context, groupID int64) (*activityContract.Group, []*activityContract.SupervisorPlanned, error)

	// FindByStaffSupervisor finds all activity groups where a staff member is a supervisor
	FindByStaffSupervisor(ctx context.Context, staffID int64) ([]*activityContract.Group, error)

	// FindAllTemplates returns all activity groups flagged as templates
	// (is_template = true). Used by the materialization service to enumerate
	// candidates when generating schedule.activity_instances rows.
	FindAllTemplates(ctx context.Context) ([]*activityContract.Group, error)

	// FindTemplateSeries returns every non-archived template segment in the
	// same split lineage as groupID. An unsplit template is one segment.
	FindTemplateSeries(ctx context.Context, groupID int64) ([]*activityContract.Group, error)

	// UpdateTemplateOfferingSource rewrites ONLY a template's offering-source
	// columns: the id array plus the Jahrgang filter, both NULLed when the
	// id list is empty (the DB CHECK forbids a filter without a source). The
	// detach flow uses it when a care offering is deleted — the jsonb array
	// carries no FK, so removing the deleted id from every sourcing template
	// is application-level work now (multi-source follow-up to #2137). Not
	// expressible via the generic Repository[T] update because both jsonb
	// columns must change atomically under the CHECK constraint.
	UpdateTemplateOfferingSource(ctx context.Context, id int64, offeringIDs []int64, gradeLevels []int, schoolClasses []string) error
}

type ActivityGroupTargets interface {
	ReplaceTargets(ctx context.Context, groupID int64, targets []*activityContract.GroupTarget) error
	FindTargetsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]*activityContract.GroupTarget, error)
	FindTargetStudentIDs(ctx context.Context, groupID int64) ([]int64, error)
	FindTargetStudentIDsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]int64, error)
}

type ActivityScheduleRecords interface {
	Create(context.Context, *activityContract.Schedule) error
	FindByID(context.Context, int64) (*activityContract.Schedule, error)
	Update(context.Context, *activityContract.Schedule) error
	Delete(context.Context, int64) error
	FindByGroupIDs(context.Context, []int64) ([]*activityContract.Schedule, error)

	// FindByGroupID finds all schedules for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.Schedule, error)

	// DeleteByGroupID removes all schedules of an activity group.
	DeleteByGroupID(ctx context.Context, groupID int64) error

	// FindTemplateStartTimesByGroupIDs returns (activity_group_id, weekday,
	// timeframe.start_time) tuples for the given group IDs. Joins
	// activities.schedules to schedule.timeframes and filters out rows
	// without a linked timeframe. Both tables are tenant-scoped in the
	// WHERE clause as defense-in-depth alongside RLS. Returns an empty
	// slice when groupIDs is empty.
	FindTemplateStartTimesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activityrecords.TemplateStartTime, error)

	// CapValidUntil ends the recurrence of every schedule of the given
	// template at validUntil (exclusive): rows that are open-ended
	// (valid_until IS NULL) or end later than validUntil are capped to
	// validUntil. Returns the number of rows changed. Used by the template
	// split ("Dieser und alle folgenden", WP-B3).
	CapValidUntil(ctx context.Context, activityGroupID int64, validUntil string) (int64, error)
}

type PlannedSupervisorRecords interface {
	Create(context.Context, *activityContract.SupervisorPlanned) error
	FindByID(context.Context, int64) (*activityContract.SupervisorPlanned, error)
	Update(context.Context, *activityContract.SupervisorPlanned) error
	Delete(context.Context, int64) error

	// ListPlannedSupervisionBlockers returns planned activity supervisions
	// as caregiver-capability blocker rows.
	ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]activityrecords.PlannedSupervisionBlocker, error)

	// CloseOpenByGroupAndPeriod closes the open planned supervisions of a
	// group for the given calendar period (NULL period matches rows without
	// one) by setting valid_until.
	CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom activityContract.Date) error

	// FindByStaffID finds all supervisions for a specific staff member
	FindByStaffID(ctx context.Context, staffID int64) ([]*activityContract.SupervisorPlanned, error)

	// FindByGroupID finds all supervisors for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.SupervisorPlanned, error)

	// FindByGroupIDs finds all supervisors for multiple groups in a single query
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activityContract.SupervisorPlanned, error)

	// SetPrimary sets a supervisor as the primary supervisor for a group
	SetPrimary(ctx context.Context, id int64) error

	// CapActiveByGroup caps open supervision rows (valid_until IS NULL).
	// Rows starting on/after the cap are deleted because they have no interval
	// left; begun rows are ended at validUntil. Bounded rows are left to their
	// owning workflow. Returns the total number changed.
	CapActiveByGroup(ctx context.Context, groupID int64, validUntil activityContract.Date) (int64, error)

	// SetValidUntilByID closes exactly one supervision without writing the
	// rest of a potentially partial read model back to the database.
	SetValidUntilByID(ctx context.Context, id int64, validUntil activityContract.Date) error
}

type ActivityEnrollmentRecords interface {
	FindByStudentIDs(context.Context, []int64) ([]*activityContract.StudentEnrollment, error)
	Create(context.Context, *activityContract.StudentEnrollment) error
	FindByID(context.Context, int64) (*activityContract.StudentEnrollment, error)
	Update(context.Context, *activityContract.StudentEnrollment) error
	Delete(context.Context, int64) error

	// CloseOpenByGroupAndPeriod closes the open enrollments of a group for
	// the given calendar period (NULL period matches rows without one) by
	// setting valid_until.
	CloseOpenByGroupAndPeriod(ctx context.Context, groupID int64, calendarPeriodID *int64, validFrom activityContract.Date) error

	// FindByStudentID finds all enrollments for a specific student
	FindByStudentID(ctx context.Context, studentID int64) ([]*activityContract.StudentEnrollment, error)

	// FindActiveByStudentIDs finds active enrollments for a batch of students
	// on the given date. valid_until is exclusive.
	FindActiveByStudentIDs(ctx context.Context, studentIDs []int64, onDate activityContract.Date) ([]*activityContract.StudentEnrollment, error)

	// FindByGroupID finds all enrollments for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*activityContract.StudentEnrollment, error)

	// CapActiveByGroup caps open enrollment rows (valid_until IS NULL). Rows
	// starting on/after the cap are deleted because they have no interval left;
	// begun rows are ended at validUntil. Bounded rows are left to their owning
	// workflow. Returns the total number changed.
	CapActiveByGroup(ctx context.Context, groupID int64, validUntil activityContract.Date) (int64, error)

	// SetValidUntilByID closes exactly one enrollment without writing the
	// rest of a potentially partial read model back to the database.
	SetValidUntilByID(ctx context.Context, id int64, validUntil activityContract.Date) error
}
