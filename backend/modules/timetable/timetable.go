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
	DefaultCategoryColor         = "#CCCCCC"
	SchulhofCategoryName         = "Schulhof"
	WCCategoryName               = "WC"
	SchulhofActivityName         = "Schulhof Freispiel"
	WCActivityName               = "WC"
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
	ScheduleQuery
	PlannedSupervisorQuery
	StudentEnrollmentQuery
	TimeframeQuery
	PlanningTrackQuery
	RecurrenceRuleQuery
	ActivityExceptionQuery
	ActivityInstanceQuery
	InstanceStaffQuery
	FindCategory(context.Context, int64) (Category, error)
	FindCategoryForAssignment(context.Context, int64) (Category, error)
	FindCategoryForShare(context.Context, int64) (Category, error)
	FindCategoryByName(context.Context, string) (Category, error)
	FindCategoryByNameForAssignment(context.Context, string) (Category, error)
	ListCategories(context.Context) ([]Category, error)
	CountCategoryUsage(context.Context) (map[int64]int, error)
	FindGroup(context.Context, int64) (Group, error)
	FindGroupForUpdate(context.Context, int64) (Group, error)
	FindGroupByName(context.Context, string) (Group, error)
	ListGroups(context.Context, GroupFilter) ([]Group, error)
	ListGroupTargets(context.Context, []int64) (map[int64][]GroupTarget, error)
	ListTargetStudentIDs(context.Context, []int64) (map[int64][]int64, error)
}

type Command interface {
	ScheduleCommand
	PlannedSupervisorCommand
	StudentEnrollmentCommand
	TimeframeCommand
	PlanningTrackCommand
	RecurrenceRuleCommand
	ActivityExceptionCommand
	ActivityInstanceCommand
	InstanceStaffCommand
	CreateCategory(context.Context, CreateCategory) (Category, error)
	UpdateCategory(context.Context, UpdateCategory) (Category, error)
	ArchiveCategory(context.Context, int64) (Category, error)
	RestoreCategory(context.Context, int64) (Category, error)
	DeleteCategory(context.Context, int64) error
	SetCategoryShiftTypeID(context.Context, int64, *int64) error
	SetCategoryShiftTypeLinks(context.Context, int64, []int64) error
	LockStudentEnrollmentsForCareExit(context.Context, []int64, string) error
	EndStudentEnrollmentsForCareExit(context.Context, []int64, string) (CareExitEnrollmentChanges, error)
	RestoreStudentEnrollmentsForCareExit(context.Context, []int64, []int64, []CareExitEnrollmentRemoval) (int, error)
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

type engine interface {
	Capability
	ObserveRejection(string, time.Duration, error)
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

func (m *Module) FindSchedule(ctx context.Context, id int64) (Schedule, error) {
	if id <= 0 {
		return Schedule{}, m.reject("find_schedule", ErrInvalidScheduleQuery)
	}
	return m.engine.FindSchedule(ctx, id)
}

func (m *Module) ListSchedules(ctx context.Context, filter ScheduleFilter) ([]Schedule, error) {
	return m.engine.ListSchedules(ctx, filter)
}

func (m *Module) FindTemplateStartTimes(ctx context.Context, groupIDs []int64) ([]TemplateStartTime, error) {
	return m.engine.FindTemplateStartTimes(ctx, groupIDs)
}

func (m *Module) CreateSchedule(ctx context.Context, input ScheduleInput) (Schedule, error) {
	if !normalizeSchedule(&input) {
		return Schedule{}, m.reject("create_schedule", ErrInvalidSchedule)
	}
	return m.engine.CreateSchedule(ctx, input)
}

func (m *Module) UpdateSchedule(ctx context.Context, id int64, input ScheduleInput) (Schedule, error) {
	if id <= 0 || !normalizeSchedule(&input) {
		return Schedule{}, m.reject("update_schedule", ErrInvalidSchedule)
	}
	return m.engine.UpdateSchedule(ctx, id, input)
}

func (m *Module) DeleteSchedule(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_schedule", ErrInvalidSchedule)
	}
	return m.engine.DeleteSchedule(ctx, id)
}

func (m *Module) DeleteSchedulesByGroup(ctx context.Context, groupID int64) error {
	if groupID <= 0 {
		return m.reject("delete_schedules_by_group", ErrInvalidSchedule)
	}
	return m.engine.DeleteSchedulesByGroup(ctx, groupID)
}

func (m *Module) CapScheduleValidUntil(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	if groupID <= 0 || !validDate(validUntil) {
		return 0, m.reject("cap_schedule_valid_until", ErrInvalidSchedule)
	}
	return m.engine.CapScheduleValidUntil(ctx, groupID, validUntil)
}

func (m *Module) FindPlannedSupervisor(ctx context.Context, id int64) (PlannedSupervisor, error) {
	if id <= 0 {
		return PlannedSupervisor{}, m.reject("find_planned_supervisor", ErrInvalidPlannedSupervisorQuery)
	}
	return m.engine.FindPlannedSupervisor(ctx, id)
}

func (m *Module) ListPlannedSupervisors(ctx context.Context, filter PlannedSupervisorFilter) ([]PlannedSupervisor, error) {
	if hasInvalidID(filter.GroupIDs) || (filter.StaffID != nil && *filter.StaffID <= 0) {
		return nil, m.reject("list_planned_supervisors", ErrInvalidPlannedSupervisorQuery)
	}
	return m.engine.ListPlannedSupervisors(ctx, filter)
}

func (m *Module) ListPlannedSupervisionBlockers(ctx context.Context, staffID int64) ([]PlannedSupervisionBlocker, error) {
	if staffID <= 0 {
		return nil, m.reject("list_planned_supervision_blockers", ErrInvalidPlannedSupervisorQuery)
	}
	return m.engine.ListPlannedSupervisionBlockers(ctx, staffID)
}

func (m *Module) CreatePlannedSupervisor(ctx context.Context, input PlannedSupervisorInput) (PlannedSupervisor, error) {
	if !normalizePlannedSupervisor(&input) {
		return PlannedSupervisor{}, m.reject("create_planned_supervisor", ErrInvalidPlannedSupervisor)
	}
	return m.engine.CreatePlannedSupervisor(ctx, input)
}

func (m *Module) UpdatePlannedSupervisor(ctx context.Context, id int64, input PlannedSupervisorInput) (PlannedSupervisor, error) {
	if id <= 0 || !normalizePlannedSupervisor(&input) {
		return PlannedSupervisor{}, m.reject("update_planned_supervisor", ErrInvalidPlannedSupervisor)
	}
	return m.engine.UpdatePlannedSupervisor(ctx, id, input)
}

func (m *Module) DeletePlannedSupervisor(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_planned_supervisor", ErrInvalidPlannedSupervisor)
	}
	return m.engine.DeletePlannedSupervisor(ctx, id)
}

func (m *Module) SetPrimaryPlannedSupervisor(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("set_primary_planned_supervisor", ErrInvalidPlannedSupervisor)
	}
	return m.engine.SetPrimaryPlannedSupervisor(ctx, id)
}

func (m *Module) DeletePlannedSupervisorsByStaff(ctx context.Context, staffID int64) (int64, error) {
	if staffID <= 0 {
		return 0, m.reject("delete_planned_supervisors_by_staff", ErrInvalidPlannedSupervisor)
	}
	return m.engine.DeletePlannedSupervisorsByStaff(ctx, staffID)
}

func (m *Module) CapActivePlannedSupervisors(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	if groupID <= 0 || !validDate(validUntil) {
		return 0, m.reject("cap_active_planned_supervisors", ErrInvalidPlannedSupervisor)
	}
	return m.engine.CapActivePlannedSupervisors(ctx, groupID, validUntil)
}

func (m *Module) SetPlannedSupervisorValidUntil(ctx context.Context, id int64, validUntil string) error {
	if id <= 0 || !validDate(validUntil) {
		return m.reject("set_planned_supervisor_valid_until", ErrInvalidPlannedSupervisor)
	}
	return m.engine.SetPlannedSupervisorValidUntil(ctx, id, validUntil)
}

func (m *Module) CloseOpenPlannedSupervisors(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	if groupID <= 0 || (periodID != nil && *periodID <= 0) || !validDate(validUntil) {
		return m.reject("close_open_planned_supervisors", ErrInvalidPlannedSupervisor)
	}
	return m.engine.CloseOpenPlannedSupervisors(ctx, groupID, periodID, validUntil)
}

func (m *Module) FindStudentEnrollment(ctx context.Context, id int64) (StudentEnrollment, error) {
	if id <= 0 {
		return StudentEnrollment{}, m.reject("find_student_enrollment", ErrInvalidStudentEnrollmentQuery)
	}
	return m.engine.FindStudentEnrollment(ctx, id)
}

func (m *Module) ListStudentEnrollments(ctx context.Context, filter StudentEnrollmentFilter) ([]StudentEnrollment, error) {
	if hasInvalidID(filter.StudentIDs) || hasInvalidID(filter.ActivityGroupIDs) || filter.Limit < 0 || filter.Offset < 0 ||
		(filter.ActiveOn != nil && !validDate(*filter.ActiveOn)) {
		return nil, m.reject("list_student_enrollments", ErrInvalidStudentEnrollmentQuery)
	}
	return m.engine.ListStudentEnrollments(ctx, filter)
}

func (m *Module) CreateStudentEnrollment(ctx context.Context, input StudentEnrollmentInput) (StudentEnrollment, error) {
	if !validStudentEnrollment(input) {
		return StudentEnrollment{}, m.reject("create_student_enrollment", ErrInvalidStudentEnrollment)
	}
	return m.engine.CreateStudentEnrollment(ctx, input)
}

func (m *Module) UpdateStudentEnrollment(ctx context.Context, id int64, input StudentEnrollmentInput) (StudentEnrollment, error) {
	if id <= 0 || !validStudentEnrollment(input) {
		return StudentEnrollment{}, m.reject("update_student_enrollment", ErrInvalidStudentEnrollment)
	}
	return m.engine.UpdateStudentEnrollment(ctx, id, input)
}

func (m *Module) DeleteStudentEnrollment(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_student_enrollment", ErrInvalidStudentEnrollment)
	}
	return m.engine.DeleteStudentEnrollment(ctx, id)
}

func (m *Module) BackfillStudentEnrollmentSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (int64, error) {
	if studentID <= 0 || requestChildID <= 0 || hasInvalidID(groupIDs) {
		return 0, m.reject("backfill_student_enrollment_source", ErrInvalidStudentEnrollment)
	}
	return m.engine.BackfillStudentEnrollmentSource(ctx, studentID, requestChildID, groupIDs)
}

func (m *Module) DeleteStudentEnrollmentsBySource(ctx context.Context, studentID, requestChildID int64) (int64, error) {
	if studentID <= 0 || requestChildID <= 0 {
		return 0, m.reject("delete_student_enrollments_by_source", ErrInvalidStudentEnrollment)
	}
	return m.engine.DeleteStudentEnrollmentsBySource(ctx, studentID, requestChildID)
}

func (m *Module) CapActiveStudentEnrollments(ctx context.Context, groupID int64, validUntil string) (int64, error) {
	if groupID <= 0 || !validDate(validUntil) {
		return 0, m.reject("cap_active_student_enrollments", ErrInvalidStudentEnrollment)
	}
	return m.engine.CapActiveStudentEnrollments(ctx, groupID, validUntil)
}

func (m *Module) SetStudentEnrollmentValidUntil(ctx context.Context, id int64, validUntil string) error {
	if id <= 0 || !validDate(validUntil) {
		return m.reject("set_student_enrollment_valid_until", ErrInvalidStudentEnrollment)
	}
	return m.engine.SetStudentEnrollmentValidUntil(ctx, id, validUntil)
}

func (m *Module) CloseOpenStudentEnrollments(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	if groupID <= 0 || (periodID != nil && *periodID <= 0) || !validDate(validUntil) {
		return m.reject("close_open_student_enrollments", ErrInvalidStudentEnrollment)
	}
	return m.engine.CloseOpenStudentEnrollments(ctx, groupID, periodID, validUntil)
}

func (m *Module) FindTimeframe(ctx context.Context, id int64) (Timeframe, error) {
	if id <= 0 {
		return Timeframe{}, m.reject("find_timeframe", ErrInvalidTimeframeQuery)
	}
	return m.engine.FindTimeframe(ctx, id)
}

func (m *Module) ListTimeframes(ctx context.Context, filter TimeframeFilter) ([]Timeframe, error) {
	if filter.Limit < 0 || filter.Offset < 0 ||
		(filter.OverlapsStart != nil && !validClock(*filter.OverlapsStart)) ||
		(filter.OverlapsEnd != nil && !validClock(*filter.OverlapsEnd)) {
		return nil, m.reject("list_timeframes", ErrInvalidTimeframeQuery)
	}
	return m.engine.ListTimeframes(ctx, filter)
}

func (m *Module) CreateTimeframe(ctx context.Context, input TimeframeInput) (Timeframe, error) {
	if !validTimeframe(input) {
		return Timeframe{}, m.reject("create_timeframe", ErrInvalidTimeframe)
	}
	return m.engine.CreateTimeframe(ctx, input)
}

func (m *Module) UpdateTimeframe(ctx context.Context, id int64, input TimeframeInput) (Timeframe, error) {
	if id <= 0 || !validTimeframe(input) {
		return Timeframe{}, m.reject("update_timeframe", ErrInvalidTimeframe)
	}
	return m.engine.UpdateTimeframe(ctx, id, input)
}

func (m *Module) DeleteTimeframe(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_timeframe", ErrInvalidTimeframe)
	}
	return m.engine.DeleteTimeframe(ctx, id)
}

func (m *Module) FindPlanningTrack(ctx context.Context, id int64) (PlanningTrack, error) {
	return m.findPlanningTrack(ctx, id, false)
}

func (m *Module) FindPlanningTrackForShare(ctx context.Context, id int64) (PlanningTrack, error) {
	return m.findPlanningTrack(ctx, id, true)
}

func (m *Module) findPlanningTrack(ctx context.Context, id int64, forShare bool) (PlanningTrack, error) {
	if id <= 0 {
		return PlanningTrack{}, m.reject("find_planning_track", ErrInvalidPlanningTrackQuery)
	}
	if forShare {
		return m.engine.FindPlanningTrackForShare(ctx, id)
	}
	return m.engine.FindPlanningTrack(ctx, id)
}

func (m *Module) ListPlanningTracks(ctx context.Context, filter PlanningTrackFilter) ([]PlanningTrack, error) {
	if hasInvalidID(filter.IDs) {
		return nil, m.reject("list_planning_tracks", ErrInvalidPlanningTrackQuery)
	}
	return m.engine.ListPlanningTracks(ctx, filter)
}

func (m *Module) CreatePlanningTrack(ctx context.Context, input PlanningTrackInput) (PlanningTrack, error) {
	input, ok := normalizePlanningTrack(input)
	if !ok {
		return PlanningTrack{}, m.reject("create_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.CreatePlanningTrack(ctx, input)
}

func (m *Module) UpdatePlanningTrack(ctx context.Context, id int64, input PlanningTrackInput) (PlanningTrack, error) {
	input, ok := normalizePlanningTrack(input)
	if id <= 0 || !ok {
		return PlanningTrack{}, m.reject("update_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.UpdatePlanningTrack(ctx, id, input)
}

func (m *Module) UpdateActivePlanningTrack(ctx context.Context, id int64, input PlanningTrackInput) (PlanningTrack, bool, error) {
	input, ok := normalizePlanningTrack(input)
	if id <= 0 || !ok {
		return PlanningTrack{}, false, m.reject("update_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.UpdateActivePlanningTrack(ctx, id, input)
}

func (m *Module) DeletePlanningTrack(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.DeletePlanningTrack(ctx, id)
}

func (m *Module) SetPlanningTrackArchivedAt(ctx context.Context, id int64, value *time.Time) (PlanningTrack, bool, error) {
	if id <= 0 {
		return PlanningTrack{}, false, m.reject("archive_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.SetPlanningTrackArchivedAt(ctx, id, value)
}

func (m *Module) ReorderPlanningTracks(ctx context.Context, ids []int64) error {
	if hasInvalidID(ids) || hasDuplicateID(ids) {
		return m.reject("reorder_planning_tracks", ErrInvalidPlanningTrack)
	}
	return m.engine.ReorderPlanningTracks(ctx, ids)
}

func (m *Module) RestorePlanningTrackAtEnd(ctx context.Context, id int64) (PlanningTrack, bool, error) {
	if id <= 0 {
		return PlanningTrack{}, false, m.reject("restore_planning_track", ErrInvalidPlanningTrack)
	}
	return m.engine.RestorePlanningTrackAtEnd(ctx, id)
}

func (m *Module) FindRecurrenceRule(ctx context.Context, id int64) (RecurrenceRule, error) {
	if id <= 0 {
		return RecurrenceRule{}, m.reject("find_recurrence_rule", ErrInvalidRecurrenceRuleQuery)
	}
	return m.engine.FindRecurrenceRule(ctx, id)
}

func (m *Module) ListRecurrenceRules(ctx context.Context, filter RecurrenceRuleFilter) ([]RecurrenceRule, error) {
	if filter.Limit < 0 || filter.Offset < 0 || !validRecurrenceRuleSort(filter.SortBy) {
		return nil, m.reject("list_recurrence_rules", ErrInvalidRecurrenceRuleQuery)
	}
	return m.engine.ListRecurrenceRules(ctx, filter)
}

func validRecurrenceRuleSort(value string) bool {
	return value == "" || value == "id" || value == "frequency" || value == "interval_count" ||
		value == "end_date" || value == "count" || value == "created_at" || value == "updated_at"
}

func (m *Module) CreateRecurrenceRule(ctx context.Context, input RecurrenceRuleInput) (RecurrenceRule, error) {
	input, ok := normalizeRecurrenceRule(input)
	if !ok {
		return RecurrenceRule{}, m.reject("create_recurrence_rule", ErrInvalidRecurrenceRule)
	}
	return m.engine.CreateRecurrenceRule(ctx, input)
}

func (m *Module) UpdateRecurrenceRule(ctx context.Context, id int64, input RecurrenceRuleInput) (RecurrenceRule, error) {
	input, ok := normalizeRecurrenceRule(input)
	if id <= 0 || !ok {
		return RecurrenceRule{}, m.reject("update_recurrence_rule", ErrInvalidRecurrenceRule)
	}
	return m.engine.UpdateRecurrenceRule(ctx, id, input)
}

func (m *Module) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_recurrence_rule", ErrInvalidRecurrenceRule)
	}
	return m.engine.DeleteRecurrenceRule(ctx, id)
}

func (m *Module) FindActivityException(ctx context.Context, id int64) (ActivityException, error) {
	if id <= 0 {
		return ActivityException{}, m.reject("find_activity_exception", ErrInvalidActivityExceptionQuery)
	}
	return m.engine.FindActivityException(ctx, id)
}

func (m *Module) ListActivityExceptions(ctx context.Context, filter ActivityExceptionFilter) ([]ActivityException, error) {
	if !validActivityExceptionFilter(filter) {
		return nil, m.reject("list_activity_exceptions", ErrInvalidActivityExceptionQuery)
	}
	return m.engine.ListActivityExceptions(ctx, filter)
}

func (m *Module) CountActivityExceptions(ctx context.Context, before *string) (int, error) {
	if !validOptionalDate(before) {
		return 0, m.reject("count_activity_exceptions", ErrInvalidActivityExceptionQuery)
	}
	return m.engine.CountActivityExceptions(ctx, before)
}

func (m *Module) OldestActivityExceptionBefore(ctx context.Context, before *string) (*string, error) {
	if !validOptionalDate(before) {
		return nil, m.reject("oldest_activity_exception", ErrInvalidActivityExceptionQuery)
	}
	return m.engine.OldestActivityExceptionBefore(ctx, before)
}

func (m *Module) CreateActivityException(ctx context.Context, input ActivityExceptionInput) (ActivityException, error) {
	if !validActivityException(input) {
		return ActivityException{}, m.reject("create_activity_exception", ErrInvalidActivityException)
	}
	return m.engine.CreateActivityException(ctx, input)
}

func (m *Module) UpdateActivityException(ctx context.Context, id int64, input ActivityExceptionInput) (ActivityException, error) {
	if id <= 0 || !validActivityException(input) {
		return ActivityException{}, m.reject("update_activity_exception", ErrInvalidActivityException)
	}
	return m.engine.UpdateActivityException(ctx, id, input)
}

func (m *Module) DeleteActivityException(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_activity_exception", ErrInvalidActivityException)
	}
	return m.engine.DeleteActivityException(ctx, id)
}

func (m *Module) DeleteActivityExceptionsBefore(ctx context.Context, before string) (int64, error) {
	if !validDate(before) {
		return 0, m.reject("delete_activity_exceptions_before", ErrInvalidActivityException)
	}
	return m.engine.DeleteActivityExceptionsBefore(ctx, before)
}

func (m *Module) FindActivityInstance(ctx context.Context, id int64) (ActivityInstance, error) {
	if id <= 0 {
		return ActivityInstance{}, m.reject("find_activity_instance", ErrInvalidActivityInstanceQuery)
	}
	return m.engine.FindActivityInstance(ctx, id)
}

func (m *Module) ListActivityInstances(ctx context.Context, filter ActivityInstanceFilter) ([]ActivityInstance, error) {
	if !validActivityInstanceFilter(filter) {
		return nil, m.reject("list_activity_instances", ErrInvalidActivityInstanceQuery)
	}
	return m.engine.ListActivityInstances(ctx, filter)
}

func (m *Module) MaxActivityInstanceID(ctx context.Context) (int64, error) {
	return m.engine.MaxActivityInstanceID(ctx)
}

func (m *Module) CountActivityInstances(ctx context.Context, before *string) (int, error) {
	if !validOptionalDate(before) {
		return 0, m.reject("count_activity_instances", ErrInvalidActivityInstanceQuery)
	}
	return m.engine.CountActivityInstances(ctx, before)
}

func (m *Module) OldestActivityInstanceBefore(ctx context.Context, before *string) (*string, error) {
	if !validOptionalDate(before) {
		return nil, m.reject("oldest_activity_instance", ErrInvalidActivityInstanceQuery)
	}
	return m.engine.OldestActivityInstanceBefore(ctx, before)
}

func (m *Module) CreateActivityInstance(ctx context.Context, input ActivityInstanceInput) (ActivityInstance, error) {
	input, ok := normalizeActivityInstance(input)
	if !ok {
		return ActivityInstance{}, m.reject("create_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.CreateActivityInstance(ctx, input)
}

func (m *Module) CreateTemplateBackedActivityInstanceIfAbsent(ctx context.Context, input ActivityInstanceInput) (ActivityInstance, bool, error) {
	input, ok := normalizeActivityInstance(input)
	if !ok || input.ActivityGroupID == nil {
		return ActivityInstance{}, false, m.reject("create_template_backed_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.CreateTemplateBackedActivityInstanceIfAbsent(ctx, input)
}

func (m *Module) CreateIdempotentActivityInstance(ctx context.Context, input ActivityInstanceInput) (ActivityInstance, bool, error) {
	input, ok := normalizeActivityInstance(input)
	if !ok || input.IdempotencyKey == nil {
		return ActivityInstance{}, false, m.reject("create_idempotent_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.CreateIdempotentActivityInstance(ctx, input)
}

func (m *Module) UpdateActivityInstance(ctx context.Context, id int64, input ActivityInstanceInput) (ActivityInstance, error) {
	input, ok := normalizeActivityInstance(input)
	if id <= 0 || !ok {
		return ActivityInstance{}, m.reject("update_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.UpdateActivityInstance(ctx, id, input)
}

func (m *Module) PatchActivityInstance(ctx context.Context, id int64, input ActivityInstanceInput, columns []string) (int64, error) {
	if id <= 0 || len(columns) == 0 || !validActivityInstanceColumns(columns) {
		return 0, m.reject("patch_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.PatchActivityInstance(ctx, id, input, columns)
}

func (m *Module) DeleteActivityInstance(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_activity_instance", ErrInvalidActivityInstance)
	}
	return m.engine.DeleteActivityInstance(ctx, id)
}

func (m *Module) MarkActivityInstanceCompleted(ctx context.Context, id int64, completedAt time.Time) error {
	if id <= 0 || completedAt.IsZero() {
		return m.reject("mark_activity_instance_completed", ErrInvalidActivityInstance)
	}
	return m.engine.MarkActivityInstanceCompleted(ctx, id, completedAt)
}

func (m *Module) CompleteActiveActivityInstances(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	if hasInvalidID(activeGroupIDs) || completedAt.IsZero() {
		return 0, m.reject("complete_active_activity_instances", ErrInvalidActivityInstance)
	}
	return m.engine.CompleteActiveActivityInstances(ctx, activeGroupIDs, completedAt)
}

func (m *Module) DeletePlannedActivityInstances(ctx context.Context, from string, to *string, groupID *int64, preserveDeviations bool) (int64, error) {
	if !validDate(from) || !validOptionalDate(to) || (groupID != nil && *groupID <= 0) {
		return 0, m.reject("delete_planned_activity_instances", ErrInvalidActivityInstance)
	}
	return m.engine.DeletePlannedActivityInstances(ctx, from, to, groupID, preserveDeviations)
}

func (m *Module) DeleteRemovedWeekendActivityInstances(ctx context.Context, groupID int64, weekdays []int) (int64, error) {
	if groupID <= 0 || invalidActivityInstanceWeekdays(weekdays) {
		return 0, m.reject("delete_removed_weekend_activity_instances", ErrInvalidActivityInstance)
	}
	return m.engine.DeleteRemovedWeekendActivityInstances(ctx, groupID, weekdays)
}

func (m *Module) PropagateActivityInstanceListKind(ctx context.Context, groupID int64, previousKind, newKind *string, after string) (int64, error) {
	if groupID <= 0 || !validDate(after) {
		return 0, m.reject("propagate_activity_instance_list_kind", ErrInvalidActivityInstance)
	}
	return m.engine.PropagateActivityInstanceListKind(ctx, groupID, previousKind, newKind, after)
}

func (m *Module) DeleteActivityInstancesBefore(ctx context.Context, before string) (int64, error) {
	if !validDate(before) {
		return 0, m.reject("delete_activity_instances_before", ErrInvalidActivityInstance)
	}
	return m.engine.DeleteActivityInstancesBefore(ctx, before)
}

func (m *Module) FindInstanceStaff(ctx context.Context, id int64) (InstanceStaff, error) {
	if id <= 0 {
		return InstanceStaff{}, m.reject("find_instance_staff", ErrInvalidInstanceStaffQuery)
	}
	return m.engine.FindInstanceStaff(ctx, id)
}

func (m *Module) ListInstanceStaff(ctx context.Context, filter InstanceStaffFilter) ([]InstanceStaff, error) {
	if !validInstanceStaffFilter(filter) {
		return nil, m.reject("list_instance_staff", ErrInvalidInstanceStaffQuery)
	}
	return m.engine.ListInstanceStaff(ctx, filter)
}

func (m *Module) CountNonAbsentInstanceStaff(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	if hasInvalidID(instanceIDs) {
		return nil, m.reject("count_non_absent_instance_staff", ErrInvalidInstanceStaffQuery)
	}
	return m.engine.CountNonAbsentInstanceStaff(ctx, instanceIDs)
}

func (m *Module) CreateInstanceStaff(ctx context.Context, input InstanceStaffInput) (InstanceStaff, error) {
	if !validInstanceStaff(input) {
		return InstanceStaff{}, m.reject("create_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.CreateInstanceStaff(ctx, input)
}

func (m *Module) UpdateInstanceStaff(ctx context.Context, id int64, input InstanceStaffInput) (InstanceStaff, error) {
	if id <= 0 || !validInstanceStaff(input) {
		return InstanceStaff{}, m.reject("update_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.UpdateInstanceStaff(ctx, id, input)
}

func (m *Module) PatchInstanceStaff(ctx context.Context, id int64, input InstanceStaffInput, columns []string) (int64, error) {
	if id <= 0 || !validInstanceStaffColumns(columns) {
		return 0, m.reject("patch_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.PatchInstanceStaff(ctx, id, input, columns)
}

func (m *Module) DeleteInstanceStaff(ctx context.Context, id int64) error {
	if id <= 0 {
		return m.reject("delete_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.DeleteInstanceStaff(ctx, id)
}

func (m *Module) DeleteInstanceStaffByInstance(ctx context.Context, instanceID int64) error {
	if instanceID <= 0 {
		return m.reject("delete_instance_staff_by_instance", ErrInvalidInstanceStaff)
	}
	return m.engine.DeleteInstanceStaffByInstance(ctx, instanceID)
}

func (m *Module) DeleteUpcomingInstanceStaff(ctx context.Context, staffID int64, after string) (int64, error) {
	if staffID <= 0 || !validDate(after) {
		return 0, m.reject("delete_upcoming_instance_staff", ErrInvalidInstanceStaff)
	}
	return m.engine.DeleteUpcomingInstanceStaff(ctx, staffID, after)
}

func invalidActivityInstanceWeekdays(weekdays []int) bool {
	for _, weekday := range weekdays {
		if !validWeekday(weekday) {
			return true
		}
	}
	return false
}

func validInstanceStaff(input InstanceStaffInput) bool {
	return input.InstanceID > 0 && input.StaffID > 0 &&
		(input.RoomID == nil || *input.RoomID > 0) && (input.SickAbsenceID == nil || *input.SickAbsenceID > 0)
}

func validInstanceStaffFilter(filter InstanceStaffFilter) bool {
	orders := 0
	for _, ordered := range []bool{filter.OrderByCreated, filter.OrderByInstanceAndCreated, filter.OrderByActivityTime, filter.OrderByActivityDateTime} {
		if ordered {
			orders++
		}
	}
	return filter.Limit >= 0 && filter.Offset >= 0 && orders <= 1 &&
		!hasInvalidID(filter.IDs) && !hasInvalidID(filter.InstanceIDs) && !hasInvalidID(filter.StaffIDs) &&
		(filter.SickAbsenceID == nil || *filter.SickAbsenceID > 0) &&
		validOptionalDate(filter.Date) && validOptionalDate(filter.FromDate) && validOptionalDate(filter.ToDate)
}

func validInstanceStaffColumns(columns []string) bool {
	if len(columns) == 0 || hasDuplicateString(columns) {
		return false
	}
	for _, column := range columns {
		if column != "is_primary" && column != "sick_absence_id" {
			return false
		}
	}
	return true
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

func normalizeCategory(name, description, color *string) error {
	*name = strings.TrimSpace(*name)
	*description = strings.TrimSpace(*description)
	*color = strings.TrimSpace(*color)
	if *name == "" || len([]rune(*name)) > categoryNameMaxLength || len([]rune(*description)) > categoryDescriptionMaxLength {
		return ErrInvalidCategory
	}
	if *color != "" && !strings.HasPrefix(*color, "#") {
		*color = "#" + *color
	}
	if *color != "" && !categoryColorPattern.MatchString(*color) {
		return ErrInvalidCategory
	}
	return nil
}

func reservedCategoryName(name string) bool {
	return strings.EqualFold(name, WCCategoryName) || strings.EqualFold(name, SchulhofCategoryName)
}

func IsSystemActivityName(name string) bool {
	return name == SchulhofActivityName || name == WCActivityName
}

func hasInvalidID(ids []int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return true
		}
	}
	return false
}

func validDate(value string) bool {
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
}

func validWeekday(value int) bool { return value >= WeekdayMonday && value <= WeekdaySunday }

func normalizeSchedule(input *ScheduleInput) bool {
	if input.ActivityGroupID <= 0 || !validWeekday(input.Weekday) {
		return false
	}
	return (input.ValidFrom == nil || validDate(*input.ValidFrom)) &&
		(input.ValidUntil == nil || validDate(*input.ValidUntil))
}

func normalizePlannedSupervisor(input *PlannedSupervisorInput) bool {
	if input.StaffID <= 0 || input.GroupID <= 0 || (input.Weekday != nil && !validWeekday(*input.Weekday)) {
		return false
	}
	return (input.ValidFrom == "" || validDate(input.ValidFrom)) &&
		(input.ValidUntil == nil || validDate(*input.ValidUntil))
}

func validStudentEnrollment(input StudentEnrollmentInput) bool {
	if input.StudentID <= 0 || input.ActivityGroupID <= 0 || !validDate(input.ValidFrom) ||
		(input.ValidUntil != nil && !validDate(*input.ValidUntil)) ||
		(input.CalendarPeriodID != nil && *input.CalendarPeriodID <= 0) ||
		(input.EnrollmentRequestChildID != nil && *input.EnrollmentRequestChildID <= 0) ||
		(input.Weekday != nil && !validWeekday(*input.Weekday)) {
		return false
	}
	if input.AttendanceStatus != nil && !validAttendanceStatus(*input.AttendanceStatus) {
		return false
	}
	seen := make(map[int]struct{}, len(input.SelectedWeekdays))
	for _, weekday := range input.SelectedWeekdays {
		if !validWeekday(weekday) {
			return false
		}
		if _, exists := seen[weekday]; exists {
			return false
		}
		seen[weekday] = struct{}{}
	}
	return true
}

func validAttendanceStatus(value string) bool {
	return value == AttendancePresent || value == AttendanceAbsent || value == AttendanceExcused || value == AttendanceUnknown
}

func validTimeframe(input TimeframeInput) bool {
	start, err := time.Parse("15:04:05", input.StartTime)
	if err != nil {
		return false
	}
	if input.EndTime == nil {
		return true
	}
	end, err := time.Parse("15:04:05", *input.EndTime)
	return err == nil && end.After(start)
}

func validClock(value string) bool {
	_, err := time.Parse("15:04:05", value)
	return err == nil
}

func validActivityExceptionFilter(filter ActivityExceptionFilter) bool {
	return filter.Limit >= 0 && filter.Offset >= 0 &&
		(filter.ActivityGroupID == nil || *filter.ActivityGroupID > 0) &&
		validOptionalDate(filter.ExceptionDate) && validOptionalDate(filter.FromDate) &&
		validOptionalDate(filter.ToDate) && validOptionalDate(filter.BeforeDate)
}

func validOptionalDate(value *string) bool {
	return value == nil || validDate(*value)
}

func validActivityException(input ActivityExceptionInput) bool {
	if input.ActivityGroupID <= 0 || !validDate(input.ExceptionDate) ||
		(input.Reason != nil && len(*input.Reason) > ActivityExceptionReasonMaxLength) {
		return false
	}
	switch input.ExceptionType {
	case ActivityExceptionCancelled:
		return input.StartTime == nil && input.EndTime == nil && input.RoomID == nil
	case ActivityExceptionModified:
		return validActivityExceptionOverride(input)
	default:
		return false
	}
}

func validActivityExceptionOverride(input ActivityExceptionInput) bool {
	if input.StartTime == nil && input.EndTime == nil && input.RoomID == nil {
		return false
	}
	if input.StartTime != nil && !validClock(*input.StartTime) {
		return false
	}
	if input.EndTime != nil && !validClock(*input.EndTime) {
		return false
	}
	if input.StartTime == nil || input.EndTime == nil {
		return true
	}
	start, _ := time.Parse("15:04:05", *input.StartTime)
	end, _ := time.Parse("15:04:05", *input.EndTime)
	return end.After(start)
}

func validActivityInstanceFilter(filter ActivityInstanceFilter) bool {
	if filter.Limit < 0 || filter.Offset < 0 || hasInvalidID(filter.IDs) ||
		hasInvalidID(filter.ActivityGroupIDs) || hasInvalidID(filter.ActiveGroupIDs) ||
		(filter.ActivityGroupID != nil && *filter.ActivityGroupID <= 0) ||
		(filter.ActiveGroupID != nil && *filter.ActiveGroupID <= 0) ||
		!validOptionalDate(filter.Date) || !validOptionalDate(filter.FromDate) || !validOptionalDate(filter.ToDate) {
		return false
	}
	for _, date := range filter.Dates {
		if !validDate(date) {
			return false
		}
	}
	return filter.Status == "" || validActivityInstanceStatus(filter.Status)
}

func normalizeActivityInstance(input ActivityInstanceInput) (ActivityInstanceInput, bool) {
	if input.ListKind != nil && *input.ListKind == "" {
		input.ListKind = nil
	}
	if input.Title == "" || len(input.Title) > ActivityInstanceTitleMaxLength ||
		!validDate(input.Date) || !validClock(input.StartTime) || !validClock(input.EndTime) ||
		input.RoomID <= 0 || !validActivityInstanceStatus(input.Status) ||
		(input.RequiredStaff != nil && *input.RequiredStaff < 0) || !normalizeListKind(&input.ListKind) {
		return input, false
	}
	start, _ := time.Parse("15:04:05", input.StartTime)
	end, _ := time.Parse("15:04:05", input.EndTime)
	if !end.After(start) {
		return input, false
	}
	if input.IdempotencyKey != nil {
		if strings.TrimSpace(*input.IdempotencyKey) == "" || len(*input.IdempotencyKey) > ActivityInstanceIdempotencyKeyMaxLength {
			return input, false
		}
	}
	return input, true
}

func validActivityInstanceStatus(value string) bool {
	return value == InstanceStatusPlanned || value == InstanceStatusActive ||
		value == InstanceStatusCompleted || value == InstanceStatusCancelled
}

func validActivityInstanceColumns(columns []string) bool {
	for _, column := range columns {
		switch column {
		case "date", "activity_group_id", "calendar_period_id", "title", "description",
			"start_time", "end_time", "room_id", "required_staff", "status", "active_group_id",
			"list_kind", "is_spontaneous", "understaffed_ack", "understaffed_note", "cancel_reason",
			"notes", "started_by", "started_at", "completed_at", "completed_by", "reopen_until",
			"completion_snapshot":
		default:
			return false
		}
	}
	return !hasDuplicateString(columns)
}

func hasDuplicateString(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func normalizePlanningTrack(input PlanningTrackInput) (PlanningTrackInput, bool) {
	input.Name = strings.TrimSpace(input.Name)
	valid := input.Name != "" && len(input.Name) <= 100 && planningTrackColorPattern.MatchString(input.Color) && input.SortOrder >= 0
	return input, valid
}

func normalizeRecurrenceRule(input RecurrenceRuleInput) (RecurrenceRuleInput, bool) {
	input.Frequency = strings.ToLower(input.Frequency)
	if !validRecurrenceFrequency(input.Frequency) || input.IntervalCount < 1 {
		return input, false
	}
	for index, weekday := range input.Weekdays {
		input.Weekdays[index] = strings.ToUpper(weekday)
		if !validRecurrenceWeekday(input.Weekdays[index]) {
			return input, false
		}
	}
	for _, day := range input.MonthDays {
		if day < 1 || day > 31 {
			return input, false
		}
	}
	return input, (input.Count == nil || *input.Count > 0) && (input.EndDate == nil || input.Count == nil)
}

func validRecurrenceFrequency(value string) bool {
	return value == "daily" || value == "weekly" || value == "monthly" || value == "yearly"
}

func validRecurrenceWeekday(value string) bool {
	switch value {
	case "MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN":
		return true
	default:
		return false
	}
}

func hasDuplicateID(ids []int64) bool {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			return true
		}
		seen[id] = struct{}{}
	}
	return false
}

func invalidCareExitRemovals(removals []CareExitEnrollmentRemoval) bool {
	for _, removal := range removals {
		if removal.ID <= 0 || removal.TenantID <= 0 || removal.StudentID <= 0 {
			return true
		}
		if removal.WasDeleted && (removal.ActivityGroupID <= 0 || !validDate(removal.ValidFrom)) {
			return true
		}
		if (removal.ValidUntil != nil && !validDate(*removal.ValidUntil)) ||
			(removal.PreviousValidUntil != nil && !validDate(*removal.PreviousValidUntil)) {
			return true
		}
	}
	return false
}

func normalizeGroup(input *GroupInput) error {
	if input.Name == "" || input.MaxParticipants < 0 || input.CategoryID <= 0 ||
		(input.RequiredStaff != nil && *input.RequiredStaff < 0) {
		return ErrInvalidGroup
	}
	if input.Type == "" {
		input.Type = GroupTypeActivity
	}
	if !validGroupType(input.Type) || !normalizeListKind(&input.ListKind) {
		return ErrInvalidGroup
	}
	if input.TargetGroupType == "" {
		input.TargetGroupType = TargetGroupTypeNone
	}
	if !validGroupTarget(input) || !normalizeOfferingSource(input) {
		return ErrInvalidGroup
	}
	return nil
}

func normalizeTemplateUpdate(input *TemplateUpdate) bool {
	group := GroupInput{
		Name: input.Name, Type: input.Type, CategoryID: input.CategoryID,
		EducationGroupID: input.EducationGroupID, MaxParticipants: input.MaxParticipants,
		RequiredStaff: input.RequiredStaff, ListKind: input.ListKind, IsTemplate: true,
		CalendarPeriodID: input.CalendarPeriodID, TargetGroupType: input.TargetGroupType,
		TargetGradeLevel: input.TargetGradeLevel, TargetSchoolClass: input.TargetSchoolClass,
		SourceCareOfferingIDs: input.SourceCareOfferingIDs, SourceGradeLevels: input.SourceGradeLevels,
		SourceSchoolClasses: input.SourceSchoolClasses, Notes: input.Notes,
	}
	if normalizeGroup(&group) != nil {
		return false
	}
	input.Type, input.ListKind, input.TargetGroupType = group.Type, group.ListKind, group.TargetGroupType
	input.TargetSchoolClass = group.TargetSchoolClass
	input.SourceCareOfferingIDs, input.SourceGradeLevels = group.SourceCareOfferingIDs, group.SourceGradeLevels
	input.SourceSchoolClasses = group.SourceSchoolClasses
	return true
}

func validGroupType(value string) bool {
	return value == GroupTypeActivity || value == GroupTypeCare || value == GroupTypeExternal
}

func normalizeListKind(value **string) bool {
	if *value == nil {
		return true
	}
	if **value == "" {
		*value = nil
		return true
	}
	switch **value {
	case ListKindEdgeHours, ListKindLearningTime, ListKindActivity, ListKindMensa:
		return true
	default:
		return false
	}
}

func validGroupTarget(input *GroupInput) bool {
	switch input.TargetGroupType {
	case TargetGroupTypeGrade:
		return input.TargetGradeLevel != nil && *input.TargetGradeLevel >= 1 && *input.TargetGradeLevel <= 13 && input.TargetSchoolClass == nil
	case TargetGroupTypeSchoolClass:
		if input.TargetSchoolClass == nil || strings.TrimSpace(*input.TargetSchoolClass) == "" || input.TargetGradeLevel != nil {
			return false
		}
		trimmed := strings.TrimSpace(*input.TargetSchoolClass)
		input.TargetSchoolClass = &trimmed
		return true
	case TargetGroupTypeEducationGroup:
		return input.EducationGroupID != nil && input.TargetGradeLevel == nil && input.TargetSchoolClass == nil
	case TargetGroupTypeOffering, TargetGroupTypeNone:
		return input.TargetGradeLevel == nil && input.TargetSchoolClass == nil
	default:
		return false
	}
}

func normalizeOfferingSource(input *GroupInput) bool {
	if len(input.SourceCareOfferingIDs) == 0 {
		input.SourceCareOfferingIDs = nil
		if len(input.SourceGradeLevels) > 0 || len(input.SourceSchoolClasses) > 0 {
			return false
		}
		input.SourceGradeLevels, input.SourceSchoolClasses = nil, nil
		return true
	}
	if input.TargetGroupType != TargetGroupTypeOffering || hasInvalidOrDuplicateIDs(input.SourceCareOfferingIDs) ||
		hasInvalidOrDuplicateGrades(input.SourceGradeLevels) {
		return false
	}
	classes, ok := normalizedSourceClasses(input.SourceSchoolClasses)
	if !ok || (len(input.SourceGradeLevels) > 0 && len(classes) > 0) {
		return false
	}
	input.SourceSchoolClasses = classes
	if len(input.SourceGradeLevels) == 0 {
		input.SourceGradeLevels = nil
	}
	return true
}

func hasInvalidOrDuplicateIDs(ids []int64) bool {
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return true
		}
		seen[id] = true
	}
	return false
}

func hasInvalidOrDuplicateGrades(grades []int) bool {
	seen := make(map[int]bool, len(grades))
	for _, grade := range grades {
		if grade < 1 || grade > 13 || seen[grade] {
			return true
		}
		seen[grade] = true
	}
	return false
}

func normalizedSourceClasses(classes []string) ([]string, bool) {
	if len(classes) == 0 {
		return nil, true
	}
	result := make([]string, 0, len(classes))
	seen := make(map[string]bool, len(classes))
	for _, class := range classes {
		trimmed := strings.TrimSpace(class)
		key := strings.ToLower(trimmed)
		if trimmed == "" || seen[key] {
			return nil, false
		}
		seen[key] = true
		result = append(result, trimmed)
	}
	return result, true
}

func normalizeGroupTargets(groupID int64, targets []GroupTargetInput) ([]GroupTargetInput, error) {
	if groupID <= 0 {
		return nil, ErrInvalidGroupTarget
	}
	result := make([]GroupTargetInput, len(targets))
	var targetType string
	for index, target := range targets {
		normalized, err := normalizeGroupTarget(target)
		if err != nil || (targetType != "" && normalized.TargetGroupType != targetType) {
			return nil, ErrInvalidGroupTarget
		}
		targetType = normalized.TargetGroupType
		result[index] = normalized
	}
	return result, nil
}

func normalizeGroupTarget(target GroupTargetInput) (GroupTargetInput, error) {
	switch target.TargetGroupType {
	case TargetGroupTypeGrade:
		if target.TargetGradeLevel == nil || *target.TargetGradeLevel < 1 || *target.TargetGradeLevel > 13 || target.TargetSchoolClass != nil || target.EducationGroupID != nil {
			return GroupTargetInput{}, ErrInvalidGroupTarget
		}
	case TargetGroupTypeSchoolClass:
		if target.TargetSchoolClass == nil || strings.TrimSpace(*target.TargetSchoolClass) == "" || target.TargetGradeLevel != nil || target.EducationGroupID != nil {
			return GroupTargetInput{}, ErrInvalidGroupTarget
		}
		trimmed := strings.TrimSpace(*target.TargetSchoolClass)
		target.TargetSchoolClass = &trimmed
	case TargetGroupTypeEducationGroup:
		if target.EducationGroupID == nil || *target.EducationGroupID <= 0 || target.TargetGradeLevel != nil || target.TargetSchoolClass != nil {
			return GroupTargetInput{}, ErrInvalidGroupTarget
		}
	default:
		return GroupTargetInput{}, ErrInvalidGroupTarget
	}
	return target, nil
}

func ErrorCode(err error) string {
	switch {
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
	default:
		return "internal_error"
	}
}
