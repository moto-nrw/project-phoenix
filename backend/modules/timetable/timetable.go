// Package timetable is the public Timetable & Activities capability. It owns
// activity categories, templates, target groups, schedules, rosters, and
// materialized activity instances.
package timetable

import (
	"context"
	"errors"
	"regexp"
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
	OpenRoomCategoryName        = "Offene Räume"
	OpenRoomCategoryDescription = "Aufenthalte in freigegebenen Räumen ohne Angebot"
	OpenRoomCategoryColor       = "#5080D8"
	OpenRoomActivityName        = "Offener Raum"
	OpenRoomMaxParticipants     = 300
)

var (
	ErrCategoryNotFound              = errors.New("activity category not found")
	ErrInvalidCategory               = errors.New("invalid activity category")
	ErrInvalidCareExitEnrollment     = errors.New("invalid care-exit activity enrollment")
	ErrUnknownCategoryIDs            = errors.New("one or more category IDs do not exist in this tenant")
	ErrSystemCategoryProtected       = errors.New("systemkategorie kann nicht bearbeitet oder archiviert werden")
	ErrSystemCategoryName            = errors.New("dieser Name ist für eine Systemkategorie reserviert")
	ErrCategoryNameExists            = errors.New("eine Kategorie mit diesem Namen existiert bereits")
	ErrCategoryArchived              = errors.New("archivierte Kategorie muss zuerst wiederhergestellt werden")
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
	HomeworkQuery
	BlockGroupQuery
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
	ActivityExceptionCommand
	ActivityInstanceCommand
	InstanceStaffCommand
	InstanceStudentCommand
	ConflictAckCommand
	PickupExtensionCommand
	CategoryCommand
	CareExitEnrollmentCommand
	ActivityGroupCommand
	HomeworkCommand
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
	// UpdateTemplateRosterReview writes the roster review of a template
	// (#5103); it reports the rows changed.
	UpdateTemplateRosterReview(context.Context, int64, TemplateRosterReviewUpdate) (int64, error)
}

// TemplateRosterReviewUpdate is a roster review write (#5103). Source turns
// the segment's Zielgruppe into the offering source in place;
// ManualRosterConfirmed stores "Bleibt von Hand gepflegt" on the series
// root. A nil field stays untouched.
type TemplateRosterReviewUpdate struct {
	Source                *OfferingSourceInput
	ManualRosterConfirmed *bool
}

type Capability interface {
	Query
	Command
}

// Owner is what the Timetable composition hands out: the capability, the
// retired-staff cleanup and the planned group read.
type Owner interface {
	Capability
	StaffRetentionCapability
	PlannedGroupQuery
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

func IsSystemActivityName(name string) bool {
	return name == SchulhofActivityName || name == WCActivityName || name == OpenRoomActivityName
}
