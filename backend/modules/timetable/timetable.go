// Package timetable is the public Timetable & Activities capability. It owns
// activity categories, templates, target groups, schedules, rosters, and
// materialized activity instances.
package timetable

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultCategoryColor = "#CCCCCC"
	SchulhofCategoryName = "Schulhof"
	WCCategoryName       = "WC"
	SchulhofActivityName = "Schulhof Freispiel"
	WCActivityName       = "WC"
	// The shared system activity and category of released rooms other than
	// the Schulhof (#3066). A room session under this activity holds the
	// independent room stays of one released room; it is not an activity
	// children participate in.
	OpenRoomCategoryName         = "Offene Räume"
	OpenRoomCategoryDescription  = "Aufenthalte in freigegebenen Räumen ohne Angebot"
	OpenRoomCategoryColor        = "#5080D8"
	OpenRoomActivityName         = "Offener Raum"
	OpenRoomMaxParticipants      = 300
	categoryNameMaxLength        = 60
	categoryDescriptionMaxLength = 255
)

var (
	ErrCategoryNotFound              = errors.New("activity category not found")
	ErrInvalidCategory               = errors.New("invalid activity category")
	ErrInvalidCareExitEnrollment     = errors.New("invalid care-exit activity enrollment")
	ErrUnknownCategoryIDs            = errors.New("one or more category IDs do not exist in this tenant")
	ErrSystemCategoryProtected       = errors.New("Systemkategorie kann nicht bearbeitet oder archiviert werden") //nolint:staticcheck // ST1005: stable user-facing contract
	ErrSystemCategoryName            = errors.New("Dieser Name ist für eine Systemkategorie reserviert")          //nolint:staticcheck // ST1005: stable user-facing contract
	ErrCategoryNameExists            = errors.New("Eine Kategorie mit diesem Namen existiert bereits")            //nolint:staticcheck // ST1005: stable user-facing contract
	ErrCategoryArchived              = errors.New("Archivierte Kategorie muss zuerst wiederhergestellt werden")   //nolint:staticcheck // ST1005: stable user-facing contract
	ErrGroupNotFound                 = errors.New("activity group not found")
	ErrInvalidGroup                  = errors.New("invalid activity group")
	ErrInvalidGroupQuery             = errors.New("invalid activity group query")
	ErrInvalidGroupTarget            = errors.New("invalid activity group target")
	ErrScheduleNotFound              = errors.New("activity schedule not found")
	ErrInvalidSchedule               = errors.New("invalid activity schedule")
	ErrInvalidScheduleQuery          = errors.New("invalid activity schedule query")
	ErrPlannedSupervisorNotFound     = errors.New("planned activity supervisor not found")
	ErrInvalidPlannedSupervisor      = errors.New("invalid planned activity supervisor")
	ErrInvalidPlannedSupervisorQuery = errors.New("invalid planned activity supervisor query")
	ErrStudentEnrollmentNotFound     = errors.New("student activity enrollment not found")
	ErrInvalidStudentEnrollment      = errors.New("invalid student activity enrollment")
	ErrInvalidStudentEnrollmentQuery = errors.New("invalid student activity enrollment query")
	ErrTimeframeNotFound             = errors.New("timeframe not found")
	ErrInvalidTimeframe              = errors.New("invalid timeframe")
	ErrInvalidTimeframeQuery         = errors.New("invalid timeframe query")
	ErrPlanningTrackNotFound         = errors.New("planning track not found")
	ErrInvalidPlanningTrack          = errors.New("invalid planning track")
	ErrInvalidPlanningTrackQuery     = errors.New("invalid planning track query")
	ErrPlanningTrackNameExists       = errors.New("planning track name already exists")
	ErrRecurrenceRuleNotFound        = errors.New("recurrence rule not found")
	ErrInvalidRecurrenceRule         = errors.New("invalid recurrence rule")
	ErrInvalidRecurrenceRuleQuery    = errors.New("invalid recurrence rule query")
	ErrActivityExceptionNotFound     = errors.New("activity exception not found")
	ErrInvalidActivityException      = errors.New("invalid activity exception")
	ErrInvalidActivityExceptionQuery = errors.New("invalid activity exception query")
	ErrActivityInstanceNotFound      = errors.New("activity instance not found")
	ErrInvalidActivityInstance       = errors.New("invalid activity instance")
	ErrInvalidActivityInstanceQuery  = errors.New("invalid activity instance query")
	ErrInstanceStaffNotFound         = errors.New("instance staff assignment not found")
	ErrInvalidInstanceStaff          = errors.New("invalid instance staff assignment")
	ErrInvalidInstanceStaffQuery     = errors.New("invalid instance staff query")
	ErrInstanceStudentNotFound       = errors.New("instance student assignment not found")
	ErrInvalidInstanceStudent        = errors.New("invalid instance student assignment")
	ErrInvalidInstanceStudentQuery   = errors.New("invalid instance student query")
)

var categoryColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)
var planningTrackColorPattern = regexp.MustCompile(`^#[A-Fa-f0-9]{6}$`)

// Category is the owner view of one activities.categories row.
type Category struct {
	ID          int64      `json:"id"`
	TenantID    int64      `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Color       string     `json:"color,omitempty"`
	IsSystem    bool       `json:"is_system"`
	ShiftTypeID *int64     `json:"shift_type_id,omitempty"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

func (c Category) IsArchived() bool { return c.ArchivedAt != nil }

func (c Category) ColorOrDefault() string {
	if c.Color == "" {
		return DefaultCategoryColor
	}
	return c.Color
}

type CreateCategory struct {
	Name        string
	Description string
	Color       string
	IsSystem    bool
}

type UpdateCategory struct {
	ID          int64
	Name        string
	Description string
	Color       string
}

// CareExitEnrollment is the reversible snapshot used when a student's care
// ends. Calendar dates use YYYY-MM-DD because this public contract represents
// dates, not instants.
type CareExitEnrollment struct {
	ID                       int64
	TenantID                 int64
	StudentID                int64
	ActivityGroupID          int64
	ValidFrom                string
	ValidUntil               *string
	CalendarPeriodID         *int64
	EnrollmentRequestChildID *int64
	SelectedWeekdays         []int
	AttendanceStatus         *string
	Weekday                  *int
}

type CareExitEnrollmentCap struct {
	TenantID           int64
	StudentID          int64
	ID                 int64
	PreviousValidUntil *string
}

type CareExitEnrollmentChanges struct {
	Deleted []CareExitEnrollment
	Capped  []CareExitEnrollmentCap
}

type CareExitEnrollmentRemoval struct {
	CareExitEnrollment
	WasDeleted         bool
	PreviousValidUntil *string
}

type ScheduleQuery interface {
	FindSchedule(context.Context, int64) (Schedule, error)
	ListSchedules(context.Context, ScheduleFilter) ([]Schedule, error)
	FindTemplateStartTimes(context.Context, []int64) ([]TemplateStartTime, error)
}

type CourseGroupQuery interface {
	ListCourseGroups(context.Context, CourseGroupFilter) ([]CourseGroup, error)
}

type ScheduleCommand interface {
	CreateSchedule(context.Context, ScheduleInput) (Schedule, error)
	UpdateSchedule(context.Context, int64, ScheduleInput) (Schedule, error)
	DeleteSchedule(context.Context, int64) error
	DeleteSchedulesByGroup(context.Context, int64) error
	CapScheduleValidUntil(context.Context, int64, string) (int64, error)
}

type ScheduleCapability interface {
	ScheduleQuery
	ScheduleCommand
}

type Query interface {
	CalendarPeriodReferenceQuery
	CourseGroupQuery
	ScheduleQuery
	PlannedSupervisorQuery
	StudentEnrollmentQuery
	TimeframeQuery
	PlanningTrackQuery
	RecurrenceRuleQuery
	ActivityExceptionQuery
	ActivityInstanceQuery
	InstanceStaffQuery
	InstanceStudentQuery
	ConflictAckQuery
	PickupExtensionQuery
	SessionRosterQuery
	CategoryQuery
	ActivityGroupQuery
	TemplateRowQuery
}

// CalendarPeriodReferenceQuery counts the Timetable rows that reference each
// calendar period.
type CalendarPeriodReferenceQuery interface {
	CountCalendarPeriodReferences(context.Context) (map[int64]CalendarPeriodReferences, error)
}

// CategoryQuery reads activity categories and their usage.
type CategoryQuery interface {
	FindCategory(context.Context, int64) (Category, error)
	FindCategoryForAssignment(context.Context, int64) (Category, error)
	FindCategoryForShare(context.Context, int64) (Category, error)
	FindCategoryByName(context.Context, string) (Category, error)
	FindCategoryByNameForAssignment(context.Context, string) (Category, error)
	ListCategories(context.Context) ([]Category, error)
	CountCategoryUsage(context.Context) (map[int64]int, error)
}

// ActivityGroupQuery reads activity groups and their target groups.
type ActivityGroupQuery interface {
	FindGroup(context.Context, int64) (Group, error)
	FindGroupForUpdate(context.Context, int64) (Group, error)
	FindGroupByName(context.Context, string) (Group, error)
	ListGroups(context.Context, GroupFilter) ([]Group, error)
	ListGroupTargets(context.Context, []int64) (map[int64][]GroupTarget, error)
	ListTargetStudentIDs(context.Context, []int64) (map[int64][]int64, error)
}

// TemplateRowQuery reads the template list, weekday roster and capacity rows.
type TemplateRowQuery interface {
	ListTemplateRows(context.Context, *int64) ([]TemplateListRow, error)
	ListTemplateRowsForTemplatePeriod(context.Context, int64, int64) ([]TemplateListRow, error)
	ListTemplateRowsForPeriod(context.Context, *int64) ([]TemplateListRow, error)
	ListTemplateWeekdayRoster(context.Context, *int64, *int64) ([]TemplateWeekdayRosterRow, error)
	ListTemplateCapacityOccurrences(context.Context, *int64, []int64, []TemplateCapacityPeriod) ([]TemplateCapacityOccurrence, error)
}

type Command interface {
	CareExitRoster
	ScheduleCommand
	PlannedSupervisorCommand
	StudentEnrollmentCommand
	TimeframeCommand
	PlanningTrackCommand
	RecurrenceRuleCommand
	ActivityExceptionCommand
	ActivityInstanceCommand
	InstanceStaffCommand
	InstanceStudentCommand
	ConflictAckCommand
	PickupExtensionCommand
	CategoryCommand
	CareExitEnrollmentCommand
	ActivityGroupCommand
}

// CategoryCommand writes activity categories and their shift type links.
type CategoryCommand interface {
	CreateCategory(context.Context, CreateCategory) (Category, error)
	UpdateCategory(context.Context, UpdateCategory) (Category, error)
	ArchiveCategory(context.Context, int64) (Category, error)
	RestoreCategory(context.Context, int64) (Category, error)
	DeleteCategory(context.Context, int64) error
	SetCategoryShiftTypeID(context.Context, int64, *int64) error
	SetCategoryShiftTypeLinks(context.Context, int64, []int64) error
}

// CareExitEnrollmentCommand ends and restores the activity enrollments of
// children leaving care.
type CareExitEnrollmentCommand interface {
	LockStudentEnrollmentsForCareExit(context.Context, []int64, string) error
	EndStudentEnrollmentsForCareExit(context.Context, []int64, string) (CareExitEnrollmentChanges, error)
	RestoreStudentEnrollmentsForCareExit(context.Context, []int64, []int64, []CareExitEnrollmentRemoval) (int, error)
}

// ActivityGroupCommand writes activity groups, templates and their sources
// and target groups.
type ActivityGroupCommand interface {
	CreateGroup(context.Context, GroupInput) (Group, error)
	UpdateGroup(context.Context, int64, GroupInput) (Group, error)
	DeleteGroup(context.Context, int64) error
	UpdateTemplate(context.Context, int64, TemplateUpdate) (int64, error)
	ArchiveTemplate(context.Context, int64) (int64, error)
	UpdateGroupOfferingSource(context.Context, int64, OfferingSourceInput) error
	ReplaceGroupTargets(context.Context, int64, []GroupTargetInput) error
}

type Capability interface {
	Query
	Command
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("timetable: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) FindCategory(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("find_category", ErrInvalidCategory)
	}
	return m.engine.FindCategory(ctx, id)
}

func (m *Module) FindCategoryForAssignment(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("find_category_for_assignment", ErrInvalidCategory)
	}
	return m.engine.FindCategoryForAssignment(ctx, id)
}

func (m *Module) FindCategoryForShare(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("find_category_for_share", ErrInvalidCategory)
	}
	return m.engine.FindCategoryForShare(ctx, id)
}

func (m *Module) FindCategoryByName(ctx context.Context, name string) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, m.reject("find_category_by_name", ErrInvalidCategory)
	}
	return m.engine.FindCategoryByName(ctx, name)
}

func (m *Module) FindCategoryByNameForAssignment(ctx context.Context, name string) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, m.reject("find_category_by_name_for_assignment", ErrInvalidCategory)
	}
	return m.engine.FindCategoryByNameForAssignment(ctx, name)
}

func (m *Module) ListCategories(ctx context.Context) ([]Category, error) {
	return m.engine.ListCategories(ctx)
}

func (m *Module) CountCategoryUsage(ctx context.Context) (map[int64]int, error) {
	return m.engine.CountCategoryUsage(ctx)
}

func (m *Module) FindGroup(ctx context.Context, id int64) (Group, error) {
	if id <= 0 {
		return Group{}, m.reject("find_group", ErrInvalidGroupQuery)
	}
	return m.engine.FindGroup(ctx, id)
}

func (m *Module) FindGroupForUpdate(ctx context.Context, id int64) (Group, error) {
	if id <= 0 {
		return Group{}, m.reject("find_group_for_update", ErrInvalidGroupQuery)
	}
	return m.engine.FindGroupForUpdate(ctx, id)
}

func (m *Module) FindGroupByName(ctx context.Context, name string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Group{}, m.reject("find_group_by_name", ErrInvalidGroupQuery)
	}
	return m.engine.FindGroupByName(ctx, name)
}

func (m *Module) ListGroups(ctx context.Context, filter GroupFilter) ([]Group, error) {
	if hasInvalidID(filter.IDs) || hasInvalidID(filter.SourceOfferingIDs) ||
		(filter.CategoryID != nil && *filter.CategoryID <= 0) ||
		(filter.SeriesForGroupID != nil && *filter.SeriesForGroupID <= 0) {
		return nil, m.reject("list_groups", ErrInvalidGroupQuery)
	}
	return m.engine.ListGroups(ctx, filter)
}

func (m *Module) ListCourseGroups(ctx context.Context, filter CourseGroupFilter) ([]CourseGroup, error) {
	if hasInvalidID(filter.LegacyGroupIDs) || hasInvalidID(filter.SourceOfferingIDs) {
		return nil, m.reject("list_course_groups", ErrInvalidGroupQuery)
	}
	if len(filter.LegacyGroupIDs) == 0 && len(filter.SourceOfferingIDs) == 0 {
		return []CourseGroup{}, nil
	}
	return m.engine.ListCourseGroups(ctx, filter)
}

func (m *Module) ListTemplateRows(ctx context.Context, templateID *int64) ([]TemplateListRow, error) {
	if templateID != nil && *templateID <= 0 {
		return nil, m.reject("list_template_rows", ErrInvalidGroupQuery)
	}
	return m.engine.ListTemplateRows(ctx, templateID)
}

func (m *Module) ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) ([]TemplateListRow, error) {
	if templateID <= 0 || periodID <= 0 {
		return nil, m.reject("list_template_rows_for_template_period", ErrInvalidGroupQuery)
	}
	return m.engine.ListTemplateRowsForTemplatePeriod(ctx, templateID, periodID)
}

func (m *Module) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]TemplateListRow, error) {
	if periodID != nil && *periodID <= 0 {
		return nil, m.reject("list_template_rows_for_period", ErrInvalidGroupQuery)
	}
	return m.engine.ListTemplateRowsForPeriod(ctx, periodID)
}

func (m *Module) ListTemplateWeekdayRoster(ctx context.Context, templateID, periodID *int64) ([]TemplateWeekdayRosterRow, error) {
	if (templateID != nil && *templateID <= 0) || (periodID != nil && *periodID <= 0) {
		return nil, m.reject("list_template_weekday_roster", ErrInvalidGroupQuery)
	}
	return m.engine.ListTemplateWeekdayRoster(ctx, templateID, periodID)
}

func (m *Module) ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64, periods []TemplateCapacityPeriod) ([]TemplateCapacityOccurrence, error) {
	if (periodID != nil && *periodID <= 0) || hasInvalidID(templateIDs) {
		return nil, m.reject("list_template_capacity_occurrences", ErrInvalidGroupQuery)
	}
	if len(templateIDs) == 0 {
		return []TemplateCapacityOccurrence{}, nil
	}
	return m.engine.ListTemplateCapacityOccurrences(ctx, periodID, templateIDs, periods)
}

func (m *Module) ListGroupTargets(ctx context.Context, groupIDs []int64) (map[int64][]GroupTarget, error) {
	if hasInvalidID(groupIDs) {
		return nil, m.reject("list_group_targets", ErrInvalidGroupQuery)
	}
	return m.engine.ListGroupTargets(ctx, groupIDs)
}

func (m *Module) ListTargetStudentIDs(ctx context.Context, groupIDs []int64) (map[int64][]int64, error) {
	if hasInvalidID(groupIDs) {
		return nil, m.reject("list_target_student_ids", ErrInvalidGroupQuery)
	}
	return m.engine.ListTargetStudentIDs(ctx, groupIDs)
}

func (m *Module) ReplaceGroupTargets(ctx context.Context, groupID int64, targets []GroupTargetInput) error {
	normalized, err := normalizeGroupTargets(groupID, targets)
	if err != nil {
		return m.reject("replace_group_targets", err)
	}
	return m.engine.ReplaceGroupTargets(ctx, groupID, normalized)
}

func (m *Module) CreateGroup(ctx context.Context, input GroupInput) (Group, error) {
	if err := normalizeGroup(&input); err != nil {
		return Group{}, m.reject("create_group", err)
	}
	return m.engine.CreateGroup(ctx, input)
}

func (m *Module) UpdateGroup(ctx context.Context, id int64, input GroupInput) (Group, error) {
	if id <= 0 {
		return Group{}, m.reject("update_group", ErrInvalidGroup)
	}
	if err := normalizeGroup(&input); err != nil {
		return Group{}, m.reject("update_group", err)
	}
	return m.engine.UpdateGroup(ctx, id, input)
}

func (m *Module) DeleteGroup(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_group", ErrInvalidGroup)
	}
	return m.engine.DeleteGroup(ctx, id)
}

func (m *Module) UpdateTemplate(ctx context.Context, id int64, input TemplateUpdate) (int64, error) {
	if id <= 0 || !normalizeTemplateUpdate(&input) {
		return 0, m.reject("update_template", ErrInvalidGroup)
	}
	return m.engine.UpdateTemplate(ctx, id, input)
}

func (m *Module) ArchiveTemplate(ctx context.Context, id int64) (int64, error) {
	if id <= 0 {
		return 0, m.reject("archive_template", ErrInvalidGroup)
	}
	return m.engine.ArchiveTemplate(ctx, id)
}

func (m *Module) UpdateGroupOfferingSource(ctx context.Context, id int64, input OfferingSourceInput) error {
	fields := GroupInput{TargetGroupType: TargetGroupTypeOffering,
		SourceCareOfferingIDs: input.CareOfferingIDs, SourceGradeLevels: input.GradeLevels,
		SourceSchoolClasses: input.SchoolClasses}
	if id <= 0 || !normalizeOfferingSource(&fields) {
		return m.reject("update_group_offering_source", ErrInvalidGroup)
	}
	input.CareOfferingIDs = fields.SourceCareOfferingIDs
	input.GradeLevels = fields.SourceGradeLevels
	input.SchoolClasses = fields.SourceSchoolClasses
	return m.engine.UpdateGroupOfferingSource(ctx, id, input)
}

func (m *Module) CreateCategory(ctx context.Context, input CreateCategory) (Category, error) {
	started := time.Now()
	if err := normalizeCategory(&input.Name, &input.Description, &input.Color); err != nil {
		m.engine.ObserveRejection("create_category", time.Since(started), err)
		return Category{}, err
	}
	if !input.IsSystem && reservedCategoryName(input.Name) {
		m.engine.ObserveRejection("create_category", time.Since(started), ErrSystemCategoryName)
		return Category{}, ErrSystemCategoryName
	}
	return m.engine.CreateCategory(ctx, input)
}

func (m *Module) UpdateCategory(ctx context.Context, input UpdateCategory) (Category, error) {
	started := time.Now()
	if input.ID <= 0 {
		m.engine.ObserveRejection("update_category", time.Since(started), ErrInvalidCategory)
		return Category{}, ErrInvalidCategory
	}
	if err := normalizeCategory(&input.Name, &input.Description, &input.Color); err != nil {
		m.engine.ObserveRejection("update_category", time.Since(started), err)
		return Category{}, err
	}
	if reservedCategoryName(input.Name) {
		m.engine.ObserveRejection("update_category", time.Since(started), ErrSystemCategoryName)
		return Category{}, ErrSystemCategoryName
	}
	return m.engine.UpdateCategory(ctx, input)
}

func (m *Module) ArchiveCategory(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("archive_category", ErrInvalidCategory)
	}
	return m.engine.ArchiveCategory(ctx, id)
}

func (m *Module) RestoreCategory(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("restore_category", ErrInvalidCategory)
	}
	return m.engine.RestoreCategory(ctx, id)
}

func (m *Module) DeleteCategory(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_category", ErrInvalidCategory)
	}
	return m.engine.DeleteCategory(ctx, id)
}

func (m *Module) SetCategoryShiftTypeID(ctx context.Context, id int64, shiftTypeID *int64) error {
	if id <= 0 || (shiftTypeID != nil && *shiftTypeID <= 0) {
		return m.reject("set_category_shift_type_id", ErrInvalidCategory)
	}
	return m.engine.SetCategoryShiftTypeID(ctx, id, shiftTypeID)
}

func (m *Module) SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
	if shiftTypeID <= 0 || hasInvalidID(categoryIDs) {
		return m.reject("set_category_shift_type_links", ErrInvalidCategory)
	}
	return m.engine.SetCategoryShiftTypeLinks(ctx, shiftTypeID, categoryIDs)
}

func (m *Module) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) error {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || !validDate(validUntil) {
		return m.reject("lock_student_enrollments_for_care_exit", ErrInvalidCareExitEnrollment)
	}
	return m.engine.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
}

func (m *Module) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (CareExitEnrollmentChanges, error) {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || !validDate(validUntil) {
		return CareExitEnrollmentChanges{}, m.reject("end_student_enrollments_for_care_exit", ErrInvalidCareExitEnrollment)
	}
	return m.engine.EndStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
}

func (m *Module) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, periodIDs []int64, removals []CareExitEnrollmentRemoval) (int, error) {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || hasInvalidID(periodIDs) || invalidCareExitRemovals(removals) {
		return 0, m.reject("restore_student_enrollments_for_care_exit", ErrInvalidCareExitEnrollment)
	}
	return m.engine.RestoreStudentEnrollmentsForCareExit(ctx, studentIDs, periodIDs, removals)
}

func (m *Module) reject(operation string, err error) error {
	m.engine.ObserveRejection(operation, 0, err)
	return err
}

func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrOffboardingConflict):
		return "offboarding_conflict"
	case err == nil:
		return "none"
	case errors.Is(err, ErrCategoryNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidCategory):
		return "invalid"
	case errors.Is(err, ErrInvalidCareExitEnrollment):
		return "invalid_care_exit_enrollment"
	case errors.Is(err, ErrUnknownCategoryIDs):
		return "unknown_category_ids"
	case errors.Is(err, ErrSystemCategoryProtected):
		return "system_category_protected"
	case errors.Is(err, ErrSystemCategoryName):
		return "system_category_name_reserved"
	case errors.Is(err, ErrCategoryNameExists):
		return "category_name_exists"
	case errors.Is(err, ErrCategoryArchived):
		return "category_archived"
	case errors.Is(err, ErrGroupNotFound):
		return "group_not_found"
	case errors.Is(err, ErrInvalidGroup):
		return "invalid_group"
	case errors.Is(err, ErrInvalidGroupQuery):
		return "invalid_group_query"
	case errors.Is(err, ErrInvalidGroupTarget):
		return "invalid_group_target"
	case errors.Is(err, ErrScheduleNotFound):
		return "schedule_not_found"
	case errors.Is(err, ErrInvalidSchedule):
		return "invalid_schedule"
	case errors.Is(err, ErrInvalidScheduleQuery):
		return "invalid_schedule_query"
	case errors.Is(err, ErrInvalidPickupExtension):
		return "invalid_pickup_extension"
	case errors.Is(err, ErrPickupExtensionNotFound):
		return "pickup_extension_not_found"
	case errors.Is(err, ErrPickupExtensionBlockGone):
		return "pickup_extension_block_gone"
	default:
		return "internal_error"
	}
}

// validActivityInstanceStatus accepts the planning states only: whether a
// block runs or ended is Student Presence's session.
