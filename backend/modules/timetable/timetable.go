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
	ErrCategoryNotFound          = errors.New("activity category not found")
	ErrInvalidCategory           = errors.New("invalid activity category")
	ErrInvalidCareExitEnrollment = errors.New("invalid care-exit activity enrollment")
	ErrUnknownCategoryIDs        = errors.New("one or more category IDs do not exist in this tenant")
	ErrSystemCategoryProtected   = errors.New("Systemkategorie kann nicht bearbeitet oder archiviert werden") //nolint:staticcheck // ST1005: stable user-facing contract
	ErrSystemCategoryName        = errors.New("Dieser Name ist für eine Systemkategorie reserviert")          //nolint:staticcheck // ST1005: stable user-facing contract
	ErrCategoryNameExists        = errors.New("Eine Kategorie mit diesem Namen existiert bereits")            //nolint:staticcheck // ST1005: stable user-facing contract
	ErrCategoryArchived          = errors.New("Archivierte Kategorie muss zuerst wiederhergestellt werden")   //nolint:staticcheck // ST1005: stable user-facing contract
	ErrGroupNotFound             = errors.New("activity group not found")
	ErrInvalidGroup              = errors.New("invalid activity group")
	ErrInvalidGroupQuery         = errors.New("invalid activity group query")
	ErrInvalidGroupTarget        = errors.New("invalid activity group target")
)

var categoryColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

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

type Query interface {
	FindCategory(context.Context, int64) (Category, error)
	FindCategoryForAssignment(context.Context, int64) (Category, error)
	FindCategoryByName(context.Context, string) (Category, error)
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
	CreateCategory(context.Context, CreateCategory) (Category, error)
	UpdateCategory(context.Context, UpdateCategory) (Category, error)
	ArchiveCategory(context.Context, int64) (Category, error)
	RestoreCategory(context.Context, int64) (Category, error)
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

func (m *Module) FindCategoryByName(ctx context.Context, name string) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, m.reject("find_category_by_name", ErrInvalidCategory)
	}
	return m.engine.FindCategoryByName(ctx, name)
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
	default:
		return "internal_error"
	}
}
