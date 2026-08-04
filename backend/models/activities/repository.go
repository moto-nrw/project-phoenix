package activities

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

	// FindByNameIncludingArchivedForShare finds a category by name across active
	// and archived rows, preferring the active row and holding a transaction-
	// scoped FOR SHARE lock against a concurrent archive.
	FindByNameIncludingArchivedForShare(ctx context.Context, name string) (*Category, error)

	// FindByIDForShare returns a category while holding a transaction-scoped
	// FOR SHARE lock. New category assignments use it so an archive cannot
	// commit between validation and the referencing write.
	FindByIDForShare(ctx context.Context, id int64) (*Category, error)

	// ListAll returns all categories
	ListAll(ctx context.Context) ([]*Category, error)

	// UpdateIfActive writes only editable fields while archived_at is still
	// NULL. The condition and update run in one statement so a stale editor
	// cannot reactivate a category or overwrite a concurrent shift mapping.
	UpdateIfActive(ctx context.Context, category *Category) (updated bool, err error)

	// SetShiftTypeForCategories syncs the Kategorie↔Schichtart mapping for one
	// shift type (#1837 follow-up): it sets shift_type_id = shiftTypeID for the
	// given category IDs and clears it on any category that currently points at
	// this shift type but is no longer in the list. A cross-row set/clear that
	// isn't reducible to a single-entity filter, so it lives here rather than on
	// the generic Repository[T].
	SetShiftTypeForCategories(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error

	// UpdateColumns is the generic partial-update helper promoted from the
	// embedded base repository: updates only the named columns by primary key
	// and returns the number of rows affected. Archive/restore writes just
	// archived_at through it (#2131), so a concurrent rename is never
	// clobbered and a restore still surfaces the case-insensitive partial
	// unique-index violation on its own.
	UpdateColumns(ctx context.Context, category *Category, columns ...string) (int64, error)
}

// GroupRepository defines operations for managing activity groups
type GroupRepository interface {
	base.Repository[*Group]

	// FindByName finds a non-archived activity group by its name.
	FindByName(ctx context.Context, name string) (*Group, error)

	// FindByIDs returns the groups matching the given IDs in one
	// tenant-scoped IN query. Archived groups are included — callers resolve
	// display names for historical references (matching FindByID). Empty
	// input returns an empty slice without hitting the DB.
	FindByIDs(ctx context.Context, ids []int64) ([]*Group, error)

	// ListTemplateRows returns the template list read model (template +
	// schedule + aggregate people counts), optionally filtered to one
	// template. Issue #584: the aggregation query moved verbatim out of
	// api/timetable; its multi-schema joins cannot be expressed through the
	// generic repository shape.
	ListTemplateRows(ctx context.Context, templateID *int64) ([]TemplateListRow, error)

	// ListTemplateRowsForTemplatePeriod returns the editable detail read model
	// for one template, scoped to one calendar period.
	ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) ([]TemplateListRow, error)

	// ListTemplateRowsForPeriod is the calendar-period-filtered variant used
	// by the template list endpoint (people aggregates and schedules filter
	// on the period when given).
	ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]TemplateListRow, error)

	// ListTemplateWeekdayRoster returns the weekday-scoped roster rows of the
	// active (open) roster of every non-archived template, optionally narrowed
	// to one template and calendar period (issue #2129). A concrete period also
	// includes series-wide rows whose calendar_period_id is NULL; a nil period
	// returns only unscoped rows and never merges several periods. It includes
	// empty markers for scheduled weekdays and expands every applicable
	// series-wide roster row onto the weekdays where it materializes.
	ListTemplateWeekdayRoster(ctx context.Context, templateID, calendarPeriodID *int64) ([]TemplateWeekdayRosterRow, error)

	// ListTemplateCapacityOccurrences returns one staffing snapshot per actual
	// recurrence date in periodID. It applies the same schedule, A/B-week,
	// cancellation, roster-validity, period, and selected-weekday rules as
	// materialization. Choosing which occurrence is worst depends on the
	// tenant's staffing ratio and therefore remains a service concern.
	// A nil periodID evaluates all active periods, used by template detail
	// responses that have no period query parameter.
	ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64) ([]TemplateCapacityOccurrence, error)

	// UpdateTemplateFields patches the editable template fields of a
	// non-archived template row.
	UpdateTemplateFields(ctx context.Context, id int64, fields TemplateFieldsUpdate) (rowsAffected int64, err error)

	// ArchiveTemplate soft-deletes a non-archived template (sets archived_at).
	ArchiveTemplate(ctx context.Context, id int64) (rowsAffected int64, err error)

	// FindByCategory finds all groups in a specific category
	FindByCategory(ctx context.Context, categoryID int64) ([]*Group, error)

	// CountByCategory returns how many activity groups (Aktivitäten and
	// Termin-Vorlagen alike) reference each category in the current tenant,
	// keyed by category id. Categories with no groups are absent from the map.
	// A GROUP BY aggregate across the whole table that the generic
	// Repository[T] shape cannot express (#2131).
	CountByCategory(ctx context.Context) (map[int64]int, error)

	// FindOpenGroups finds all groups that are open for enrollment
	FindOpenGroups(ctx context.Context) ([]*Group, error)

	// FindWithEnrollmentCounts returns groups with their current enrollment counts
	FindWithEnrollmentCounts(ctx context.Context) ([]*Group, map[int64]int, error)

	// FindWithSupervisors returns a group with its supervisors
	FindWithSupervisors(ctx context.Context, groupID int64) (*Group, []*SupervisorPlanned, error)

	// FindByStaffSupervisor finds all activity groups where a staff member is a supervisor
	FindByStaffSupervisor(ctx context.Context, staffID int64) ([]*Group, error)

	// FindAllTemplates returns all activity groups flagged as templates
	// (is_template = true). Used by the materialization service to enumerate
	// candidates when generating schedule.activity_instances rows.
	FindAllTemplates(ctx context.Context) ([]*Group, error)

	// FindTemplateSeries returns every non-archived template segment in the
	// same split lineage as groupID. An unsplit template is one segment.
	FindTemplateSeries(ctx context.Context, groupID int64) ([]*Group, error)

	// FindTemplatesBySourceOffering returns every non-archived template that
	// declares the given care offering as its roster source (#2137). One
	// offering may feed many parallel Regeltermine; split successors carry
	// the copied source column, so every live segment appears individually.
	FindTemplatesBySourceOffering(ctx context.Context, offeringID int64) ([]*Group, error)

	// FindTemplatesWithOfferingSource returns every non-archived template of
	// the tenant that declares ANY care offering as its roster source (#2137).
	// Grade transitions use it to re-reconcile all sourced rosters after
	// school_class rewrites — a per-offering lookup cannot enumerate them
	// because the affected offerings are unknown at that point.
	FindTemplatesWithOfferingSource(ctx context.Context) ([]*Group, error)
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

	// SetPrimary sets a supervisor as the primary supervisor for a group
	SetPrimary(ctx context.Context, id int64) error

	// DeleteByStaffID removes all planned supervisions for a staff member
	// (staff offboarding cleanup).
	DeleteByStaffID(ctx context.Context, staffID int64) (int64, error)

	// CapActiveByGroup caps open supervision rows (valid_until IS NULL).
	// Rows starting on/after the cap are deleted because they have no interval
	// left; begun rows are ended at validUntil. Bounded rows are left to their
	// owning workflow. Returns the total number changed.
	CapActiveByGroup(ctx context.Context, groupID int64, validUntil timezone.Date) (int64, error)

	// SetValidUntilByID closes exactly one supervision without writing the
	// rest of a potentially partial read model back to the database.
	SetValidUntilByID(ctx context.Context, id int64, validUntil timezone.Date) error
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

	// FindActiveByStudentIDs finds active enrollments for a batch of students
	// on the given date. valid_until is exclusive.
	FindActiveByStudentIDs(ctx context.Context, studentIDs []int64, onDate timezone.Date) ([]*StudentEnrollment, error)

	// FindByGroupID finds all enrollments for a specific group
	FindByGroupID(ctx context.Context, groupID int64) ([]*StudentEnrollment, error)

	// BackfillEnrollmentRequestChildSource stamps legacy rows that were
	// materialized during the same approval as requestChildID but predate the
	// explicit provenance column. The group list keeps the operation bounded to
	// offerings that were linked before an adjustment.
	BackfillEnrollmentRequestChildSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (int64, error)

	// DeleteByEnrollmentRequestChild removes rows materialized from one
	// approved enrollment request child for a specific student.
	DeleteByEnrollmentRequestChild(ctx context.Context, studentID, requestChildID int64) (int64, error)

	// CapActiveByGroup caps open enrollment rows (valid_until IS NULL). Rows
	// starting on/after the cap are deleted because they have no interval left;
	// begun rows are ended at validUntil. Bounded rows are left to their owning
	// workflow. Returns the total number changed.
	CapActiveByGroup(ctx context.Context, groupID int64, validUntil timezone.Date) (int64, error)

	// SetValidUntilByID closes exactly one enrollment without writing the
	// rest of a potentially partial read model back to the database.
	SetValidUntilByID(ctx context.Context, id int64, validUntil timezone.Date) error
}

// TemplateFieldsUpdate carries the editable fields of PUT /templates/{id}.
// Grouped into a struct (rather than positional params) because the field
// count grew past what's readable positionally once Zielgruppe/calendar
// period joined the original create-time fields.
type TemplateFieldsUpdate struct {
	Name             string
	Type             string
	CategoryID       int64
	RoomID           int64
	EducationGroupID *int64
	MaxParticipants  int
	// RequiredStaff is the manual Personalbedarf override (#1839). nil ->
	// clear the override (derive from the Betreuungsschlüssel).
	RequiredStaff     *int
	CalendarPeriodID  *int64
	TargetGroupType   string
	TargetGradeLevel  *int16
	TargetSchoolClass *string
	// ListKind classifies the template for printable daily lists (#1565);
	// nil clears it.
	ListKind *string
	// Notes is the durable Wochennotiz for the template; nil clears it.
	Notes *string
	// SourceCareOfferingID/SourceGradeLevels are the offering-source rule
	// (#2137); nil source clears both.
	SourceCareOfferingID *int64
	SourceGradeLevels    []int
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
	// RequiredStaff is the template's manual Personalbedarf override (#1839);
	// NULL = derive from the Betreuungsschlüssel.
	RequiredStaff sql.NullInt64 `bun:"required_staff"`
	// TemplateCalendarPeriodID is the template's OWN period pin (Group.
	// CalendarPeriodID), distinct from CalendarPeriodID below which is the
	// per-schedule-row pin. See materialization_service.go's
	// schedulePinnedPeriodID for the precedence between the two.
	TemplateCalendarPeriodID sql.NullInt64  `bun:"template_calendar_period_id"`
	TargetGroupType          string         `bun:"target_group_type"`
	TargetGradeLevel         sql.NullInt16  `bun:"target_grade_level"`
	TargetSchoolClass        sql.NullString `bun:"target_school_class"`
	SourceCareOfferingID     sql.NullInt64  `bun:"source_care_offering_id"`
	// SourceGradeLevelsJSON carries the jsonb grade filter as its text form
	// ('' = NULL); parse with ParseSourceGradeLevels.
	SourceGradeLevelsJSON string `bun:"source_grade_levels_json"`
	// ListKind classifies the template for printable daily lists (#1565).
	ListKind sql.NullString `bun:"list_kind"`
	// Notes is the template's durable Wochennotiz (#1837 follow-up); NULL = none.
	Notes sql.NullString `bun:"notes"`
	// ShiftTypeName/ShiftTypeColor come from the category's optional
	// Kategorie↔Schichtart mapping (#1836/#1837 follow-up); empty when unmapped.
	ShiftTypeName   string `bun:"shift_type_name"`
	ShiftTypeColor  string `bun:"shift_type_color"`
	EnrollmentCount int    `bun:"enrollment_count"`
	SupervisorCount int    `bun:"supervisor_count"`
	// CapacityEnrollmentCount and CapacitySupervisorCount are the roster on
	// the actual recurrence date selected as worst for the template. They
	// intentionally differ from the period-tolerant display roster below.
	CapacityEnrollmentCount int `bun:"capacity_enrollment_count"`
	CapacitySupervisorCount int `bun:"capacity_supervisor_count"`
	// CapacityOccurrenceFound reports whether a real occurrence was selected
	// for the capacity counts above. When false (every date cancelled,
	// AB-week-filtered, or unmaterializable) the manual required_staff
	// override must not create demand — otherwise the list shows a false 0/N
	// understaffing indicator for a block that never occurs (#1839).
	CapacityOccurrenceFound bool           `bun:"-"`
	StudentIDs              []int64        `bun:"student_ids,array"`
	StaffIDs                []int64        `bun:"staff_ids,array"`
	PrimaryStaffID          sql.NullInt64  `bun:"primary_staff_id"`
	ScheduleID              int64          `bun:"schedule_id"`
	Weekday                 int            `bun:"weekday"`
	StartTime               sql.NullString `bun:"start_time"`
	EndTime                 sql.NullString `bun:"end_time"`
	WeekPattern             int            `bun:"week_pattern"`
	CalendarPeriodID        sql.NullInt64  `bun:"calendar_period_id"`
	ScheduleValidFrom       sql.NullString `bun:"schedule_valid_from"`
	ScheduleValidUntil      sql.NullString `bun:"schedule_valid_until"`
}

// ParseSourceGradeLevels decodes the SourceGradeLevelsJSON text form of the
// jsonb grade filter. Empty text (NULL column) yields nil.
func (r TemplateListRow) ParseSourceGradeLevels() ([]int, error) {
	if r.SourceGradeLevelsJSON == "" {
		return nil, nil
	}
	var levels []int
	if err := json.Unmarshal([]byte(r.SourceGradeLevelsJSON), &levels); err != nil {
		return nil, fmt.Errorf("parse source_grade_levels: %w", err)
	}
	if len(levels) == 0 {
		return nil, nil
	}
	return levels, nil
}

// TemplateWeekdayRosterRow is one weekday-scoped roster membership of a
// recurring template (issue #2129). Kind is either
// TemplateWeekdayRosterKindStaff, TemplateWeekdayRosterKindStudent, or
// TemplateWeekdayRosterKindProtectedStudent; PersonID is the staff or student
// id accordingly. Only rows that actually carry a weekday scope are returned,
// except protected enrollments, which are expanded onto each applicable
// scheduled weekday. Empty marker rows make an intentionally empty weekday
// distinguishable from a shared roster.
type TemplateWeekdayRosterRow struct {
	TemplateID int64  `bun:"template_id"`
	Weekday    int    `bun:"weekday"`
	Kind       string `bun:"kind"`
	PersonID   int64  `bun:"person_id"`
	IsPrimary  bool   `bun:"is_primary"`
}

const (
	TemplateWeekdayRosterKindEmpty   = "empty"
	TemplateWeekdayRosterKindStaff   = "staff"
	TemplateWeekdayRosterKindStudent = "student"
	// ProtectedStudent is read metadata for enrollment-owned rows. It lets the
	// editor seed explicit weekday lists without broadening selected_weekdays.
	TemplateWeekdayRosterKindProtectedStudent = "protected_student"
)

// TemplateCapacityOccurrence is the capacity-relevant roster for one date on
// which a recurring template actually runs. OccurrenceDate is a calendar day,
// not an instant; valid_until fields are evaluated as exclusive boundaries.
type TemplateCapacityOccurrence struct {
	TemplateID       int64         `bun:"template_id"`
	CalendarPeriodID int64         `bun:"calendar_period_id"`
	OccurrenceDate   timezone.Date `bun:"occurrence_date"`
	EnrollmentCount  int           `bun:"enrollment_count"`
	SupervisorCount  int           `bun:"supervisor_count"`
}
